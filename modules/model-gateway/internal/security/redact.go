package security

import (
 "context"
 "strconv"
 "slices"
 "nice-wiki/libs/platform"
)

type RedactLogInput struct{FieldNames []string `json:"fieldNames"`;FieldValues []string `json:"fieldValues"`}
type RedactLogResult struct{SafeNames []string `json:"safeNames"`;SafeValues []string `json:"safeValues"`}
func RedactLog(ctx context.Context,op OperationContext,input RedactLogInput)(RedactLogResult,error){
 if len(input.FieldNames)!=len(input.FieldValues){return RedactLogResult{},platform.NewError(platform.CodeValidation,false,"日志字段名和值须等长")}
 result:=RedactLogResult{SafeNames:make([]string,0),SafeValues:make([]string,0)};seen:=make(map[string]bool)
 for i,name:=range input.FieldNames{
  value:=input.FieldValues[i];allowed:=false
  switch name{
  case "invocationId","rootOperationId","providerId","modelId":allowed=platform.ValidIdentifier(value)
  case "state":allowed=slices.Contains([]string{"accepted","reserved","dispatching","streaming","succeeded","failed","cancel_requested","cancelled","charge_unknown","reconciled"},value)
  case "code":_,allowed=platform.ProblemHTTPStatus[value]
  case "durationMs":duration,err:=strconv.ParseInt(value,10,64);allowed=err==nil&&duration>=0&&duration<=9007199254740991
  default:continue
  }
  if !allowed||seen[name]{return RedactLogResult{},platform.NewError(platform.CodeValidation,false,"安全日志字段值不合法或重复")};seen[name]=true;result.SafeNames=append(result.SafeNames,name);result.SafeValues=append(result.SafeValues,value)
 }
 return result,nil
}
