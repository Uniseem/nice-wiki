package runtime

import (
 "bytes"
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "io"
 "os"
 "path/filepath"
 "strconv"
 "strings"
 "time"
 "unicode/utf8"

 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
 "nice-wiki/libs/platform/outbox"
 "nice-wiki/modules/memory/internal/gen"
)

// Queryer 允许所有关联读取固定在同一 SQLite 快照中。
type Queryer interface {
 QueryContext(context.Context,string,...any)(*sql.Rows,error)
 QueryRowContext(context.Context,string,...any)*sql.Row
}

// ConfigureWrites 由入口装配公共运行权威和本域屏障安全窗口。
func ConfigureWrites(window platform.WriteWindowConfig) error {
 s:=CurrentStore(); if s==nil {return platform.NewError(platform.CodeDependencyUnavailable,true,"记忆存储未打开")}
 s.writeMu.Lock(); defer s.writeMu.Unlock(); s.window=window; return nil
}

// ReadSnapshot 把条目、映射、预算和版本放在同一只读事务中读取。
func ReadSnapshot(ctx context.Context, fn func(*Store,*sql.Tx) error) error {
 s:=CurrentStore(); if s==nil {return platform.NewError(platform.CodeDependencyUnavailable,true,"记忆存储未打开")}
 tx,err:=s.db.BeginTx(ctx,&sql.TxOptions{ReadOnly:true}); if err!=nil{return DBError(err)};defer tx.Rollback()
 if err=fn(s,tx);err!=nil{return err};return DBError(tx.Commit())
}

// Write 原结果、业务变化与 outbox 同事务；收费调用不在此处执行。
func Write(ctx context.Context,op platform.OperationContext,functionID,key string,input,result any,fn func(*Store,*sql.Tx) error) error {
 s:=CurrentStore();if s==nil{return platform.NewError(platform.CodeDependencyUnavailable,true,"记忆存储未打开")}
 s.writeMu.Lock();defer s.writeMu.Unlock()
 if s.window.Runtime==nil||s.window.WriteWindow==nil{return platform.NewError(platform.CodeDependencyUnavailable,true,"记忆安全写窗口未装配")}
 work,view,cancel,err:=lifecycle.NewWriteContext(ctx,s.window);if err!=nil{return err};defer cancel()
 if view.DataEpoch!=op.DataEpoch || s.dataEpoch!=op.DataEpoch{return platform.NewError(platform.CodeVersionMismatch,false,"记忆数据代次已变化")}
 var state string
 if err=s.controlDB.QueryRowContext(work,"SELECT barrier_state FROM control_state WHERE id=1").Scan(&state);err!=nil{return DBError(err)}
 if state!="released"{return platform.NewError(platform.CodeLocked,false,"记忆屏障阻止新写入")}
 raw,err:=platform.CanonicalJSON(input);if err!=nil{return platform.NewError(platform.CodeInternal,false,"记忆请求不可编码")};digest:=platform.HashBytes(raw)
 tx,err:=s.db.BeginTx(work,nil);if err!=nil{return DBError(err)};defer tx.Rollback()
 var prior []byte;var hash,expiry string
 err=tx.QueryRowContext(work,`SELECT input_hash,result_json,expires_at FROM memory_receipts WHERE site_id=? AND actor_id=? AND function_id=? AND key_hash=? AND data_epoch=?`,op.Actor.SiteID,op.Actor.SubjectID,functionID,platform.HashBytes([]byte(key)),op.DataEpoch).Scan(&hash,&prior,&expiry)
 expires,_:=platform.ParseTimestamp(expiry)
 if err==nil && expires.After(time.Now()){
  if hash!=digest{return platform.NewError(platform.CodeConflict,false,"同幂等键的记忆请求不同")}
  if err=json.Unmarshal(prior,result);err!=nil{return platform.NewError(platform.CodeIntegrityError,false,"记忆原结果损坏")};return nil
 }
 if err!=nil&&!errors.Is(err,sql.ErrNoRows){return DBError(err)}
 if err=fn(s,tx);err!=nil{return err}
 encoded,err:=platform.CanonicalJSON(result);if err!=nil{return platform.NewError(platform.CodeInternal,false,"记忆结果不可编码")}
 _,err=tx.ExecContext(work,`INSERT INTO memory_receipts(site_id,actor_id,function_id,key_hash,data_epoch,input_hash,result_json,expires_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(site_id,actor_id,function_id,key_hash,data_epoch) DO UPDATE SET input_hash=excluded.input_hash,result_json=excluded.result_json,expires_at=excluded.expires_at`,op.Actor.SiteID,op.Actor.SubjectID,functionID,platform.HashBytes([]byte(key)),op.DataEpoch,digest,encoded,time.Now().UTC().Add(7*24*time.Hour).Format(time.RFC3339Nano))
 if err!=nil{return DBError(err)};return DBError(tx.Commit())
}

