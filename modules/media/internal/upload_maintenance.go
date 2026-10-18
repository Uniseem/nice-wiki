package media
import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/media/internal/gen"
)
type ExpireUploadSessionsInput struct{Now time.Time `json:"now"`}
type ExpireUploadSessionsResult struct{ExpiredIds []string `json:"expiredIds"`}

// ExpireUploadSessions先持久expired+prepared，再只清该上传的服务生成临时路径。
func ExpireUploadSessions(ctx context.Context,op OperationContext,input ExpireUploadSessionsInput)(ExpireUploadSessionsResult,error){
 result:=ExpireUploadSessionsResult{ExpiredIds:[]string{}}
 if op.CallerService!="media"||op.Actor.Kind!="service"||op.Actor.SubjectID!="media"{return result,platform.NewError(platform.CodeForbidden,false,"临时上传回收仅限本域后台")}
 s,ok:=currentBlobStore().(*localBlobStore);if !ok{return result,platform.NewError(platform.CodeDependencyUnavailable,true,"媒体存储未装配")}
 if op.DataEpoch!=s.openedEpoch{return result,platform.NewError(platform.CodeVersionMismatch,false,"临时上传回收代次已变化")}
 work,cancel,err:=s.mediaWriteContext(ctx,op);if err!=nil{return result,err};defer cancel()
 s.mu.Lock();defer s.mu.Unlock()
 rows,err:=s.db.QueryContext(work,`SELECT upload_id FROM upload_sessions WHERE state='receiving' AND data_epoch=? AND julianday(json_extract(session_json,'$.expiresAt'))<=julianday(?) UNION SELECT upload_id FROM upload_expirations WHERE state='prepared' AND data_epoch=? ORDER BY upload_id LIMIT 1000`,op.DataEpoch,input.Now.UTC().Format(time.RFC3339Nano),op.DataEpoch)
 if err!=nil{return result,mapMediaDBError(err,"读取临时上传回收计划失败")}
 ids:=[]string{};for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return result,mapMediaDBError(err,"读取临时上传标识失败")};ids=append(ids,id)};err=rows.Err();rows.Close();if err!=nil{return result,mapMediaDBError(err,"读取临时上传回收计划失败")}
 root,err:=s.openMediaRoot();if err!=nil{return result,err};defer root.Close()
 for _,id:=range ids{
  tx,err:=s.db.BeginTx(work,nil);if err!=nil{return result,mapMediaDBError(err,"开始临时上传回收失败")}
  record,found,err:=readUploadRow(work,tx,id,s.validateUpload);if err!=nil{tx.Rollback();return result,err};if !found||record.dataEpoch!=op.DataEpoch{tx.Rollback();return result,platform.NewError(platform.CodeIntegrityError,false,"临时上传回收原记录缺失")}
  if record.session.State==gen.Receiving{
   if record.session.ExpiresAt.After(input.Now){tx.Rollback();continue}
   record.session.State=gen.Expired;record.session.Revision++
   if err=writeUploadRow(work,tx,record,false);err!=nil{tx.Rollback();return result,err}
   if _,err=tx.ExecContext(work,`INSERT INTO upload_expirations(upload_id,site_id,data_epoch,state,decided_at) VALUES(?,?,?,'prepared',?)`,id,record.session.SiteId,op.DataEpoch,platform.Now());err!=nil{tx.Rollback();return result,mapMediaDBError(err,"保存临时上传删除意图失败")}
  }else if record.session.State!=gen.Expired{tx.Rollback();return result,platform.NewError(platform.CodeIntegrityError,false,"临时上传删除意图与实际状态冲突")}
  if err=tx.Commit();err!=nil{return result,mapMediaDBError(err,"提交临时上传回收决定失败")}
  name:="uploads/"+id+".part";info,err:=root.Lstat(name)
  if err==nil{if !info.Mode().IsRegular(){return result,platform.NewError(platform.CodeIntegrityError,false,"临时上传删除目标不是普通文件")};if err=root.Remove(name);err!=nil{return result,mapMediaDBError(err,"删除已过期临时上传失败")}}else if !errors.Is(err,os.ErrNotExist){return result,mapMediaDBError(err,"读取已过期临时上传失败")}
  if err=syncDirectory(filepath.Join(s.root,"uploads"));err!=nil{return result,err}
  if _,err=s.db.ExecContext(work,`UPDATE upload_expirations SET state='removed',removed_at=? WHERE upload_id=? AND data_epoch=? AND state='prepared'`,platform.Now(),id,op.DataEpoch);err!=nil{return result,mapMediaDBError(err,"确认临时上传删除失败")}
  result.ExpiredIds=append(result.ExpiredIds,id)
 }
 return result,nil
}
