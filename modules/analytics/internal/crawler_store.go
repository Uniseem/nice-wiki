package analytics

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "time"
 "nice-wiki/libs/platform"
)

// RecordCrawlerAccess不接收访客元数据，成功Markdown读取按固定family独立计数。
func RecordCrawlerAccess(ctx context.Context,op OperationContext,in RecordCrawlerAccessInput)(RecordCrawlerAccessResult,error){
 if op.CallerService!="public-site"||op.Actor.Kind!="service"||op.Actor.SubjectID!="public-site"{return RecordCrawlerAccessResult{},platform.NewError(platform.CodeForbidden,false,"爬虫访问只接受公开交付服务")}
 known:=false;for _,family:=range []string{"GPTBot","ChatGPT-User","ClaudeBot","PerplexityBot","Googlebot","Bingbot"}{known=known||family==in.CrawlerFamily};if !known{return RecordCrawlerAccessResult{},platform.NewError(platform.CodeValidation,false,"爬虫类别未登记")}
 work,rt,tx,cancel,err:=beginWrite(ctx,op);if err!=nil{return RecordCrawlerAccessResult{},err};defer cancel();defer tx.Rollback()
 if rt.CountingLocation==nil{return RecordCrawlerAccessResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"可信报表时区未配置")}
 raw,err:=platform.CanonicalJSON(in);if err!=nil{return RecordCrawlerAccessResult{},err};digest:=platform.HashBytes(raw)
 var result RecordCrawlerAccessResult;found,err:=readReceipt(work,tx,op,"analytics.RecordCrawlerAccess",digest,&result);if err!=nil{return result,err};if found{return result,nil}
 if in.Status>=200&&in.Status<300{
  if _,err=tx.ExecContext(work,`INSERT INTO crawler_counts VALUES(?,?,?,?,1,?) ON CONFLICT(day,document_id,locale,crawler_family) DO UPDATE SET count=count+1`,time.Now().In(rt.CountingLocation).Format("2006-01-02"),in.DocumentID,in.Locale,in.CrawlerFamily,op.DataEpoch);err!=nil{return RecordCrawlerAccessResult{},storageError(err)}
  if _,err=advanceWatermark(work,tx,"crawlers");err!=nil{return RecordCrawlerAccessResult{},err};result.Accepted=true
 }
 if err=saveReceipt(work,tx,op,"analytics.RecordCrawlerAccess",digest,result);err==nil{err=tx.Commit()};if err!=nil{return RecordCrawlerAccessResult{},storageError(err)};return result,nil
}

// readReceipt按主体/函数/操作绑定，不能把不同命令的相同操作ID误当重放。
func readReceipt(ctx context.Context,tx *sql.Tx,op OperationContext,function,digest string,out any)(bool,error){
 var old,epoch,raw string;err:=tx.QueryRowContext(ctx,`SELECT request_hash,data_epoch,result_json FROM operation_receipts WHERE subject_id=? AND function_id=? AND operation_id=?`,op.Actor.SubjectID,function,op.OperationID).Scan(&old,&epoch,&raw)
 if errors.Is(err,sql.ErrNoRows){return false,nil};if err!=nil{return false,storageError(err)};if old!=digest||epoch!=op.DataEpoch{return false,platform.NewError(platform.CodeConflict,false,"统计命令操作标识已绑定不同输入")};if json.Unmarshal([]byte(raw),out)!=nil{return false,platform.NewError(platform.CodeIntegrityError,false,"统计命令回执损坏")};return true,nil
}
func saveReceipt(ctx context.Context,tx *sql.Tx,op OperationContext,function,digest string,result any)error{
 raw,err:=json.Marshal(result);if err!=nil{return platform.NewError(platform.CodeInternal,false,"统计回执编码失败")};_,err=tx.ExecContext(ctx,`INSERT INTO operation_receipts VALUES(?,?,?,?,?,?,?)`,op.Actor.SubjectID,function,op.OperationID,digest,string(raw),op.DataEpoch,time.Now().UTC().AddDate(0,0,7).Format(analyticsTimeLayout));return storageError(err)
}