// RequireManager 防止服务身份代替人工管理权限；memory.manage 已由 HTTP 边界建立，不在内部重复远程授权。
func RequireManager(op platform.OperationContext) error {
 if op.Actor.Kind!="admin"{return platform.NewError(platform.CodeForbidden,false,"记忆管理只接受管理员")}
 return nil
}

// DBError 统一映射 IO 错误，不能输出 SQL 或业务正文。
func DBError(err error) error {
 if err==nil{return nil};return platform.NormalizeIOError(err,platform.CodeInternal,false,"记忆持久化失败")
}

// TaskIDs 是根唯一任务目录的当前生成输入快照，交付时由集成核对。
var TaskIDs=[]string{"qa.preflight","qa.inspect","qa.answer","qa.cache-match","qa.cache-translate","qa.compact","knowledge.context","knowledge.describe","knowledge.describe-visual","knowledge.extract-attachment","knowledge.embed","knowledge.rerank","localization.document","localization.ui","localization.terms","localization.review","localization.suggest","memory.overview","memory.assistant","content.slug","content.seo"}
func ValidateTask(task string) error {if !allowedTask(task){return platform.NewError(platform.CodeValidation,false,"记忆任务不在目录")};return nil}

// ReadEntryObject 以索引定位完整权威 Markdown，核对对象摘要和正文摘要。
func (s *Store) ReadEntryObject(ctx context.Context,q Queryer,siteID,id string,version *int)(gen.MemoryEntry,error){
 if !platform.ValidIdentifier(id)||id=="."||id==".."{return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆对象标识不是安全路径段")}
 var v int;var refJSON,bodyHash string
 query:=`SELECT e.version,e.file_ref,e.body_hash FROM memory_entries e JOIN entry_current c ON c.site_id=e.site_id AND c.entry_id=e.entry_id AND c.version=e.version WHERE e.site_id=? AND e.entry_id=?`
 args:=[]any{siteID,id};if version!=nil{query=`SELECT version,file_ref,body_hash FROM memory_entries WHERE site_id=? AND entry_id=? AND version=?`;args=append(args,*version)}
 err:=q.QueryRowContext(ctx,query,args...).Scan(&v,&refJSON,&bodyHash);if errors.Is(err,sql.ErrNoRows){return gen.MemoryEntry{},platform.NewError(platform.CodeNotFound,false,"记忆条目不存在")};if err!=nil{return gen.MemoryEntry{},DBError(err)}
 var ref gen.ObjectRef;if json.Unmarshal([]byte(refJSON),&ref)!=nil || ref.Owner!="memory" || ref.ObjectId!=id || ref.Version!=v{return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆对象引用不一致")}
 path:=filepath.Join(s.objectsRoot,id,strconv.Itoa(v)+".md")
 info,err:=os.Lstat(path);if err!=nil||!info.Mode().IsRegular()||info.Size()!=int64(ref.ByteSize)||info.Size()>1<<20{return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆对象大小或类型不一致")}
 raw,err:=os.ReadFile(path);if err!=nil{return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆对象不可读")}
 if platform.HashBytes(raw)!=ref.Sha256{return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆对象摘要不一致")}
 entry,err:=decodeEntry(raw);if err!=nil{return gen.MemoryEntry{},err}
 if entry.EntryId!=id || entry.Version!=v || entry.BodyHash!=bodyHash || platform.HashBytes([]byte(entry.Body))!=bodyHash{return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆权威正文与提交不一致")}
 entry.FileRef=ref;return entry,nil
}

type entryHeader struct {SchemaVersion int `json:"schemaVersion"`;Entry gen.MemoryEntry `json:"entry"`}

func decodeEntry(raw []byte)(gen.MemoryEntry,error){
 bad:=func()(gen.MemoryEntry,error){return gen.MemoryEntry{},platform.NewError(platform.CodeIntegrityError,false,"记忆 Markdown 编码不合法")}
 if !utf8.Valid(raw)||!bytes.HasPrefix(raw,[]byte("---\n")){return bad()}
 end:=bytes.Index(raw[4:],[]byte("\n---\n"));if end<0{return bad()};end+=4
 header:=raw[4:end];var h entryHeader
 d:=json.NewDecoder(bytes.NewReader(header));d.DisallowUnknownFields();if d.Decode(&h)!=nil||h.SchemaVersion!=1{return bad()};if d.Decode(new(any))!=io.EOF{return bad()}
 canonical,err:=platform.CanonicalJSON(h);if err!=nil||!bytes.Equal(canonical,header){return bad()}
 // 规范重编码同时拒绝重复字段、缺字段以及非规范转义；正文只在分隔线后出现。
 h.Entry.Body=string(raw[end+5:]);return h.Entry,nil
}

// SaveEntryObject 先落盘不可变对象。事务失败只留下未激活孤儿，不改变 current 指针。
func (s *Store) SaveEntryObject(ctx context.Context,q Queryer,site string,entry gen.MemoryEntry)(gen.MemoryEntry,error){
 header:=entry;header.Body="";header.FileRef=gen.ObjectRef{}
 encoded,err:=platform.CanonicalJSON(entryHeader{SchemaVersion:1,Entry:header});if err!=nil{return gen.MemoryEntry{},platform.NewError(platform.CodeInternal,false,"记忆 Markdown 编码失败")}
 raw:=append(append(append([]byte("---\n"),encoded...),[]byte("\n---\n")...),[]byte(entry.Body)...)
 directory:=filepath.Join(s.objectsRoot,entry.EntryId);if err=os.MkdirAll(directory,0700);err!=nil{return gen.MemoryEntry{},DBError(err)}
 parent,openErr:=os.Open(s.objectsRoot);if openErr!=nil{return gen.MemoryEntry{},DBError(openErr)};err=parent.Sync();_ = parent.Close();if err!=nil{return gen.MemoryEntry{},DBError(err)}
 path:=filepath.Join(directory,strconv.Itoa(entry.Version)+".md")
 if old,readErr:=os.ReadFile(path);readErr==nil{if !bytes.Equal(old,raw){
 var committed int;checkErr:=q.QueryRowContext(ctx,`SELECT 1 FROM memory_entries WHERE site_id=? AND entry_id=? AND version=?`,site,entry.EntryId,entry.Version).Scan(&committed)
 if checkErr==nil{return gen.MemoryEntry{},platform.NewError(platform.CodeConflict,false,"记忆不可变版本已经提交")};if !errors.Is(checkErr,sql.ErrNoRows){return gen.MemoryEntry{},DBError(checkErr)}
 // 无已提交索引的对象是失败事务留下的孤儿；只移除这个未激活版本。
 if err=os.Remove(path);err!=nil{return gen.MemoryEntry{},DBError(err)}
 return s.SaveEntryObject(ctx,q,site,entry)
}}else{
  if !errors.Is(readErr,os.ErrNotExist){return gen.MemoryEntry{},DBError(readErr)}
  temp,createErr:=os.CreateTemp(directory,".entry-");if createErr!=nil{return gen.MemoryEntry{},DBError(createErr)}
  name:=temp.Name();defer os.Remove(name)
  if _,err=temp.Write(raw);err==nil{err=temp.Sync()};closeErr:=temp.Close();if err==nil{err=closeErr};if err!=nil{return gen.MemoryEntry{},DBError(err)}
  if err=os.Rename(name,path);err!=nil{return gen.MemoryEntry{},DBError(err)}
  dir,openErr:=os.Open(directory);if openErr!=nil{return gen.MemoryEntry{},DBError(openErr)};err=dir.Sync();_ = dir.Close();if err!=nil{return gen.MemoryEntry{},DBError(err)}
 }
 entry.FileRef=gen.ObjectRef{Owner:"memory",ObjectId:entry.EntryId,Version:entry.Version,Sha256:platform.HashBytes(raw),ByteSize:len(raw),MediaType:"text/markdown"};return entry,nil
}

// InsertEntryVersion 提交 manifest/current、不可变修订与派生索引。
func InsertEntryVersion(ctx context.Context,tx *sql.Tx,op platform.OperationContext,e gen.MemoryEntry,previous *int)error{
 ref,_:=json.Marshal(e.FileRef);tags,_:=json.Marshal(e.Tags)
 _,err:=tx.ExecContext(ctx,`INSERT INTO memory_entries(site_id,entry_id,version,title,file_ref,language,tags_json,priority,state,updated_at,body_hash,description,maximum_bytes,readonly,source,auto_update_enabled) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,op.Actor.SiteID,e.EntryId,e.Version,e.Title,string(ref),e.Language,string(tags),e.Priority,e.State,e.UpdatedAt.UTC().Format(time.RFC3339Nano),e.BodyHash,e.Description,e.MaximumBytes,e.Readonly,e.Source,e.AutoUpdateEnabled);if err!=nil{return DBError(err)}
 _,err=tx.ExecContext(ctx,`INSERT INTO entry_revisions(site_id,entry_id,version,body_hash,file_ref,actor_id,created_at) VALUES(?,?,?,?,?,?,?)`,op.Actor.SiteID,e.EntryId,e.Version,e.BodyHash,string(ref),op.Actor.SubjectID,e.UpdatedAt.UTC().Format(time.RFC3339Nano));if err!=nil{return DBError(err)}
 _,err=tx.ExecContext(ctx,`INSERT INTO entry_commits(site_id,entry_id,version,file_ref,body_hash,previous_version) VALUES(?,?,?,?,?,?)`,op.Actor.SiteID,e.EntryId,e.Version,string(ref),e.BodyHash,previous);if err!=nil{return DBError(err)}
 if previous==nil{_,err=tx.ExecContext(ctx,`INSERT INTO entry_current(site_id,entry_id,version) VALUES(?,?,?)`,op.Actor.SiteID,e.EntryId,e.Version)}else{var res sql.Result;res,err=tx.ExecContext(ctx,`UPDATE entry_current SET version=? WHERE site_id=? AND entry_id=? AND version=?`,e.Version,op.Actor.SiteID,e.EntryId,*previous);if err==nil{n,_:=res.RowsAffected();if n!=1{return platform.NewError(platform.CodeVersionMismatch,false,"记忆版本已变化")}}};if err!=nil{return DBError(err)}
 _,err=tx.ExecContext(ctx,`UPDATE runtime_state SET memory_generation=memory_generation+1,updated_at=? WHERE id=1`,platform.Now());return DBError(err)
}

// Emit 仅把 ID/hash/版本载荷放入平台 outbox。
func Emit(tx *sql.Tx,op platform.OperationContext,eventType,subject string,revision int,payload any)error{
 raw,err:=platform.CanonicalJSON(payload);if err!=nil{return platform.NewError(platform.CodeInternal,false,"记忆事件编码失败")}
 return outbox.EnqueueEvent(tx,platform.EventEnvelope{EventID:platform.NewIdentifier(),EventType:eventType,EventVersion:1,Producer:"memory",SubjectID:subject,Revision:int64(revision),OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:raw})
}

// ReadAllEntries 在事务内读取 current 对象，正文仍从权威文件核验。
func (s *Store) ReadAllEntries(ctx context.Context,q Queryer,site string)([]gen.MemoryEntry,error){
 rows,err:=q.QueryContext(ctx,`SELECT entry_id FROM entry_current WHERE site_id=? ORDER BY entry_id`,site);if err!=nil{return nil,DBError(err)}
 ids:=[]string{};for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return nil,DBError(err)};ids=append(ids,id)};err=rows.Err();rows.Close();if err!=nil{return nil,DBError(err)}
 entries:=make([]gen.MemoryEntry,0,len(ids));for _,id:=range ids{e,err:=s.ReadEntryObject(ctx,q,site,id,nil);if err!=nil{return nil,err};entries=append(entries,e)};return entries,nil
}

// NormalizeLineEndings 仅用于确定性 Markdown 编码。
func NormalizeLineEndings(s string)string{return strings.ReplaceAll(strings.ReplaceAll(s,"\r\n","\n"),"\r","\n")}

// SaveContextManifest 只保存派生引用清单，不递增业务代数或触发模型。
func SaveContextManifest(ctx context.Context,op platform.OperationContext,id,task,language string,revision int,versionHash,bodyHash string,size int,manifest any)error{
 s:=CurrentStore();if s==nil{return platform.NewError(platform.CodeDependencyUnavailable,true,"记忆存储未打开")}
 raw,err:=platform.CanonicalJSON(manifest);if err!=nil{return platform.NewError(platform.CodeInternal,false,"记忆注入清单编码失败")}
 _,err=s.db.ExecContext(ctx,`INSERT INTO context_snapshots(site_id,snapshot_id,task_id,language,mapping_revision,manifest_hash,body_hash,byte_size,created_at,manifest_json) SELECT ?,?,?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM runtime_state WHERE id=1 AND data_epoch=?) ON CONFLICT(site_id,snapshot_id) DO NOTHING`,op.Actor.SiteID,id,task,language,revision,versionHash,bodyHash,size,platform.Now(),raw,op.DataEpoch)
 return DBError(err)
}

// ConfigurePublisher 注入已绑定服务身份的 jobs 发布适配器。
func ConfigurePublisher(publisher platform.EventPublisher)error{s:=CurrentStore();if s==nil{return platform.NewError(platform.CodeDependencyUnavailable,true,"记忆存储未打开")};s.publisher=publisher;return nil}
// RelayBatch 使用平台当前运行态/安全写窗口，只装配本域参数。
func RelayBatch(ctx context.Context,limit int)(platform.RelayBatchResult,error){
 s:=CurrentStore();if s==nil||s.publisher==nil{return platform.RelayBatchResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"记忆事件发布器未装配")}
 return outbox.RunRelayBatch(ctx,outbox.NewStore(s.db),s.publisher,platform.RelayConfig{BatchLimit:limit,PollMillis:1000,Runtime:s.window.Runtime,WriteWindow:s.window.WriteWindow})
}
