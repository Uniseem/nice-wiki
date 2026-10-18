package analytics

import (
 "context"
 "crypto/sha256"
 "database/sql"
 "encoding/hex"
 "encoding/json"
 "errors"
 "hash"
 "io"
 "os"
 "path/filepath"
 "strconv"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
)

const exportByteLimit int64=32<<20

type exportWriter struct{File *os.File;Hash hash.Hash;Bytes int64;Rows int64}

// CreateExport生成当前权限已核验的固定快照CSV；明确持久状态不假装文件与SQLite同一事务。
func CreateExport(ctx context.Context,op OperationContext,in CreateExportInput)(CreateExportResult,error){
 rt,err:=runtimeFrom(ctx,op);if err!=nil{return CreateExportResult{},err};rt.exportMu.Lock();defer rt.exportMu.Unlock()
 if in.Kind!="pages"&&in.Kind!="audits"&&in.Kind!="usage"{return CreateExportResult{},platform.NewError(platform.CodeValidation,false,"导出类型未登记")}
 interval,err:=ValidateDateRange(ctx,op,ValidateDateRangeInput{From:in.From,To:in.To,MaxDays:366});if err!=nil{return CreateExportResult{},err};begin,_:=time.Parse(time.RFC3339Nano,interval.FromUTC);end,_:=time.Parse(time.RFC3339Nano,interval.ToUTC)
 if in.Kind=="pages"{begin,end,_,err=pageInterval(ctx,op,rt,QueryPageStatisticsInput{From:in.From,To:in.To,Granularity:"hour"});if err!=nil{return CreateExportResult{},err}}
 input,_:=platform.CanonicalJSON([]string{in.Kind,in.From,in.To});digest:=platform.HashBytes(input);binding,_:=platform.CanonicalJSON([]string{rt.SiteID,rt.Epoch,op.Actor.SubjectID,op.OperationID});id:="export."+platform.HashBytes(binding)
 work,_,tx,cancel,err:=beginWrite(ctx,op);if err!=nil{return CreateExportResult{},err};var old,state,resultJSON,expiry string
 err=tx.QueryRowContext(work,`SELECT filter_hash,state,result_json,expires_at FROM export_runs WHERE export_id=? AND actor_id=? AND data_epoch=?`,id,op.Actor.SubjectID,rt.Epoch).Scan(&old,&state,&resultJSON,&expiry)
 exists:=err==nil;if err!=nil&&!errors.Is(err,sql.ErrNoRows){_ = tx.Rollback();cancel();return CreateExportResult{},storageError(err)}
 if exists{_ = tx.Rollback();cancel();if state!="preparing"&&state!="prepared"&&state!="ready"&&state!="expired"{return CreateExportResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"未知导出journal状态")};if old!=digest{return CreateExportResult{},platform.NewError(platform.CodeConflict,false,"导出操作已绑定不同条件")};expires,e:=time.Parse(analyticsTimeLayout,expiry);if e!=nil{return CreateExportResult{},platform.NewError(platform.CodeIntegrityError,false,"导出到期记录损坏")};if !expires.After(time.Now()){return CreateExportResult{},platform.NewError(platform.CodeGone,false,"导出已过期，请创建新操作")};if state=="ready"||state=="prepared"{if state=="prepared"{if err=installPreparedExport(ctx,op,rt,id);err!=nil{return CreateExportResult{},err}};var result CreateExportResult;if json.Unmarshal([]byte(resultJSON),&result)!=nil{return CreateExportResult{},platform.NewError(platform.CodeIntegrityError,false,"导出原回执损坏")};return result,nil}}else{
  expiry=time.Now().UTC().Add(24*time.Hour).Format(analyticsTimeLayout);_,err=tx.ExecContext(work,`INSERT INTO export_runs(export_id,filter_hash,actor_id,object_ref,expires_at,state,kind,operation_id,data_epoch) VALUES(?,?,?,'',?,'preparing',?,?,?)`,id,digest,op.Actor.SubjectID,expiry,in.Kind,op.OperationID,rt.Epoch);if err==nil{_,err=advanceWatermark(work,tx,"export-journal")};if err==nil{err=tx.Commit()};_ = tx.Rollback();cancel();if err!=nil{return CreateExportResult{},storageError(err)}
 }
 // preparing尚无已封存字节，可继续同原请求；prepared以后只核完整原对象，不重新查询。
 path:=filepath.Join(rt.Root,"active","exports",id+".csv.part");if err=lifecycle.ValidatePrivatePath(path,"regular-file",true);err!=nil{return CreateExportResult{},err};if err=os.Remove(path);err!=nil&&!errors.Is(err,os.ErrNotExist){return CreateExportResult{},storageError(err)}
 work,_,done,err:=lifecycle.NewWriteContext(platform.WithOperationContext(ctx,op),rt.Window);if err!=nil{return CreateExportResult{},err};defer done();file,err:=os.OpenFile(path,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600);if err!=nil{return CreateExportResult{},storageError(err)};defer file.Close();writer:=&exportWriter{File:file,Hash:sha256.New()}
 snapshot,err:=rt.DB.BeginTx(work,&sql.TxOptions{ReadOnly:true});if err!=nil{return CreateExportResult{},storageError(err)};watermark,err:=streamExportRows(work,snapshot,op,in.Kind,begin,end,writer);_ = snapshot.Rollback();if err!=nil{return CreateExportResult{},err};if err=file.Sync();err!=nil{return CreateExportResult{},storageError(err)};if err=file.Close();err!=nil{return CreateExportResult{},storageError(err)};if err=syncDirectory(filepath.Dir(path),rt.Root);err!=nil{return CreateExportResult{},err}
 ref:=platform.ObjectRef{Owner:"analytics",ObjectID:id,Version:1,SHA256:hex.EncodeToString(writer.Hash.Sum(nil)),MediaType:"text/csv",ByteSize:writer.Bytes};result:=CreateExportResult{ExportID:id,ObjectRef:ref,ExpiresAt:expiry,Kind:in.Kind,Watermark:watermark};refRaw,_:=platform.CanonicalJSON(ref);resultRaw,_:=platform.CanonicalJSON(result)
 work,_,tx,cancel,err=beginWrite(ctx,op);if err!=nil{return CreateExportResult{},err};_,err=tx.ExecContext(work,`UPDATE export_runs SET state='prepared',object_ref=?,result_json=?,source_watermark=? WHERE export_id=? AND state='preparing' AND data_epoch=?`,string(refRaw),string(resultRaw),watermark,id,rt.Epoch);if err==nil{_,err=advanceWatermark(work,tx,"export-journal")};if err==nil{err=tx.Commit()};_ = tx.Rollback();cancel();if err!=nil{return CreateExportResult{},storageError(err)}
 if err=installPreparedExport(ctx,op,rt,id);err!=nil{return CreateExportResult{},err};return result,nil
}

