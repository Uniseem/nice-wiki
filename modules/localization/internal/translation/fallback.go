package translation

import (
 "context"
 "nice-wiki/libs/platform"
)

type PlanTranslationFallbackInput struct{TranslationId string `json:"translationId"`;FailedUnitIds []string `json:"failedUnitIds"`;Failure string `json:"failure"`;Stage string `json:"stage"`;ContextReduced bool `json:"contextReduced"`;TerminologyRetried bool `json:"terminologyRetried"`}
type PlanTranslationFallbackResult struct{RetryGroups [][]string `json:"retryGroups"`;ReduceContext bool `json:"reduceContext"`;FallbackUnitIds []string `json:"fallbackUnitIds"`;ReviewUnitIds []string `json:"reviewUnitIds"`;NextStage string `json:"nextStage"`}
func PlanTranslationFallback(ctx context.Context,op OperationContext,input PlanTranslationFallbackInput)(PlanTranslationFallbackResult,error){
 result:=PlanTranslationFallbackResult{RetryGroups:[][]string{},FallbackUnitIds:[]string{},ReviewUnitIds:[]string{},NextStage:input.Stage};ids:=input.FailedUnitIds
 if len(ids)==0{return result,nil}
 if input.Failure=="length"&&!input.ContextReduced{result.ReduceContext=true;result.RetryGroups=append(result.RetryGroups,append([]string{},ids...));return result,nil}
 if input.Failure=="terminology"{if !input.TerminologyRetried{result.RetryGroups=append(result.RetryGroups,append([]string{},ids...))}else{result.ReviewUnitIds=append(result.ReviewUnitIds,ids...)};return result,nil}
 switch input.Stage{
 case "batch":
  if len(ids)==1{result.RetryGroups=append(result.RetryGroups,append([]string{},ids...));result.NextStage="single"}else{middle:=(len(ids)+1)/2;result.RetryGroups=append(result.RetryGroups,append([]string{},ids[:middle]...),append([]string{},ids[middle:]...));result.NextStage="halves"}
 case "halves":for _,id:=range ids{result.RetryGroups=append(result.RetryGroups,[]string{id})};result.NextStage="single"
 case "single":result.FallbackUnitIds=append(result.FallbackUnitIds,ids...)
 default:return PlanTranslationFallbackResult{},platform.NewError(platform.CodeValidation,false,"翻译降级阶段不在登记集合")
 };return result,nil
}
