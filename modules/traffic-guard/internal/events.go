package trafficguard

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
)

type ConsumeIdentityEventInput struct{ Event platform.EventEnvelope }
type ConsumeIdentityEventResult struct{ Accepted bool }

// ConsumeIdentityEvent 只存撤销通知的最大修订；管理写入仍即时查询identity权威。
func ConsumeIdentityEvent(ctx context.Context, op OperationContext, in ConsumeIdentityEventInput) (ConsumeIdentityEventResult, error) {
	if op.CallerService != "traffic-guard" || in.Event.Producer != "identity" || in.Event.EventType != "identity.authorization.changed" || in.Event.EventVersion != 1 {
		return ConsumeIdentityEventResult{}, platform.NewError(platform.CodeForbidden, false, "身份通知来源或版本不符")
	}
	if in.Event.DataEpoch != op.DataEpoch {
		return ConsumeIdentityEventResult{}, platform.NewError(platform.CodeVersionMismatch, false, "身份通知代次不一致")
	}
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.ValidateIdentityEvent == nil {
		return ConsumeIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份通知载荷校验未装配")
	}
	if err := runtime.ValidateIdentityEvent(in.Event.Payload); err != nil {
		return ConsumeIdentityEventResult{}, err
	}
	var payload struct {
		AccountID          string `json:"accountId"`
		SiteID             string `json:"siteId"`
		PermissionRevision int64  `json:"permissionRevision"`
	}
	if json.Unmarshal(in.Event.Payload, &payload) != nil {
		return ConsumeIdentityEventResult{}, platform.NewError(platform.CodeValidation, false, "身份通知不可读")
	}
	if payload.SiteID != op.Actor.SiteID || payload.AccountID != in.Event.SubjectID || payload.PermissionRevision != in.Event.Revision {
		return ConsumeIdentityEventResult{}, platform.NewError(platform.CodeValidation, false, "身份通知主题或修订不一致")
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return ConsumeIdentityEventResult{}, err
	}
	defer t.close()
	claimed, err := inbox.ClaimInbox(t.tx, in.Event)
	if err != nil {
		return ConsumeIdentityEventResult{}, err
	}
	if claimed {
		_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_identity_revisions(site_id,data_epoch,account_id,permission_revision) VALUES(?,?,?,?)
   ON CONFLICT(site_id,data_epoch,account_id) DO UPDATE SET permission_revision=MAX(guard_identity_revisions.permission_revision,excluded.permission_revision)`, t.site, op.DataEpoch, payload.AccountID, payload.PermissionRevision)
		if err != nil {
			return ConsumeIdentityEventResult{}, loginDBError()
		}
	}
	if err = t.commit(); err != nil {
		return ConsumeIdentityEventResult{}, err
	}
	return ConsumeIdentityEventResult{Accepted: true}, nil
}
