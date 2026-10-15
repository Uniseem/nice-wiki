// Package catalog 实现供应方目录的纯校验与CAS保存。
// 本切片只保存渠道元数据和不可变事件，不连接真实供应方、不读取密钥明文。
package catalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// OperationContext 是服务间可信操作上下文的本域别名。
type OperationContext = platform.OperationContext

// Provider 复用OpenAPI生成的供应方类型，避免重新声明公共字段。
type Provider = gen.Provider

type ValidateProviderInput struct {
	Provider Provider `json:"provider"`
}

type ValidateProviderResult struct {
	Valid             bool   `json:"valid"`
	NormalizedBaseURL string `json:"normalizedBaseUrl"`
}

type PutProviderInput struct {
	Provider        Provider `json:"provider"`
	ExpectedVersion int      `json:"expectedVersion"`
	IdempotencyKey  string   `json:"idempotencyKey"`
}

type PutProviderResult struct {
	Provider Provider `json:"provider"`
}

const (
	providerFunctionID   = "model-gateway.PutProvider"
	providerEventType    = "provider.changed"
	providerEventVersion = 1
	maxDisplayNameRunes  = 65536
	maxHeaderNameRunes   = 4096
	maxSecretRefRunes    = 4096
)

// ValidateProvider 只验证渠道元数据和安全URL边界，不执行网络探测。
func ValidateProvider(ctx context.Context, op OperationContext, input ValidateProviderInput) (ValidateProviderResult, error) {
	if _, cancel, err := operationContext(ctx, op, 5*time.Second); err != nil {
		return ValidateProviderResult{}, err
	} else {
		cancel()
	}
	provider := input.Provider
	if !platform.ValidIdentifier(provider.ProviderId) || provider.Version < 0 || provider.CredentialVersion < 0 ||
		provider.ConcurrencyLimit < 0 || provider.ConnectTimeoutMs < 0 || provider.ConnectTimeoutMs > 10000 ||
		provider.RequestTimeoutMs < 0 || provider.RequestTimeoutMs > 180000 || provider.StreamIdleTimeoutMs < 0 {
		return ValidateProviderResult{}, platform.NewError(platform.CodeValidation, false, "供应方数值或标识不合法")
	}
	if !validProviderAdapter(provider.Adapter) || !validDisplayName(provider.DisplayName) {
		return ValidateProviderResult{}, platform.NewError(platform.CodeValidation, false, "供应方协议或名称不合法")
	}
	if len(provider.ExtraHeaderNames) > 256 || len(provider.ExtraHeaderSecretRefs) > 256 || len(provider.ExtraHeaderNames) != len(provider.ExtraHeaderSecretRefs) {
		return ValidateProviderResult{}, platform.NewError(platform.CodeValidation, false, "供应方附加头配置不完整")
	}
	for _, name := range provider.ExtraHeaderNames {
		if !validHeaderName(name) || strings.EqualFold(name, "authorization") {
			return ValidateProviderResult{}, platform.NewError(platform.CodeValidation, false, "供应方附加头不允许覆盖授权头")
		}
	}
	for _, ref := range provider.ExtraHeaderSecretRefs {
		if !validBoundedString(ref, maxSecretRefRunes) {
			return ValidateProviderResult{}, platform.NewError(platform.CodeValidation, false, "供应方附加密钥引用不合法")
		}
	}
	normalized, err := normalizeProviderURL(provider.BaseUrl)
	if err != nil {
		return ValidateProviderResult{}, err
	}
	return ValidateProviderResult{Valid: true, NormalizedBaseURL: normalized}, nil
}

