// Package catalog 实现供应方目录的纯校验与CAS保存。
// 本切片只保存渠道元数据和不可变事件，不连接真实供应方、不读取密钥明文。
package catalog

import (
	"bytes"
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
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/security"
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
	Provider        Provider                 `json:"provider"`
	ExpectedVersion int                      `json:"expectedVersion"`
	IdempotencyKey  string                   `json:"idempotencyKey"`
	SecretHeaders   *[]security.SecretHeader `json:"secretHeaders,omitempty"`
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
	if input.ExpectedVersion >= 9007199254740991 || input.Provider.Version != input.ExpectedVersion+1 {
		return PutProviderResult{}, platform.NewError(platform.CodeVersionMismatch, false, "供应方版本必须为预期版本加一")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return PutProviderResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	var prepared []security.SealedHeader
	var secretDigest string
	if input.SecretHeaders != nil && len(*input.SecretHeaders) > 0 {
		prepared, secretDigest, err = security.PrepareHeaderSecrets(operationCtx, op, input.Provider.ProviderId, input.Provider.Version, *input.SecretHeaders, struct {
			Caller, Epoch string
			Input         PutProviderInput
		}{op.CallerService, op.DataEpoch, input})
		if err != nil {
			return PutProviderResult{}, err
		}
	}
	snapshot, err := beginAvailabilitySnapshot(operationCtx, op, store)
	if err != nil {
		return PutProviderResult{}, err
	}
	tx := snapshot.tx
	rollback := func(cause error) (PutProviderResult, error) {
		_ = tx.Rollback()
		return PutProviderResult{}, cause
	}
	provider := input.Provider
	provider.BaseUrl = validated.NormalizedBaseURL
	inputHash, err := hashJSON(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Provider        Provider            `json:"provider"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Provider, input.ExpectedVersion})
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "供应方输入摘要计算失败"))
	}
	if input.SecretHeaders != nil {
		if len(*input.SecretHeaders) > 0 {
			inputHash = secretDigest
		} else {
			inputHash, err = hashJSON(struct {
				Caller, Epoch string
				Input         PutProviderInput
			}{op.CallerService, op.DataEpoch, input})
			if err != nil {
				return rollback(err)
			}
		}
	}
	if result, found, err := readProviderReceipt(operationCtx, op, tx, input.IdempotencyKey, inputHash, provider); err != nil {
		return rollback(err)
	} else if found {
		if err := tx.Commit(); err != nil {
			return PutProviderResult{}, platform.NewError(platform.CodeInternal, true, "提交供应方幂等读取失败")
		}
		return result, nil
	}

	var blocked bool
	if err := tx.QueryRowContext(operationCtx, `SELECT EXISTS(SELECT 1 FROM barriers WHERE state <> 'released')`).Scan(&blocked); err != nil {
		return rollback(availabilityReadError(err))
	}
	if blocked {
		return rollback(platform.NewError(platform.CodeLocked, false, "供应方配置屏障尚未释放"))
	}

	var currentVersion, currentCredentialVersion int
	var currentEpoch string
	err = tx.QueryRowContext(operationCtx, `SELECT version, credential_version, data_epoch FROM providers WHERE provider_id = ?`, string(input.Provider.ProviderId)).Scan(&currentVersion, &currentCredentialVersion, &currentEpoch)
	switch {
	case errors.Is(err, sql.ErrNoRows) && input.ExpectedVersion != 0:
		return rollback(platform.NewError(platform.CodeNotFound, false, "供应方不存在"))
	case err == nil && currentVersion != input.ExpectedVersion:
		return rollback(platform.NewError(platform.CodeVersionMismatch, false, "供应方版本已变化"))
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return rollback(platform.NewError(platform.CodeInternal, true, "读取供应方版本失败"))
	}

	if err == nil && currentEpoch != op.DataEpoch {
		return rollback(platform.NewError(platform.CodeVersionMismatch, false, "供应方数据代次已变化"))
	}
	if input.Provider.CredentialVersion != currentCredentialVersion {
		return rollback(platform.NewError(platform.CodeValidation, false, "供应方编辑必须保留当前凭据版本"))
	}

	// 元数据中的refs只回显；省略secretHeaders始终保留原映射，不能把客户端refs当路径或凭据。
	provider.ExtraHeaderNames = make([]string, 0)
	provider.ExtraHeaderSecretRefs = make([]string, 0)
	if currentVersion > 0 {
		var namesRaw, refsRaw []byte
		if err := tx.QueryRowContext(operationCtx, `SELECT extra_header_names_json,extra_header_secret_refs_json FROM providers WHERE provider_id=?`, provider.ProviderId).Scan(&namesRaw, &refsRaw); err != nil {
			return rollback(availabilityReadError(err))
		}
		if json.Unmarshal(namesRaw, &provider.ExtraHeaderNames) != nil || json.Unmarshal(refsRaw, &provider.ExtraHeaderSecretRefs) != nil {
			return rollback(platform.NewError(platform.CodeIntegrityError, false, "原额外头映射损坏"))
		}
	}
	if input.SecretHeaders != nil {
		provider.ExtraHeaderNames = make([]string, 0, len(prepared))
		provider.ExtraHeaderSecretRefs = make([]string, 0, len(prepared))
		for _, header := range prepared {
			provider.ExtraHeaderNames = append(provider.ExtraHeaderNames, header.Name)
			provider.ExtraHeaderSecretRefs = append(provider.ExtraHeaderSecretRefs, header.RefID)
		}
		if err := security.SaveHeaderSecrets(operationCtx, op, tx, provider.ProviderId, provider.Version, prepared); err != nil {
			return rollback(err)
		}
	}
	names, _ := json.Marshal(provider.ExtraHeaderNames)
	refs, _ := json.Marshal(provider.ExtraHeaderSecretRefs)
	saved, err := tx.ExecContext(operationCtx, `
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
			updated_at = excluded.updated_at
		WHERE providers.version = ? AND providers.data_epoch = ? AND providers.credential_version = ?`,
		string(provider.ProviderId), provider.Version, string(provider.Adapter), provider.BaseUrl,
		boolInt(provider.Enabled), provider.CredentialVersion, provider.ConnectTimeoutMs,
		provider.RequestTimeoutMs, provider.DisplayName, names, refs, provider.ConcurrencyLimit,
		provider.StreamIdleTimeoutMs, string(op.DataEpoch), string(platform.Now()), input.ExpectedVersion, op.DataEpoch, currentCredentialVersion)
	if err != nil {
		return rollback(availabilityReadError(err))
	}
	if count, err := saved.RowsAffected(); err != nil {
		return rollback(availabilityReadError(err))
	} else if count != 1 {
		return rollback(platform.NewError(platform.CodeVersionMismatch, false, "供应方保存条件已变化"))
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
	if _, err := tx.ExecContext(operationCtx, `DELETE FROM operation_receipts WHERE site_id=? AND actor_id=? AND function_id=? AND idempotency_key=?`, op.Actor.SiteID, op.Actor.SubjectID, providerFunctionID, input.IdempotencyKey); err != nil {
		return rollback(availabilityReadError(err))
	}
	if _, err := tx.ExecContext(operationCtx, `
		INSERT INTO operation_receipts
			(site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at,result_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, string(op.Actor.SiteID), string(op.Actor.SubjectID), providerFunctionID,
		input.IdempotencyKey, inputHash, resultJSON, string(platform.Now()), platform.HashBytes(resultJSON)); err != nil {
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

// AllowLoopbackHTTP 只在本地开发（配置 allowLocalHTTP）时置位：允许供应方地址是回环地址的 http，便于接本机的假供应方联调。
var AllowLoopbackHTTP bool

func normalizeProviderURL(raw string) (string, error) {
	if !validBoundedString(raw, 2048) {
		return "", platform.NewError(platform.CodeValidation, false, "供应方URL不合法")
	}
	u, err := url.Parse(raw)
	loopback := err == nil && AllowLoopbackHTTP && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1")
	if err != nil || (u.Scheme != "https" && !loopback) || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Opaque != "" {
		return "", platform.NewError(platform.CodeValidation, false, "供应方URL必须为无凭证HTTPS根地址")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	if u.Path == "" {
		u.Path = ""
	}
	return u.String(), nil
}

func hashJSON(value any) (platform.Hash, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return platform.Hash(hex.EncodeToString(sum[:])), nil
}

func readProviderReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, key string, inputHash platform.Hash, expected Provider) (PutProviderResult, bool, error) {
	var storedHash, createdAt string
	var resultHash sql.NullString
	var resultJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT input_hash, result_json, created_at,result_hash FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, providerFunctionID, key).Scan(&storedHash, &resultJSON, &createdAt, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return PutProviderResult{}, false, nil
	}
	if err != nil {
		return PutProviderResult{}, false, availabilityReadError(err)
	}
	created, err := platform.ParseTimestamp(createdAt)
	now := time.Now().UTC()
	if err != nil || created.After(now) {
		return PutProviderResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "供应方回执时间损坏")
	}
	if now.Sub(created) > 7*24*time.Hour {
		return PutProviderResult{}, false, nil
	}
	if storedHash != inputHash {
		return PutProviderResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同供应方配置或代次")
	}
	if resultHash.Valid {
		var result PutProviderResult
		if resultHash.String != platform.HashBytes(resultJSON) || json.Unmarshal(resultJSON, &result) != nil || result.Provider.ProviderId != expected.ProviderId || result.Provider.Version != expected.Version {
			return PutProviderResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "供应方原安全回执损坏")
		}
		return result, true, nil
	}
	expectedJSON, _ := json.Marshal(PutProviderResult{Provider: expected})
	if !bytes.Equal(resultJSON, expectedJSON) {
		return PutProviderResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "供应方原结果与摘要不一致")
	}
	return PutProviderResult{Provider: expected}, true, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// NewCatalogHandlers 绑定既有供应方和模型保存入口，启停沿配置enabled字段写入。
func NewCatalogHandlers() map[string]http.Handler {
	handlers := make(map[string]http.Handler, 2)
	for _, operationID := range []string{"putProvider", "putModel"} {
		handlers[operationID] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var result any
			var err error
			switch operationID {
			case "putProvider":
				var input PutProviderInput
				if json.NewDecoder(r.Body).Decode(&input) != nil {
					err = platform.NewError(platform.CodeValidation, false, "供应方输入不可读")
				} else if r.PathValue("providerId") != input.Provider.ProviderId || (r.Header.Get("Idempotency-Key") != "" && r.Header.Get("Idempotency-Key") != input.IdempotencyKey) {
					err = platform.NewError(platform.CodeValidation, false, "供应方路径或幂等头与正文不一致")
				} else {
					result, err = PutProvider(r.Context(), op, input)
				}
			case "putModel":
				var input PutModelInput
				if json.NewDecoder(r.Body).Decode(&input) != nil {
					err = platform.NewError(platform.CodeValidation, false, "模型输入不可读")
				} else if r.PathValue("modelId") != input.Model.ModelId || (r.Header.Get("Idempotency-Key") != "" && r.Header.Get("Idempotency-Key") != input.IdempotencyKey) {
					err = platform.NewError(platform.CodeValidation, false, "模型路径或幂等头与正文不一致")
				} else {
					result, err = PutModel(r.Context(), op, input)
				}
			}
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		})
	}
	return handlers
}
