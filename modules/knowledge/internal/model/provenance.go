package model

import (
 "strings"
 "unicode/utf8"
 "nice-wiki/libs/platform"
)

// WinningModelSnapshot 只信MG真实resultAttemptId与opaque快照hash，不按费用状态猜winning attempt。
func WinningModelSnapshot(outcome InvokeModelResult)(string,error) {
 var current *platform.Invocation
 if outcome.CurrentSnapshot!=nil {current=&outcome.CurrentSnapshot.Invocation} else {for _,event:=range outcome.Events {if event.Invocation!=nil {current=event.Invocation}}}
 if current==nil||current.ResultAttemptID==nil {return "",platform.NewError(platform.CodeDependencyUnavailable,false,"原模型结果缺实际winning来源证明")}
 for _,attempt:=range current.Attempts {if attempt.AttemptID==*current.ResultAttemptID {if attempt.ModelSnapshotHash=="" {return "",platform.NewError(platform.CodeDependencyUnavailable,false,"原模型结果缺实际模型快照证明")};return attempt.ModelSnapshotHash,nil}}
 return "",platform.NewError(platform.CodeIntegrityError,false,"模型winning来源不在原调用尝试中")
}

// HasCompleteResult 分离有效输出与费用状态；unknown只有真实完整结果和来源时才可保留使用。
func HasCompleteResult(outcome InvokeModelResult)bool {
 if _,err:=WinningModelSnapshot(outcome);err!=nil {return false}
 if outcome.CurrentSnapshot!=nil {return outcome.CurrentSnapshot.Output!=nil}
 if outcome.State!="completed"&&outcome.State!="charge_unknown" {return false}
 for _,event:=range outcome.Events {if event.Event==platform.StreamCompleted {return true}}
 return false
}

// CompletedText 优先真实完整Output.Text，指针区分完整空文本与缺失；不把部分delta拼成当前快照。
// 没有CurrentSnapshot的旧本域已保存结果仅兼容原真实completed事件全文。
func CompletedText(outcome InvokeModelResult,maximumBytes int)(string,error) {
 if !HasCompleteResult(outcome) {return "",platform.NewError(platform.CodeDependencyUnavailable,false,"模型文字尚无完整结果证明，保留原调用")}
 if outcome.CurrentSnapshot!=nil {
  output:=outcome.CurrentSnapshot.Output
  if output==nil||output.Text==nil {return "",platform.NewError(platform.CodeDependencyUnavailable,false,"当前模型快照没有完整文字证明")}
  if len(*output.Text)>maximumBytes||!utf8.ValidString(*output.Text) {return "",platform.NewError(platform.CodeIntegrityError,false,"当前完整模型文字超过任务预算或编码不合法")};return *output.Text,nil
 }
 var text strings.Builder
 for _,event:=range outcome.Events {if event.Event==platform.StreamDelta {if text.Len()>maximumBytes-len(event.Delta) {return "",platform.NewError(platform.CodeIntegrityError,false,"原完整模型文字超过任务预算")};text.WriteString(event.Delta)}}
 if !utf8.ValidString(text.String()) {return "",platform.NewError(platform.CodeIntegrityError,false,"原完整模型文字编码不合法")};return text.String(),nil
}

// ModelObservationState 只投影真实调用及全部attempt；Outcome.completed是结果可消费，不代表费用终局。
func ModelObservationState(outcome InvokeModelResult)string {
 var current *platform.Invocation
 if outcome.CurrentSnapshot!=nil {current=&outcome.CurrentSnapshot.Invocation} else {for _,event:=range outcome.Events {if event.Invocation!=nil {current=event.Invocation}}}
 if current==nil {return "pending"}
 for _,attempt:=range current.Attempts {if attempt.State==platform.InvocationChargeUnknown {return "charge_unknown"}}
 for _,attempt:=range current.Attempts {switch attempt.State {case platform.InvocationSucceeded,platform.InvocationFailed,platform.InvocationCancelled,"reconciled":default:return "pending"}}
 switch current.State {case platform.InvocationSucceeded:return "completed";case platform.InvocationFailed,"reconciled":return "failed";case platform.InvocationCancelled:return "cancelled";case platform.InvocationChargeUnknown:return "charge_unknown"}
 return current.State
}
