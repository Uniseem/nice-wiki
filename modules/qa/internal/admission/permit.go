package admission

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"time"
)

type OperationContext = platform.OperationContext
type ValidateAdmissionInput struct {
	Admission      platform.AdmissionPermit `json:"admission"`
	ConversationID string                   `json:"conversationId"`
	Action         string                   `json:"action"`
}
type ValidateAdmissionResult struct {
	Valid          bool   `json:"valid"`
	VisitorSubject string `json:"visitorSubject"`
}

// ValidateAdmission 在线核对并消费guard原许可，绝不把客户端许可当可信费用上下文。
func ValidateAdmission(ctx context.Context, op OperationContext, input ValidateAdmissionInput) (ValidateAdmissionResult, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return ValidateAdmissionResult{}, err
	}
	if runtime.Guard == nil {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "访客准入权威未装配")
	}
	permit := input.Admission
	expires, err := platform.ParseTimestamp(permit.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || permit.SubjectID != op.Actor.SubjectID || permit.RequestID != op.RequestID || permit.DataEpoch != op.DataEpoch || permit.Action != input.Action || permit.Audience != "model-gateway" || op.Actor.Kind != "visitor" {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "准入许可与当前动作或主体不匹配")
	}
	request := struct {
		RequestID string `json:"requestId"`
	}{op.RequestID}
	raw, err := runtime.Guard.VerifyAdmission(platform.WithOperationContext(ctx, op), permit.AdmissionID, request, op.OperationID+":verify")
	if err != nil {
		return ValidateAdmissionResult{}, err
	}
	var verification struct {
		AdmissionID string `json:"admissionId"`
		Valid       bool   `json:"valid"`
	}
	if err := qaruntime.DecodeResponse(runtime, "GuardValidatePermitResult", raw, &verification); err != nil {
		return ValidateAdmissionResult{}, err
	}
	if !verification.Valid || verification.AdmissionID != permit.AdmissionID {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "准入许可已无效")
	}
	raw, err = runtime.Guard.ConsumeAdmission(platform.WithOperationContext(ctx, op), permit.AdmissionID, request, op.OperationID+":consume")
	if err != nil {
		return ValidateAdmissionResult{}, err
	}
	var consumed struct {
		Consumed bool                     `json:"consumed"`
		Permit   platform.AdmissionPermit `json:"permit"`
	}
	if err := qaruntime.DecodeResponse(runtime, "GuardConsumeAdmissionResult", raw, &consumed); err != nil {
		return ValidateAdmissionResult{}, err
	}
	expected, _ := json.Marshal(permit)
	actual, _ := json.Marshal(consumed.Permit)
	if !consumed.Consumed || string(expected) != string(actual) {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "准入消费回执与原签名许可不一致")
	}
	return ValidateAdmissionResult{Valid: true, VisitorSubject: permit.SubjectID}, nil
}
