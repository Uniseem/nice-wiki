package trafficguard

import (
 "context"
 "nice-wiki/libs/platform"
)

type ClassifyActionInput struct { RouteID string }
type ClassifyActionResult struct { Action string }

// ClassifyAction 使用生产方OpenAPI已登记的三个匿名动作，不接受浏览器覆盖。
func ClassifyAction(ctx context.Context,op OperationContext,in ClassifyActionInput)(ClassifyActionResult,error) {
 switch in.RouteID {
 case "qa.createConversation": return ClassifyActionResult{"qa.conversation"},nil
 case "qa.startTurn": return ClassifyActionResult{"qa.ask"},nil
 case "localization.suggestLanguage": return ClassifyActionResult{"localization.suggest"},nil
 default: return ClassifyActionResult{},platform.NewError(platform.CodeValidation,false,"UNKNOWN_ADMISSION_ACTION")
 }
}