// PutProvider 在单个SQLite事务内保存渠道配置、幂等回执和provider.changed
// outbox。它不会验证或保存任何密钥明文，也不会触发供应方调用。
func PutProvider(ctx context.Context, op OperationContext, input PutProviderInput) (PutProviderResult, error) {
	operationCtx, cancel, err := operationContext(ctx, op, 5*time.Second)
	if err != nil {
		return PutProviderResult{}, err
	}
	defer cancel()
	if err := requireSuperadmin(op); err != nil {
		return PutProviderResult{}, err
	}
	validated, err := ValidateProvider(operationCtx, op, ValidateProviderInput{Provider: input.Provider})
	if err != nil {
		return PutProviderResult{}, err
	}
	if input.ExpectedVersion < 0 || input.IdempotencyKey == "" || !validBoundedString(input.IdempotencyKey, 65536) {
		return PutProviderResult{}, platform.NewError(platform.CodeValidation, false, "供应方保存幂等参数不合法")
	}
	if input.Provider.Version != input.ExpectedVersion+1 {
		return PutProviderResult{}, platform.NewError(platform.CodeVersionMismatch, false, "供应方版本必须为预期版本加一")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return PutProviderResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	if err := store.CheckDataEpoch(operationCtx, op.DataEpoch); err != nil {
		return PutProviderResult{}, err
	}
	tx, err := store.BeginTx(operationCtx)
	if err != nil {
		return PutProviderResult{}, err
	}
	rollback := func(cause error) (PutProviderResult, error) {
		_ = tx.Rollback()
		return PutProviderResult{}, cause
	}
	if err := rejectDrainingBarrier(operationCtx, tx, op.DataEpoch); err != nil {
		return rollback(err)
	}
	inputHash, err := hashJSON(struct {
		Provider        Provider `json:"provider"`
		ExpectedVersion int      `json:"expectedVersion"`
	}{input.Provider, input.ExpectedVersion})
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "供应方输入摘要计算失败"))
	}
	if result, found, err := readProviderReceipt(operationCtx, tx, op, input.IdempotencyKey, inputHash); err != nil {
		return rollback(err)
	} else if found {
		if err := tx.Commit(); err != nil {
			return PutProviderResult{}, platform.NewError(platform.CodeInternal, true, "提交供应方幂等读取失败")
		}
		return result, nil
	}

	var currentVersion int
	err = tx.QueryRowContext(operationCtx, `SELECT version FROM providers WHERE provider_id = ?`, string(input.Provider.ProviderId)).Scan(&currentVersion)
	switch {
	case errors.Is(err, sql.ErrNoRows) && input.ExpectedVersion != 0:
		return rollback(platform.NewError(platform.CodeNotFound, false, "供应方不存在"))
	case err == nil && currentVersion != input.ExpectedVersion:
		return rollback(platform.NewError(platform.CodeVersionMismatch, false, "供应方版本已变化"))
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return rollback(platform.NewError(platform.CodeInternal, true, "读取供应方版本失败"))
	}

	provider := input.Provider
	provider.BaseUrl = validated.NormalizedBaseURL
	names, _ := json.Marshal(provider.ExtraHeaderNames)
	refs, _ := json.Marshal(provider.ExtraHeaderSecretRefs)
	if _, err := tx.ExecContext(operationCtx, `
		INSERT INTO providers (
			provider_id, version, adapter, base_url, enabled, credential_version,
			connect_timeout_ms, request_timeout_ms, display_name,
			extra_header_names_json, extra_header_secret_refs_json,
			concurrency_limit, stream_idle_timeout_ms, data_epoch, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			version = excluded.version, adapter = excluded.adapter, base_url = excluded.base_url,
			enabled = excluded.enabled, credential_version = excluded.credential_version,
			connect_timeout_ms = excluded.connect_timeout_ms, request_timeout_ms = excluded.request_timeout_ms,
			display_name = excluded.display_name, extra_header_names_json = excluded.extra_header_names_json,
			extra_header_secret_refs_json = excluded.extra_header_secret_refs_json,
			concurrency_limit = excluded.concurrency_limit, stream_idle_timeout_ms = excluded.stream_idle_timeout_ms,
			data_epoch = excluded.data_epoch, updated_at = excluded.updated_at`,
		string(provider.ProviderId), provider.Version, string(provider.Adapter), provider.BaseUrl,
		boolInt(provider.Enabled), provider.CredentialVersion, provider.ConnectTimeoutMs,
		provider.RequestTimeoutMs, provider.DisplayName, names, refs, provider.ConcurrencyLimit,
		provider.StreamIdleTimeoutMs, string(op.DataEpoch), string(platform.Now())); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "保存供应方配置失败"))
	}

	payload, err := json.Marshal(struct {
		ProviderID string `json:"providerId"`
		Version    int    `json:"version"`
		Enabled    bool   `json:"enabled"`
		DataEpoch  string `json:"dataEpoch"`
	}{string(provider.ProviderId), provider.Version, provider.Enabled, string(op.DataEpoch)})
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "供应方事件编码失败"))
	}
	if err := outbox.EnqueueEvent(tx, platform.EventEnvelope{
		EventID:      platform.NewIdentifier(),
		EventType:    providerEventType,
		EventVersion: providerEventVersion,
		Producer:     "model-gateway",
		SubjectID:    platform.Identifier(provider.ProviderId),
		Revision:     platform.Revision(provider.Version),
		OccurredAt:   platform.Now(),
		DataEpoch:    op.DataEpoch,
		Payload:      payload,
	}); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "保存供应方事件失败"))
	}
	result := PutProviderResult{Provider: provider}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "供应方回执编码失败"))
	}
	if _, err := tx.ExecContext(operationCtx, `
		INSERT INTO operation_receipts
			(site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, string(op.Actor.SiteID), string(op.Actor.SubjectID), providerFunctionID,
		input.IdempotencyKey, inputHash, resultJSON, string(platform.Now())); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "保存供应方幂等回执失败"))
	}
	if err := tx.Commit(); err != nil {
		return PutProviderResult{}, platform.NewError(platform.CodeInternal, true, "提交供应方配置失败")
	}
	return result, nil
}

func operationContext(ctx context.Context, op OperationContext, maximum time.Duration) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, func() {}, platform.NewError(platform.CodeValidation, false, "模型网关上下文为空")
	}
	if err := contextError(ctx); err != nil {
		return nil, func() {}, err
	}
	if op.Deadline == "" {
		return nil, func() {}, platform.NewError(platform.CodeValidation, false, "模型网关操作期限缺失")
	}
	opDeadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return nil, func() {}, platform.NewError(platform.CodeValidation, false, "模型网关操作期限不合法")
	}
	deadline := time.Now().UTC().Add(maximum)
	if opDeadline.Before(deadline) {
		deadline = opDeadline
	}
	if !time.Now().UTC().Before(deadline) {
		return nil, func() {}, platform.NewError(platform.CodeDeadlineExceeded, true, "模型网关操作已超过期限")
	}
	operationCtx, cancel := context.WithDeadline(ctx, deadline)
	return operationCtx, cancel, nil
}

func requireSuperadmin(op OperationContext) error {
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "superadmin") {
		return platform.NewError(platform.CodeForbidden, false, "仅超管可以修改供应方配置")
	}
	if !platform.ValidIdentifier(op.Actor.SiteID) || !platform.ValidIdentifier(op.Actor.SubjectID) {
		return platform.NewError(platform.CodeUnauthenticated, false, "管理主体无效")
	}
	if !platform.ValidIdentifier(op.DataEpoch) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "运行数据代次不可用")
	}
	return nil
}

func hasGrant(grants []string, wanted string) bool {
	for _, grant := range grants {
		if grant == wanted {
			return true
		}
	}
	return false
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "模型网关操作已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "模型网关操作已取消")
	default:
		return nil
	}
}

func validProviderAdapter(adapter gen.ProviderAdapter) bool {
	switch adapter {
	case gen.ProviderAdapterOpenaiChat, gen.ProviderAdapterAnthropicMessages, gen.ProviderAdapterGemini,
		gen.ProviderAdapterOpenaiEmbeddings, gen.ProviderAdapterGeminiEmbeddings, gen.ProviderAdapterRerank:
		return true
	default:
		return false
	}
}

func validDisplayName(value string) bool {
	return validBoundedString(value, maxDisplayNameRunes)
}

func validBoundedString(value string, maxRunes int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= maxRunes
}

func validHeaderName(value string) bool {
	return validBoundedString(value, maxHeaderNameRunes) && http.CanonicalHeaderKey(value) != ""
}

func normalizeProviderURL(raw string) (string, error) {
	if !validBoundedString(raw, 2048) {
		return "", platform.NewError(platform.CodeValidation, false, "供应方URL不合法")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Opaque != "" {
		return "", platform.NewError(platform.CodeValidation, false, "供应方URL必须为无凭证HTTPS根地址")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	if u.Path == "" {
		u.Path = ""
	}
	return u.String(), nil
}

func rejectDrainingBarrier(ctx context.Context, tx *sql.Tx, dataEpoch platform.Identifier) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM barriers WHERE state <> 'released' AND (data_epoch = ? OR data_epoch = '')`, string(dataEpoch)).Scan(&count); err != nil {
		return platform.NewError(platform.CodeInternal, true, "读取模型网关屏障失败")
	}
	if count > 0 {
		return platform.NewError(platform.CodeLocked, false, "模型网关屏障正在排空")
	}
	return nil
}

func hashJSON(value any) (platform.Hash, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return platform.Hash(hex.EncodeToString(sum[:])), nil
}

func readProviderReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, key string, inputHash platform.Hash) (PutProviderResult, bool, error) {
	var storedHash string
	var resultJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT input_hash, result_json FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		string(op.Actor.SiteID), string(op.Actor.SubjectID), providerFunctionID, key).Scan(&storedHash, &resultJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return PutProviderResult{}, false, nil
	}
	if err != nil {
		return PutProviderResult{}, false, platform.NewError(platform.CodeInternal, true, "读取供应方幂等回执失败")
	}
	if storedHash != string(inputHash) {
		return PutProviderResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同供应方配置")
	}
	var result PutProviderResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return PutProviderResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "供应方幂等回执损坏")
	}
	return result, true, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
