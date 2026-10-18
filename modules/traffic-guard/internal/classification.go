package trafficguard

import (
 "context"
 "nice-wiki/libs/platform"
)

type ClassifyActionInput struct { Action string }
type ClassifyActionResult struct { Action string;OwnerCaller string;RequiresAIProof bool }

// ClassifyAction 的动作由portal已登记路由产生；在本域映射拥有者与问答专属保护。
func ClassifyAction(ctx context.Context,op OperationContext,in ClassifyActionInput)(ClassifyActionResult,error) {
 switch in.Action {
 case "qa.conversation","qa.followup","qa.ask":return ClassifyActionResult{Action:in.Action,OwnerCaller:"qa",RequiresAIProof:true},nil
 case "localization.suggest":return ClassifyActionResult{Action:in.Action,OwnerCaller:"localization",RequiresAIProof:false},nil
 default:return ClassifyActionResult{},platform.NewError(platform.CodeValidation,false,"UNKNOWN_ADMISSION_ACTION")
 }
}
