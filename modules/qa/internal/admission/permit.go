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
	Valid          bool                     `json:"valid"`
	VisitorSubject string                   `json:"visitorSubject"`
	Permit         platform.AdmissionPermit `json:"permit"`
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
	freeConversation := input.Action == "qa.conversation"
	if !freeConversation && input.Action != "qa.ask" && input.Action != "qa.followup" {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "未登记的问答准入动作")
	}
	if freeConversation && permit.Token != "" {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "创建会话许可不能携带模型token")
	}
	expires, err := platform.ParseTimestamp(permit.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || permit.SubjectID != op.Actor.SubjectID || permit.RequestID != op.RootOperationID || permit.DataEpoch != op.DataEpoch || permit.Action != input.Action || permit.Audience != "model-gateway" || op.Actor.Kind != "visitor" {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "准入许可与当前动作或主体不匹配")
	}
	request := struct {
		RequestID string `json:"requestId"`
	}{op.RootOperationID}
	raw, err := runtime.Guard.ConsumeAdmission(qaruntime.ServiceReadContext(ctx, op), permit.AdmissionID, request, op.OperationID+":consume")
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
	signedExpires, err := platform.ParseTimestamp(consumed.Permit.ExpiresAt)
	if !consumed.Consumed || err != nil || !signedExpires.After(time.Now()) {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "准入消费回执缺少有效消费状态或保留期")
	}
	if freeConversation && consumed.Permit.Token != "" {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "免费会话消费不得签发模型token")
	}
	if !freeConversation && consumed.Permit.Token == "" {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "付费问答消费回执缺少模型准入签名")
	}
	// mint只有分配元数据；付费消费才签名，免费创建会话消费后仍为空token。
	expectedPermit := permit
	expectedPermit.Token = consumed.Permit.Token
	expectedPermit.ExpiresAt = consumed.Permit.ExpiresAt
	expected, _ := json.Marshal(expectedPermit)
	actual, _ := json.Marshal(consumed.Permit)
	if string(expected) != string(actual) {
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "准入消费回执与原分配元数据不一致")
	}
	raw, err = runtime.Guard.VerifyAdmission(qaruntime.ServiceReadContext(ctx, op), permit.AdmissionID, request, op.OperationID+":verify-consumed")
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
		return ValidateAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "已消费准入状态已失效")
	}
	return ValidateAdmissionResult{Valid: true, VisitorSubject: permit.SubjectID, Permit: consumed.Permit}, nil
}
