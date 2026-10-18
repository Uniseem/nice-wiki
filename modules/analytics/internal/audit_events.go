package analytics

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"time"
)

type EventDelivery = platform.EventDelivery
type EventHandled = platform.EventHandled

// AuditRecordedPayload对应本域types，生产者原事务outbox经jobs送来；不接原文或用户proof。
type AuditRecordedPayload struct {
	AuditID           string `json:"auditId"`
	ActorID           string `json:"actorId"`
	ActorKind         string `json:"actorKind"`
	OccurredAt        string `json:"occurredAt"`
	SourceOperationID string `json:"sourceOperationId"`
	SiteID            string `json:"siteId"`
	Action            string `json:"action"`
	ResourceID        string `json:"resourceId"`
	ResultCode        string `json:"resultCode"`
	Metadata          string `json:"metadata"`
	DataEpoch         string `json:"dataEpoch"`
}

func normalizeAuditFact(ctx context.Context, op OperationContext, origin string, payload AuditRecordedPayload, historical bool) (AuditRecord, error) {
	if _, err := ValidateAudit(ctx, op, ValidateAuditInput{VerifiedOrigin: origin, Historical: historical, AuditID: payload.AuditID, ActorID: payload.ActorID, Action: payload.Action, ResourceID: payload.ResourceID, ResultCode: payload.ResultCode, OccurredAt: payload.OccurredAt}); err != nil {
		return AuditRecord{}, err
	}
	if payload.ActorKind != "admin" && payload.ActorKind != "visitor" && payload.ActorKind != "service" || !platform.ValidIdentifier(payload.SourceOperationID) {
		return AuditRecord{}, platform.NewError(platform.CodeValidation, false, "原审计主体类别或操作引用不合法")
	}
	safe, err := RedactAuditMetadata(ctx, op, RedactAuditMetadataInput{Action: payload.Action, Metadata: payload.Metadata})
	if err != nil {
		return AuditRecord{}, err
	}
	at, _ := time.Parse(time.RFC3339Nano, payload.OccurredAt)
	code := payload.ResultCode
	if code == "SUCCESS" {
		code = "OK"
	}
	return AuditRecord{SchemaVersion: 2, SiteID: payload.SiteID, Producer: origin, ActorKind: payload.ActorKind, SourceOperationID: payload.SourceOperationID, OriginEpoch: payload.DataEpoch, AuditID: payload.AuditID, ActorID: payload.ActorID, Action: payload.Action, ResourceID: payload.ResourceID, ResultCode: code, OccurredAt: at.UTC().Format(analyticsTimeLayout), Metadata: safe.SafeMetadata}, nil
}

// ConsumeAuditEvent的schema由公共订阅边界一次校验；这里只核本入口独有的来源/站点/版本绑定。
func ConsumeAuditEvent(ctx context.Context, op OperationContext, input EventDelivery) (EventHandled, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return EventHandled{}, err
	}
	event := input.Envelope
	if event.EventType != event.Producer+".audit-recorded" || event.EventVersion != 1 || event.Revision != 1 {
		return EventHandled{}, platform.NewError(platform.CodeForbidden, false, "审计事件来源或版本不合法")
	}
	var payload AuditRecordedPayload
	if json.Unmarshal(event.Payload, &payload) != nil {
		return EventHandled{}, platform.NewError(platform.CodeIntegrityError, false, "审计事实载荷不可读")
	}
	if event.DataEpoch != rt.Epoch || payload.DataEpoch != rt.Epoch || payload.SiteID != rt.SiteID || event.SubjectID != payload.AuditID {
		return EventHandled{}, platform.NewError(platform.CodeVersionMismatch, false, "审计事实站点/代次或引用不匹配")
	}
	record, err := normalizeAuditFact(ctx, op, event.Producer, payload, true)
	if err != nil {
		return EventHandled{}, err
	}
	_, applied, err := appendTrustedAudit(ctx, op, trustedAuditFact{Record: record, Historical: true}, &event)
	return EventHandled{Applied: applied}, err
}
