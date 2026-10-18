// Package translation 维护绑定不可变源修订的译文对象；数据库不能替代权威文件。
package translation

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "os"
 "path/filepath"
 "strconv"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
 localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type OperationContext=platform.OperationContext
type TranslationObject struct{SchemaVersion int `json:"schemaVersion"`;Translation gen.Translation `json:"translation"`;SourceTree json.RawMessage `json:"sourceTree"`;Units []gen.TranslationUnit `json:"units"`}
func objectPath(root string,t gen.Translation)(string,error){
 if !platform.ValidIdentifier(t.Document.DocumentId)||t.Document.DocumentId=="."||t.Document.DocumentId==".."{return "",platform.NewError(platform.CodeIntegrityError,false,"译文来源标识不是安全路径段")}
 tag,err:=language.NormalizeLanguageTag(context.Background(),OperationContext{},language.NormalizeLanguageTagInput{Tag:t.TargetLanguage});if err!=nil{return "",err};if tag.Tag!=t.TargetLanguage{return "",platform.NewError(platform.CodeIntegrityError,false,"译文对象目标语言未规范")}
 return filepath.Join(root,"active","objects",tag.Tag,t.Document.DocumentId,strconv.Itoa(t.Generation)+".json"),nil
}
func ReadObject(ctx context.Context,tx *sql.Tx,op OperationContext,id string)(TranslationObject,error){
 var refJSON,epoch string;err:=tx.QueryRowContext(ctx,`SELECT file_ref,data_epoch FROM translation_manifests WHERE translation_id=?`,id).Scan(&refJSON,&epoch);if errors.Is(err,sql.ErrNoRows){return TranslationObject{},platform.NewError(platform.CodeNotFound,false,"译文不存在")};if err!=nil{return TranslationObject{},language.StorageError(err)};if epoch!=op.DataEpoch{return TranslationObject{},platform.NewError(platform.CodeVersionMismatch,false,"译文数据代次已变化")}
 var ref gen.ObjectRef;if json.Unmarshal([]byte(refJSON),&ref)!=nil||ref.Owner!="localization"||ref.ObjectId!=id{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文manifest对象引用不合法")}
 // 由已提交元数据决定路径，不接受引用中的任意路径。
 var target,documentJSON string;var generation int
 if err=tx.QueryRowContext(ctx,`SELECT target_language,publication_ref,generation FROM translation_manifests WHERE translation_id=?`,id).Scan(&target,&documentJSON,&generation);err!=nil{return TranslationObject{},language.StorageError(err)};var document gen.PublishedRevisionRef;if json.Unmarshal([]byte(documentJSON),&document)!=nil{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文来源索引损坏")}
 store,_:=localizationruntime.LanguageGateway();if store==nil{return TranslationObject{},platform.NewError(platform.CodeDependencyUnavailable,true,"译文存储未装配")};path,err:=objectPath(store.Root(),gen.Translation{TargetLanguage:target,Document:document,Generation:generation});if err!=nil{return TranslationObject{},err}
 info,err:=os.Lstat(path);if err!=nil||!info.Mode().IsRegular()||info.Size()!=int64(ref.ByteSize)||info.Size()>16<<20{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文权威对象大小或类型不一致")};raw,err:=os.ReadFile(path);if err!=nil||platform.HashBytes(raw)!=ref.Sha256{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文权威对象摘要不一致")}
 var object TranslationObject;if json.Unmarshal(raw,&object)!=nil||object.SchemaVersion!=1||object.Translation.TranslationId!=id||object.Translation.Generation!=generation||object.Translation.Document!=document{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文对象与激活manifest不一致")}
 if len(object.SourceTree)==0||string(object.SourceTree)=="null"{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文权威对象缺少完整源树")}
 seen:=map[string]bool{}
 for i:=range object.Units{u:=&object.Units[i];if seen[u.UnitId]||u.Document!=document||u.TargetLanguage!=target{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文单元归属或唯一性不一致")};seen[u.UnitId]=true;if u.TargetHash!=platform.HashBytes([]byte(u.TargetText)){return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文单元正文摘要不一致")};u.FileRef=ref
 var indexedHash string
 if err=tx.QueryRowContext(ctx,`SELECT target_hash,origin,protected,version,human_generation,status FROM translation_units WHERE translation_id=? AND unit_id=? AND data_epoch=?`,id,u.UnitId,op.DataEpoch).Scan(&indexedHash,&u.Origin,&u.Protected,&u.Version,&u.HumanGeneration,&u.Status);err!=nil{return TranslationObject{},language.StorageError(err)}
 if indexedHash!=u.TargetHash{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文正文索引与权威对象不一致")}
 };if err=tx.QueryRowContext(ctx,`SELECT state FROM translation_manifests WHERE translation_id=?`,id).Scan(&object.Translation.State);err!=nil{return TranslationObject{},language.StorageError(err)};return object,nil
}
func WriteObject(ctx context.Context,tx *sql.Tx,op OperationContext,object TranslationObject,previousGeneration int)(TranslationObject,error){
 store,_:=localizationruntime.LanguageGateway();if store==nil{return TranslationObject{},platform.NewError(platform.CodeDependencyUnavailable,true,"译文存储未装配")}
 if len(object.SourceTree)==0||string(object.SourceTree)=="null"{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文写入缺少完整源树")}
 generation,reserveErr:=store.ReserveObjectGeneration(object.Translation.TargetLanguage+"/"+object.Translation.Document.DocumentId,previousGeneration+1);if reserveErr!=nil{return TranslationObject{},reserveErr}
 sourceBytes,sourceErr:=platform.CanonicalJSON(object.SourceTree);if sourceErr!=nil||platform.HashBytes(sourceBytes)!=object.Translation.Document.ContentHash{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文源树与发布引用摘要不一致")}
 object.SchemaVersion=1;object.Translation.Generation=generation
 for i:=range object.Units{object.Units[i].FileRef=gen.ObjectRef{};object.Units[i].TargetHash=platform.HashBytes([]byte(object.Units[i].TargetText))}
 raw,err:=platform.CanonicalJSON(object);if err!=nil{return TranslationObject{},platform.NewError(platform.CodeInternal,false,"译文对象编码失败")};path,err:=objectPath(store.Root(),object.Translation);if err!=nil{return TranslationObject{},err}
 if err=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return TranslationObject{},language.StorageError(err)}
 if existing,readErr:=os.ReadFile(path);readErr==nil&&string(existing)!=string(raw){return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"预留译文对象路径已有不同不可变内容")}else if readErr!=nil&&!errors.Is(readErr,os.ErrNotExist){return TranslationObject{},language.StorageError(readErr)}
 temp,err:=os.CreateTemp(filepath.Dir(path),".translation-");if err!=nil{return TranslationObject{},language.StorageError(err)};defer os.Remove(temp.Name())
 if _,err=temp.Write(raw);err==nil{err=temp.Sync()};closeErr:=temp.Close();if err==nil{err=closeErr};if err!=nil{return TranslationObject{},language.StorageError(err)};if err=os.Rename(temp.Name(),path);err!=nil{return TranslationObject{},language.StorageError(err)}
 // 从文件父目录一直同步到active，确保新语言/文档目录也先于SQL提交持久化。
 active:=filepath.Join(store.Root(),"active");for dir:=filepath.Dir(path);;dir=filepath.Dir(dir){handle,openErr:=os.Open(dir);if openErr!=nil{return TranslationObject{},language.StorageError(openErr)};err=handle.Sync();_ = handle.Close();if err!=nil{return TranslationObject{},language.StorageError(err)};if dir==active{break}}
 ref:=gen.ObjectRef{Owner:"localization",ObjectId:object.Translation.TranslationId,Version:object.Translation.Generation,Sha256:platform.HashBytes(raw),MediaType:"application/json",ByteSize:len(raw)};refRaw,_:=json.Marshal(ref);now:=time.Now().UTC().Format(time.RFC3339Nano)
 result,err:=tx.ExecContext(ctx,`UPDATE translation_manifests SET generation=?,file_ref=?,updated_at=? WHERE translation_id=? AND generation=? AND data_epoch=?`,object.Translation.Generation,string(refRaw),now,object.Translation.TranslationId,previousGeneration,op.DataEpoch);if err!=nil{return TranslationObject{},language.StorageError(err)};n,_:=result.RowsAffected();if n!=1{return TranslationObject{},platform.NewError(platform.CodeVersionMismatch,false,"译文目标代数已变化")}
 for i:=range object.Units{u:=&object.Units[i];u.FileRef=ref
  if u.SourceText==nil{return TranslationObject{},platform.NewError(platform.CodeIntegrityError,false,"译文单元缺少固定原文")}
  _,err=tx.ExecContext(ctx,`UPDATE translation_units SET target_file_ref=?,target_text=?,target_hash=?,origin=?,protected=?,version=?,human_generation=?,status=?,previous_target_text=?,updated_at=? WHERE translation_id=? AND unit_id=? AND data_epoch=?`,string(refRaw),u.TargetText,u.TargetHash,u.Origin,u.Protected,u.Version,u.HumanGeneration,u.Status,u.PreviousTargetText,now,object.Translation.TranslationId,u.UnitId,op.DataEpoch);if err!=nil{return TranslationObject{},language.StorageError(err)}
 };return object,nil
}
