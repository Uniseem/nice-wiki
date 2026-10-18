package translation

import (
 "context"
 "strings"
 "golang.org/x/text/cases"
 "golang.org/x/text/unicode/norm"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/gen"
)

type CheckTerminologyComplianceInput struct{Units []gen.TranslationUnit `json:"units"`;Terms []gen.GlossaryTerm `json:"terms"`;AlreadyRetried bool `json:"alreadyRetried"`}
type CheckTerminologyComplianceResult struct{RetryUnitIds []string `json:"retryUnitIds"`;ReviewUnitIds []string `json:"reviewUnitIds"`}
func CheckTerminologyCompliance(ctx context.Context,op OperationContext,input CheckTerminologyComplianceInput)(CheckTerminologyComplianceResult,error){
 result:=CheckTerminologyComplianceResult{RetryUnitIds:[]string{},ReviewUnitIds:[]string{}};fold:=cases.Fold()
 for _,unit:=range input.Units{
  if unit.SourceText==nil{return CheckTerminologyComplianceResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"术语核验缺少固定原文")}
  bad:=false
  for _,term:=range input.Terms{if strings.Contains(*unit.SourceText,term.SourceTerm)&&!strings.Contains(fold.String(norm.NFC.String(unit.TargetText)),fold.String(norm.NFC.String(term.TargetTerm))){bad=true;break}}
  if bad{if input.AlreadyRetried{result.ReviewUnitIds=append(result.ReviewUnitIds,unit.UnitId)}else{result.RetryUnitIds=append(result.RetryUnitIds,unit.UnitId)}}
 };return result,nil
}
