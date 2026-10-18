package translation

import (
 "context"
 "database/sql"
 "sort"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/language"
)

type ListTranslationProgressInput struct{Language *string `json:"language,omitempty"`}
type LanguageProgress struct{Language string `json:"language"`;CompletedUnits int `json:"completedUnits"`;QueuedUnits int `json:"queuedUnits"`;ReviewUnits int `json:"reviewUnits"`}
type ListTranslationProgressResult struct{Languages []LanguageProgress `json:"languages"`}
func ListTranslationProgress(ctx context.Context,op OperationContext,input ListTranslationProgressInput)(ListTranslationProgressResult,error){
 if err:=language.RequireAdministrator(op);err!=nil{return ListTranslationProgressResult{},err};filter:="";if input.Language!=nil{parsed,err:=language.NormalizeLanguageTag(ctx,op,language.NormalizeLanguageTagInput{Tag:*input.Language});if err!=nil{return ListTranslationProgressResult{},err};filter=parsed.Tag}
 result:=ListTranslationProgressResult{Languages:[]LanguageProgress{}}
 err:=language.ReadData(ctx,op,func(tx *sql.Tx)error{
  rows,err:=tx.QueryContext(ctx,`SELECT language_tag FROM languages WHERE data_epoch=? ORDER BY language_tag`,op.DataEpoch);if err!=nil{return language.StorageError(err)};progress:=map[string]LanguageProgress{}
  for rows.Next(){var tag string;if err=rows.Scan(&tag);err!=nil{rows.Close();return language.StorageError(err)};canonical,err:=language.NormalizeLanguageTag(ctx,op,language.NormalizeLanguageTagInput{Tag:tag});if err!=nil{rows.Close();return platform.NewError(platform.CodeIntegrityError,false,"语言进度配置的标签不合法")};progress[canonical.Tag]=LanguageProgress{Language:tag}};err=rows.Err();rows.Close();if err!=nil{return language.StorageError(err)}
  // 每个文档/目标仅统计最新manifest，历史结果与旧任务不重复累计。
  rows,err=tx.QueryContext(ctx,`SELECT u.target_language,SUM(CASE WHEN u.status='normal' THEN 1 ELSE 0 END),SUM(CASE WHEN u.status='fallback' AND m.state IN('queued','running') THEN 1 ELSE 0 END),SUM(CASE WHEN u.status='review' THEN 1 ELSE 0 END) FROM translation_units u JOIN translation_manifests m ON m.translation_id=u.translation_id WHERE u.data_epoch=? AND m.data_epoch=? AND NOT EXISTS(SELECT 1 FROM translation_manifests newer WHERE newer.target_language=m.target_language AND newer.data_epoch=m.data_epoch AND json_extract(newer.publication_ref,'$.documentId')=json_extract(m.publication_ref,'$.documentId') AND newer.generation>m.generation) GROUP BY u.target_language`,op.DataEpoch,op.DataEpoch);if err!=nil{return language.StorageError(err)};defer rows.Close()
  for rows.Next(){var tag string;var done,queued,review int;if err=rows.Scan(&tag,&done,&queued,&review);err!=nil{return language.StorageError(err)};p,exists:=progress[tag];if !exists{return platform.NewError(platform.CodeIntegrityError,false,"译文进度引用未登记语言")};p.CompletedUnits=done;p.QueuedUnits=queued;p.ReviewUnits=review;progress[tag]=p};if err=rows.Err();err!=nil{return language.StorageError(err)}
  for tag,p:=range progress{if filter==""||tag==filter{result.Languages=append(result.Languages,p)}};sort.Slice(result.Languages,func(i,j int)bool{return result.Languages[i].Language<result.Languages[j].Language});return nil
 });return result,err
}