func writeCSVRecord(ctx context.Context,op OperationContext,writer *exportWriter,cells []string)error{
 if err:=ctx.Err();err!=nil{return storageError(err)};line:="";for i,value:=range cells{cell,err:=ValidateCSVCell(ctx,op,ValidateCSVCellInput{Value:value});if err!=nil{return err};if i!=0{line+=","};line+=cell.Escaped};line+="\r\n";if writer.Bytes+int64(len(line))>exportByteLimit{return platform.NewError(platform.CodeBudgetExceeded,false,"CSV超过32MiB私有导出预算")};n,err:=io.WriteString(writer.File,line);if err!=nil{return storageError(err)};if n!=len(line){return storageError(io.ErrShortWrite)};_,_ = writer.Hash.Write([]byte(line));writer.Bytes+=int64(n);return nil
}

func streamExportRows(ctx context.Context,tx *sql.Tx,op OperationContext,kind string,begin,end time.Time,writer *exportWriter)(int64,error){
 query,source:="","";columns:=[]string{}
 switch kind{case "pages":source="pages";columns=[]string{"periodStart","documentId","locale","views","visits"};query=`SELECT hour,document_id,locale,views,visits FROM hourly_page_counts WHERE hour>=? AND hour<? AND data_epoch=? ORDER BY hour,document_id,locale LIMIT 100001`
 case "audits":source="audits";columns=[]string{"auditId","actorId","action","resourceId","resultCode","occurredAt","hash","previousHash"};query=`SELECT audit_id,actor_id,action,resource_ref,result_code,occurred_at,hash,previous_hash FROM audit_records WHERE occurred_at>=? AND occurred_at<? AND data_epoch=? ORDER BY seq LIMIT 100001`
 case "usage":source="usage";columns=[]string{"ledgerEntryId","ledgerRevision","operationId","costClass","amountMicros","currency","state","occurredAt"};query=`SELECT ledger_entry_id,ledger_revision,operation_id,cost_class,amount_micros,currency,state,occurred_at FROM usage_projection WHERE occurred_at>=? AND occurred_at<? AND data_epoch=? ORDER BY occurred_at,ledger_entry_id LIMIT 100001`}
 var revision int64;if err:=tx.QueryRowContext(ctx,`SELECT coalesce((SELECT revision FROM aggregation_watermarks WHERE source=?),0)`,source).Scan(&revision);err!=nil{return 0,storageError(err)};if err:=writeCSVRecord(ctx,op,writer,columns);err!=nil{return 0,err};rows,err:=tx.QueryContext(ctx,query,begin.UTC().Format(analyticsTimeLayout),end.UTC().Format(analyticsTimeLayout),op.DataEpoch);if err!=nil{return 0,storageError(err)};defer rows.Close()
 for rows.Next(){writer.Rows++;if writer.Rows>100000{return 0,platform.NewError(platform.CodeBudgetExceeded,false,"导出超过100000行，未截断为成功")};values:=make([]string,len(columns));dest:=make([]any,len(values));for i:=range values{dest[i]=&values[i]};if err=rows.Scan(dest...);err!=nil{return 0,storageError(err)};if kind=="usage"{amount,e:=strconv.ParseInt(values[4],10,64);if e!=nil||amount<0||values[5]!="USD"||values[6]=="price_unknown"||values[6]=="charge_unknown"{return 0,platform.NewError(platform.CodeDependencyUnavailable,true,"未知费用不可导出为0")}};if err=writeCSVRecord(ctx,op,writer,values);err!=nil{return 0,err}}
 return revision,storageError(rows.Err())
}

