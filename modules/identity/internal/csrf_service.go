package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"

	"nice-wiki/libs/platform"
)

type VerifySessionCSRFInput struct {
	AccessToken string              `json:"accessToken"`
	SessionID   platform.Identifier `json:"sessionId,omitempty"`
	CSRFToken   string              `json:"csrfToken"`
	Origin      string              `json:"origin"`
}

type VerifySessionCSRFResult struct {
	Valid     bool               `json:"valid"`
	ExpiresAt platform.Timestamp `json:"expiresAt"`
}

// VerifySessionCSRF 绑定固定Origin、已验签access与当前会话的独立CSRF摘要。
func VerifySessionCSRF(ctx context.Context, op OperationContext, input VerifySessionCSRFInput) (VerifySessionCSRFResult, error) {
	if op.CallerService != "portal" {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeForbidden, false, "CSRF校验仅允许portal")
	}
	runtime, err := sessionRuntimeFromContext(ctx)
	if err != nil {
		return VerifySessionCSRFResult{}, err
	}
	if runtime.PublicOrigin == "" {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "固定公开Origin尚未装配")
	}
	if input.Origin != runtime.PublicOrigin {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeForbidden, false, "请求Origin不匹配")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "CSRF私有存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "CSRF读取事务不可用")
	}
	defer tx.Rollback()
	snapshot, err := verifySessionAccess(ctx, tx, runtime, op.DataEpoch, input.AccessToken)
	if err != nil {
		return VerifySessionCSRFResult{}, err
	}
	if snapshot.Status != "active" || (input.SessionID != "" && input.SessionID != snapshot.Session.SessionID) {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeUnauthenticated, false, "CSRF会话不可用")
	}
	digest := platform.HashBytes([]byte(input.CSRFToken))
	if subtle.ConstantTimeCompare([]byte(snapshot.CSRFDigest), []byte(digest)) != 1 {
		return VerifySessionCSRFResult{}, platform.NewError(platform.CodeForbidden, false, "CSRF凭据不匹配")
	}
	return VerifySessionCSRFResult{Valid: true, ExpiresAt: snapshot.Session.ExpiresAt}, nil
}
