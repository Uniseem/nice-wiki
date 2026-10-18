package translation

import (
 "context"
 "database/sql"
 "encoding/json"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
 localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type FetchTranslationSourceInput struct{TranslationId string `json:"translationId"`;LeaseId string `json:"leaseId"`;Job platform.JobExecutionInput `json:"-"`}
type FetchTranslationSourceResult struct{Source string `json:"source"`;Document gen.PublishedRevisionRef `json:"document"`;SourceLanguage string `json:"sourceLanguage"`;Title string `json:"title"`}
func FetchTranslationSource(ctx context.Context,op OperationContext,input FetchTranslationSourceInput)(FetchTranslationSourceResult,error){
 job:=input.Job
 if op.CallerService!="localization"||op.Actor.Kind!="service"||job.JobType!="localization.document"||job.DataEpoch!=op.DataEpoch||job.InputRef.Owner!="localization"||job.InputRef.ObjectID!=input.TranslationId||job.WorkerID!=input.LeaseId||job.CheckLease==nil{return FetchTranslationSourceResult{},platform.NewError(platform.CodeForbidden,false,"翻译来源读取缺少受信任务租约绑定")}
 if err:=job.CheckLease(ctx);err!=nil{return FetchTranslationSourceResult{},err}
 var document gen.PublishedRevisionRef
 err:=language.ReadData(ctx,op,func(tx *sql.Tx)error{
  var raw,epoch,state string;if err:=tx.QueryRowContext(ctx,`SELECT document_ref,data_epoch,state FROM translation_jobs WHERE job_id=?`,input.TranslationId).Scan(&raw,&epoch,&state);err!=nil{return language.StorageError(err)};if epoch!=op.DataEpoch||state=="cancelled"||state=="superseded"||state=="failed"{return platform.NewError(platform.CodeStaleResult,false,"翻译任务已取消或属于旧代")};if json.Unmarshal([]byte(raw),&document)!=nil{return platform.NewError(platform.CodeIntegrityError,false,"翻译任务的固定来源损坏")};return nil
 });if err!=nil{return FetchTranslationSourceResult{},err}
 source,err:=localizationruntime.FetchPublishedSnapshot(ctx,op,document);if err!=nil{return FetchTranslationSourceResult{},err}
 return FetchTranslationSourceResult{Source:string(source.Body),Document:source.Reference,SourceLanguage:source.SourceLanguage,Title:source.Title},nil
}