func installPreparedExport(ctx context.Context,op OperationContext,rt *Runtime,id string)error{
 if !exportObjectNameValid(id){return platform.NewError(platform.CodeIntegrityError,false,"导出journal对象ID不合法")};var raw,state string;if err:=rt.DB.QueryRowContext(ctx,`SELECT object_ref,state FROM export_runs WHERE export_id=? AND data_epoch=?`,id,rt.Epoch).Scan(&raw,&state);err!=nil{return storageError(err)};if state=="ready"{return nil};if state!="prepared"{return platform.NewError(platform.CodeConflict,false,"导出对象未完整准备")};var ref platform.ObjectRef;if json.Unmarshal([]byte(raw),&ref)!=nil{return platform.NewError(platform.CodeIntegrityError,false,"导出对象引用损坏")}
 final:=filepath.Join(rt.Root,"active","exports",id+".csv");part:=final+".part";if err:=lifecycle.ValidatePrivatePath(final,"regular-file",true);err!=nil{return err}
 if _,err:=os.Lstat(final);errors.Is(err,os.ErrNotExist){file,e:=verifyExportFile(part,ref);if e!=nil{return e};_ = file.Close();work,_,cancel,e:=lifecycle.NewWriteContext(platform.WithOperationContext(ctx,op),rt.Window);if e!=nil{return e};defer cancel();if e=work.Err();e!=nil{return storageError(e)};if e=os.Rename(part,final);e!=nil{return storageError(e)};if e=syncDirectory(filepath.Dir(final),rt.Root);e!=nil{return e}}else if err!=nil{return storageError(err)}
 file,err:=verifyExportFile(final,ref);if err!=nil{return err};_ = file.Close();work,_,tx,cancel,err:=beginWrite(ctx,op);if err!=nil{return err};defer cancel();defer tx.Rollback();if _,err=tx.ExecContext(work,`UPDATE export_runs SET state='ready' WHERE export_id=? AND state='prepared' AND data_epoch=?`,id,rt.Epoch);err!=nil{return storageError(err)};if _,err=advanceWatermark(work,tx,"exports");err!=nil{return err};return storageError(tx.Commit())
}

