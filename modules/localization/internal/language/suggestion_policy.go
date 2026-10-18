package language

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"nice-wiki/libs/platform"
)

type LanguageSuggestionPolicy struct {
	Enabled    bool               `json:"enabled"`
	Version    int64              `json:"version"`
	ApprovalID string             `json:"approvalId,omitempty"`
	SiteID     string             `json:"siteId"`
	DataEpoch  string             `json:"dataEpoch"`
	ApprovedAt platform.Timestamp `json:"approvedAt,omitempty"`
}
type ReadLanguageSuggestionPolicyInput struct{}
type PutLanguageSuggestionPolicyInput struct {
	Enabled         bool   `json:"enabled"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}
type ReadLanguageSuggestionApprovalInput struct {
	ApprovalID string `json:"approvalId"`
}
type ReadLanguageSuggestionApprovalResult = LanguageSuggestionPolicy

func readSuggestionPolicy(ctx context.Context, tx *sql.Tx, op OperationContext) (LanguageSuggestionPolicy, error) {
	result := LanguageSuggestionPolicy{SiteID: op.Actor.SiteID, DataEpoch: op.DataEpoch}
	var enabled int
	err := tx.QueryRowContext(ctx, `SELECT a.enabled,p.version,p.approval_id,a.approved_at
 FROM suggestion_policy p JOIN suggestion_approvals a ON a.approval_id=p.approval_id AND a.site_id=p.site_id AND a.data_epoch=p.data_epoch AND a.version=p.version
 WHERE p.site_id=? AND p.data_epoch=?`, op.Actor.SiteID, op.DataEpoch).Scan(&enabled, &result.Version, &result.ApprovalID, &result.ApprovedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM suggestion_policy WHERE site_id=? AND data_epoch=?)`, op.Actor.SiteID, op.DataEpoch).Scan(&exists); err != nil {
			return result, StorageError(err)
		}
		if exists {
			return result, platform.NewError(platform.CodeIntegrityError, false, "语言建议批准指针损坏")
		}
		return result, nil
	}
	if err != nil {
		return result, StorageError(err)
	}
	result.Enabled = enabled == 1
	return result, nil
}

func ReadLanguageSuggestionPolicy(ctx context.Context, op OperationContext, input ReadLanguageSuggestionPolicyInput) (LanguageSuggestionPolicy, error) {
	if err := RequireAdministrator(op); err != nil {
		return LanguageSuggestionPolicy{}, err
	}
	var result LanguageSuggestionPolicy
	err := ReadData(ctx, op, func(tx *sql.Tx) error { var err error; result, err = readSuggestionPolicy(ctx, tx, op); return err })
	return result, err
}

// PutLanguageSuggestionPolicy 只接受管理员明确开启，不把站点内容偏好转成付费批准。
func PutLanguageSuggestionPolicy(ctx context.Context, op OperationContext, input PutLanguageSuggestionPolicyInput) (LanguageSuggestionPolicy, error) {
	if err := RequireAdministrator(op); err != nil {
		return LanguageSuggestionPolicy{}, err
	}
	var result LanguageSuggestionPolicy
	err := Mutate(ctx, op, "localization.PutLanguageSuggestionPolicy", input.IdempotencyKey, input, &result, func(tx *sql.Tx) error {
		current, err := readSuggestionPolicy(ctx, tx, op)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return platform.NewError(platform.CodeVersionMismatch, false, "语言建议开关版本已变化")
		}
		result = LanguageSuggestionPolicy{Enabled: input.Enabled, Version: current.Version + 1, ApprovalID: platform.NewIdentifier(), SiteID: op.Actor.SiteID, DataEpoch: op.DataEpoch, ApprovedAt: platform.Now()}
		if _, err := tx.ExecContext(ctx, `INSERT INTO suggestion_approvals(approval_id,site_id,data_epoch,version,enabled,approved_at,approved_by,permission_revision,root_operation_id) VALUES(?,?,?,?,?,?,?,?,?)`, result.ApprovalID, result.SiteID, result.DataEpoch, result.Version, boolInt(result.Enabled), result.ApprovedAt, op.Actor.SubjectID, op.Actor.PermissionRevision, op.RootOperationID); err != nil {
			return StorageError(err)
		}
		saved, err := tx.ExecContext(ctx, `INSERT INTO suggestion_policy(site_id,data_epoch,version,approval_id) VALUES(?,?,?,?) ON CONFLICT(site_id,data_epoch) DO UPDATE SET version=excluded.version,approval_id=excluded.approval_id WHERE suggestion_policy.version=?`, result.SiteID, result.DataEpoch, result.Version, result.ApprovalID, input.ExpectedVersion)
		if err != nil {
			return StorageError(err)
		}
		count, err := saved.RowsAffected()
		if err != nil {
			return StorageError(err)
		}
		if count != 1 {
			return platform.NewError(platform.CodeVersionMismatch, false, "语言建议开关并发改变")
		}
		return nil
	})
	return result, err
}

// ReadLanguageSuggestionApproval 让网关从持久权威核每个新attempt的许可。
func ReadLanguageSuggestionApproval(ctx context.Context, op OperationContext, input ReadLanguageSuggestionApprovalInput) (ReadLanguageSuggestionApprovalResult, error) {
	if op.CallerService != "model-gateway" || op.Actor.Kind != "service" || op.Actor.SubjectID != "model-gateway" {
		return LanguageSuggestionPolicy{}, platform.NewError(platform.CodeForbidden, false, "批准核验只允许模型网关纯服务")
	}
	var result LanguageSuggestionPolicy
	err := ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		result, err = readSuggestionPolicy(ctx, tx, op)
		if err != nil {
			return err
		}
		if result.ApprovalID == "" || result.ApprovalID != input.ApprovalID {
			return platform.NewError(platform.CodeNotFound, false, "当前语言建议批准不存在")
		}
		return nil
	})
	return result, err
}

func NewSuggestionPolicyHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"readLanguageSuggestionPolicy": JSONHandler(ReadLanguageSuggestionPolicy),
		"putLanguageSuggestionPolicy": JSONHandler(PutLanguageSuggestionPolicy, func(input *PutLanguageSuggestionPolicyInput, r *http.Request) error {
			if key := r.Header.Get("Idempotency-Key"); key != "" && key != input.IdempotencyKey {
				return platform.NewError(platform.CodeValidation, false, "幂等头与正文不同")
			}
			return nil
		}),
		"readLanguageSuggestionApproval": JSONHandler(ReadLanguageSuggestionApproval, func(input *ReadLanguageSuggestionApprovalInput, r *http.Request) error {
			input.ApprovalID = r.PathValue("approvalId")
			return nil
		}),
	}
}
