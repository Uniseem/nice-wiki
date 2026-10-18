package provider

import ("context";"nice-wiki/libs/platform")
type OperationContext=platform.OperationContext

type ClassifyFailureInput struct{HTTPStatus int `json:"httpStatus"`;ProviderCode string `json:"providerCode"`;BytesSent int `json:"bytesSent"`;ResponseStarted bool `json:"responseStarted"`;AcceptanceProof string `json:"acceptanceProof"`;UsageRecorded bool `json:"usageRecorded"`}
type ClassifyFailureResult struct{Classification string `json:"classification"`;RetryAfterMS int `json:"retryAfterMs"`;RetainPreviousReservation bool `json:"retainPreviousReservation"`}
func ClassifyFailure(ctx context.Context,op OperationContext,input ClassifyFailureInput)(ClassifyFailureResult,error){
 if input.AcceptanceProof=="unknown"||(input.AcceptanceProof=="accepted"&&!input.UsageRecorded){return ClassifyFailureResult{Classification:"charge_unknown",RetainPreviousReservation:true},nil}
 if input.ResponseStarted{return ClassifyFailureResult{Classification:"terminal"},nil}
 if input.ProviderCode=="invalid_structured_output"&&input.UsageRecorded{return ClassifyFailureResult{Classification:"retry_same"},nil}
 if input.HTTPStatus==0||input.HTTPStatus==429||input.HTTPStatus>=500{return ClassifyFailureResult{Classification:"fallback"},nil}
 return ClassifyFailureResult{Classification:"terminal"},nil
}