func verifyExportFile(path string,ref platform.ObjectRef)(*os.File,error){
 if ref.Owner!="analytics"||ref.Version!=1||ref.MediaType!="text/csv"||ref.ByteSize<0||ref.ByteSize>exportByteLimit{return nil,platform.NewError(platform.CodeIntegrityError,false,"导出对象引用不合法")};if err:=lifecycle.ValidatePrivatePath(path,"regular-file",false);err!=nil{return nil,err};file,err:=os.Open(path);if err!=nil{return nil,storageError(err)};success:=false;defer func(){if !success{_ = file.Close()}}();info,err:=file.Stat();if err!=nil{return nil,storageError(err)};if info.Size()!=ref.ByteSize{return nil,platform.NewError(platform.CodeIntegrityError,false,"导出长度不符")};digest:=sha256.New();if _,err=io.Copy(digest,io.LimitReader(file,exportByteLimit+1));err!=nil{return nil,storageError(err)};if hex.EncodeToString(digest.Sum(nil))!=ref.SHA256{return nil,platform.NewError(platform.CodeIntegrityError,false,"导出完整哈希不符")};if _,err=file.Seek(0,io.SeekStart);err!=nil{return nil,storageError(err)};success=true;return file,nil
}

// OpenExport只读受控同actor/站点/代次/类型且未过期的不可变CSV，返回前核完整长度hash。
func OpenExport(ctx context.Context,op OperationContext,id,kind string)(*os.File,platform.ObjectRef,error){
 rt,err:=runtimeFrom(ctx,op);if err!=nil{return nil,platform.ObjectRef{},err};if !exportObjectNameValid(id){return nil,platform.ObjectRef{},platform.NewError(platform.CodeNotFound,false,"导出不存在")};rt.exportMu.Lock();defer rt.exportMu.Unlock();var actor,storedKind,epoch,expiry,state,raw string
 if err=rt.DB.QueryRowContext(ctx,`SELECT actor_id,kind,data_epoch,expires_at,state,object_ref FROM export_runs WHERE export_id=?`,id).Scan(&actor,&storedKind,&epoch,&expiry,&state,&raw);errors.Is(err,sql.ErrNoRows){return nil,platform.ObjectRef{},platform.NewError(platform.CodeNotFound,false,"导出不存在")};if err!=nil{return nil,platform.ObjectRef{},storageError(err)};if actor!=op.Actor.SubjectID||epoch!=op.DataEpoch||storedKind!=kind{return nil,platform.ObjectRef{},platform.NewError(platform.CodeForbidden,false,"导出主体代次或类型与授权路由不同")};expires,err:=time.Parse(analyticsTimeLayout,expiry);if err!=nil{return nil,platform.ObjectRef{},platform.NewError(platform.CodeIntegrityError,false,"导出期限损坏")};if !expires.After(time.Now()){return nil,platform.ObjectRef{},platform.NewError(platform.CodeGone,false,"导出已过期，请重新创建")};if state!="ready"{return nil,platform.ObjectRef{},platform.NewError(platform.CodeDependencyUnavailable,true,"导出尚未完整安装")};var ref platform.ObjectRef;if json.Unmarshal([]byte(raw),&ref)!=nil||ref.ObjectID!=id{return nil,platform.ObjectRef{},platform.NewError(platform.CodeIntegrityError,false,"导出引用不符")};file,err:=verifyExportFile(filepath.Join(rt.Root,"active","exports",id+".csv"),ref);return file,ref,err
}
