package translation

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
 "nice-wiki/modules/localization/internal/language"
)

type PlanIncrementalTranslationInput struct{TranslationId string `json:"translationId"`;Units []gen.TranslationUnit `json:"units"`;MinimumCharacters int `json:"minimumCharacters"`}
type PlanIncrementalTranslationResult struct{ReusedIds []string `json:"reusedIds"`;TranslateIds []string `json:"translateIds"`;ProtectedIds []string `json:"protectedIds"`;ReviewUnitIds []string `json:"reviewUnitIds"`}
func PlanIncrementalTranslation(ctx context.Context,op OperationContext,input PlanIncrementalTranslationInput)(PlanIncrementalTranslationResult,error){
 result:=PlanIncrementalTranslationResult{ReusedIds:[]string{},TranslateIds:[]string{},ProtectedIds:[]string{},ReviewUnitIds:[]string{}}
 err:=language.ReadData(ctx,op,func(tx *sql.Tx)error{
  object,err:=ReadObject(ctx,tx,op,input.TranslationId);if err!=nil{return err};previous:=map[string]gen.TranslationUnit{};for _,u:=range object.Units{previous[u.UnitId]=u}
  for _,candidate:=range input.Units{
   old,exists:=previous[candidate.UnitId]
   if exists&&(old.Protected||old.Origin=="human"){result.ProtectedIds=append(result.ProtectedIds,candidate.UnitId);if old.SourceHash!=candidate.SourceHash{result.ReviewUnitIds=append(result.ReviewUnitIds,candidate.UnitId)};continue}
   if exists&&old.SourceHash==candidate.SourceHash{result.ReusedIds=append(result.ReusedIds,candidate.UnitId);continue}
   if candidate.SourceText==nil{return platform.NewError(platform.CodeDependencyUnavailable,false,"增量规划缺少原文片段")}
   reused,err:=ReuseInSnapshot(ctx,tx,op,ReuseTranslationMemoryInput{SourceHash:candidate.SourceHash,SourceText:*candidate.SourceText,TargetLanguage:candidate.TargetLanguage,MinimumCharacters:input.MinimumCharacters,Candidate:candidate});if err!=nil{return err};if reused.Reused{result.ReusedIds=append(result.ReusedIds,candidate.UnitId)}else{result.TranslateIds=append(result.TranslateIds,candidate.UnitId)}
  };return nil
 });return result,err
}
