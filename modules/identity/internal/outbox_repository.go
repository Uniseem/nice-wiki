package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
)

const identityAuthorizationChangedEvent = "identity.authorization.changed"

// RelayOutboxInput 是持续转发循环的单批参数。
type RelayOutboxInput struct {
	BatchLimit int `json:"batchLimit"`
}

// RelayOutboxResult 只在转发循环因进程上下文取消而安全退出时返回。
type RelayOutboxResult struct {
	Relayed     int64 `json:"relayed"`
	Pending     int64 `json:"pending"`
	Quarantined int64 `json:"quarantined"`
}

type relayRuntimeContextKey struct{}

type relayRuntime struct {
	store      platform.OutboxStore
	publisher  platform.EventPublisher
	snapshot   *lifecycle.SnapshotManager
	siteConfig platform.SiteConfigClient
}

// WithRelayRuntime 由本域启动装配注入真实存储、jobs客户端、运行权威和快照
// scope。这里只保存进程内句柄，不允许网络输入提供这些依赖。
func WithRelayRuntime(ctx context.Context, store *Store, publisher platform.EventPublisher, snapshot *lifecycle.SnapshotManager, siteConfig platform.SiteConfigClient) context.Context {
	var outboxStore platform.OutboxStore
	if store != nil && store.db != nil {
		outboxStore = outbox.NewStore(store.db)
	}
	return context.WithValue(ctx, relayRuntimeContextKey{}, relayRuntime{
		store: outboxStore, publisher: publisher, snapshot: snapshot, siteConfig: siteConfig,
	})
}

func relayRuntimeFromContext(ctx context.Context) (relayRuntime, bool) {
	runtime, ok := ctx.Value(relayRuntimeContextKey{}).(relayRuntime)
	return runtime, ok && runtime.store != nil && runtime.publisher != nil && runtime.snapshot != nil && runtime.siteConfig != nil
}

// IdentityChanged 是身份撤销事件载荷；不包含口令、IP或正文。
type IdentityChanged struct {
	AccountID          platform.Identifier `json:"accountId"`
	SiteID             platform.Identifier `json:"siteId"`
	PermissionRevision platform.Revision   `json:"permissionRevision"`
	CredentialRevision platform.Revision   `json:"credentialRevision"`
	Reason             string              `json:"reason"`
	OccurredAt         platform.Timestamp  `json:"occurredAt"`
}

// PublishIdentityEventInput 是身份撤销事件登记参数。
type PublishIdentityEventInput struct {
	Event   IdentityChanged     `json:"event"`
	EventID platform.Identifier `json:"eventId"`
}

// PublishIdentityEventResult 表示事件已在本域事务中排队。
type PublishIdentityEventResult struct {
	Queued bool `json:"queued"`
}

type identityTransactionContextKey struct{}

// WithIdentityTransaction 允许业务提交事务把事件登记与自身写入保持原子。
// 事务所有权仍归调用方；PublishIdentityEvent 不会提交或回滚外部事务。
func WithIdentityTransaction(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, identityTransactionContextKey{}, tx)
}

func identityTransactionFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(identityTransactionContextKey{}).(*sql.Tx)
	return tx, ok && tx != nil
}

// PublishIdentityEvent 在身份业务事务内登记唯一事件和幂等回执。
// 提交后的投递由 RelayOutbox 经 jobs 完成，本函数不直接调用 jobs。
func PublishIdentityEvent(ctx context.Context, op OperationContext, input PublishIdentityEventInput) (PublishIdentityEventResult, error) {
	if err := checkIdentityRepositoryContext(ctx, op); err != nil {
		return PublishIdentityEventResult{}, err
	}
	if err := validateIdentityChanged(input, op); err != nil {
		return PublishIdentityEventResult{}, err
	}

	var (
		tx          *sql.Tx
		ownedTx     bool
		store       *Store
		requestHash = platform.HashBytes(mustJSON(input))
		resultHash  = platform.HashBytes([]byte(`{"queued":true}`))
	)
	if externalTx, ok := identityTransactionFromContext(ctx); ok {
		tx = externalTx
	} else {
		var ok bool
		store, ok = identityStoreFromContext(ctx)
		if !ok || store.db == nil {
			return PublishIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份私有存储尚未装配")
		}
		if err := outbox.EnsureSchema(ctx, store.db); err != nil {
			return PublishIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份事件outbox不可用")
		}
		var err error
		tx, err = store.db.BeginTx(ctx, nil)
		if err != nil {
			return PublishIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份事件事务不可用")
		}
		ownedTx = true
	}
	if ownedTx {
		defer func() { _ = tx.Rollback() }()
	}

	// 外部业务事务拥有请求摘要和完整结果回执，本函数只追加事件。
	// 自有事务仍维持事件登记操作本身的幂等回执。
	var existingDigest, state string
	var err error
	if ownedTx {
		err = tx.QueryRowContext(ctx, `
		SELECT input_digest, state
		FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND operation_id = ?`,
			op.Actor.SiteID, op.Actor.SubjectID, op.OperationID,
		).Scan(&existingDigest, &state)
		switch {
		case err == nil:
			if existingDigest != requestHash {
				return PublishIdentityEventResult{}, platform.NewError(platform.CodeConflict, false, "幂等操作摘要不一致")
			}
			if state != "committed" {
				return PublishIdentityEventResult{}, platform.NewError(platform.CodeConflict, false, "幂等回执状态不一致")
			}
		case errors.Is(err, sql.ErrNoRows):
			// 首次登记，继续在同一事务写入outbox和receipt。
		case err != nil:
			return PublishIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "幂等回执不可读")
		}
	}
	if err := ctx.Err(); err != nil {
		return PublishIdentityEventResult{}, identityContextError(err)
	}

	payload, err := json.Marshal(input.Event)
	if err != nil {
		return PublishIdentityEventResult{}, platform.NewError(platform.CodeIntegrityError, false, "身份事件载荷不可编码")
	}
	envelope := platform.EventEnvelope{
		EventID:      input.EventID,
		EventType:    identityAuthorizationChangedEvent,
		EventVersion: 1,
		Producer:     "identity",
		SubjectID:    input.Event.AccountID,
		Revision:     input.Event.PermissionRevision,
		OccurredAt:   input.Event.OccurredAt,
		DataEpoch:    op.DataEpoch,
		Payload:      payload,
	}
	if err := outbox.EnqueueEvent(tx, envelope); err != nil {
		return PublishIdentityEventResult{}, mapIdentityOutboxError(err)
	}
	if ownedTx && existingDigest == "" {
		if err := ctx.Err(); err != nil {
			return PublishIdentityEventResult{}, identityContextError(err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO operation_receipts
			(operation_id, site_id, actor_id, input_digest, result_hash, state, created_at)
			VALUES (?, ?, ?, ?, ?, 'committed', ?)`,
			op.OperationID, op.Actor.SiteID, op.Actor.SubjectID, requestHash, resultHash, platform.Now()); err != nil {
			if stringsContains(err.Error(), "UNIQUE constraint failed") {
				return PublishIdentityEventResult{}, platform.NewError(platform.CodeConflict, false, "幂等回执已被其他请求提交")
			}
			return PublishIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "写入幂等回执失败")
		}
	}
	if ownedTx {
		if err := tx.Commit(); err != nil {
			return PublishIdentityEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交身份事件事务失败")
		}
	}
	return PublishIdentityEventResult{Queued: true}, nil
}

// RelayOutbox 只装配本域持久outbox、jobs发布端口与SnapshotManager真实scope，
// 单批领取、重试、隔离和旧代次处理由平台统一实现。
func RelayOutbox(ctx context.Context, op OperationContext, input RelayOutboxInput) (RelayOutboxResult, error) {
	if input.BatchLimit < 1 || input.BatchLimit > 100 {
		return RelayOutboxResult{}, platform.NewError(platform.CodeValidation, false, "outbox批量上限不合法")
	}
	if err := checkIdentityRepositoryContext(ctx, op); err != nil {
		return RelayOutboxResult{}, err
	}
	if op.CallerService != "identity" || op.Actor.Kind != "service" || !platform.ValidIdentifier(op.Actor.SiteID) {
		return RelayOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "outbox转发只允许身份服务后台")
	}
	runtime, ok := relayRuntimeFromContext(ctx)
	if !ok {
		return RelayOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "outbox转发运行依赖尚未装配")
	}
	relayCtx := platform.WithOperationContext(ctx, op)
	view, err := runtime.siteConfig.ReadRuntimeState(relayCtx)
	if err != nil {
		return RelayOutboxResult{}, err
	}
	if view.DataEpoch != op.DataEpoch {
		return RelayOutboxResult{}, platform.NewError(platform.CodeVersionMismatch, false, "outbox转发代次与运行权威不一致")
	}

	result, err := outbox.RunRelayBatch(relayCtx, runtime.store, runtime.publisher, platform.RelayConfig{
		BatchLimit:     input.BatchLimit,
		PollMillis:     1000,
		AdmitBatch:     runtime.snapshot.AdmitEventBatch,
		BindEventScope: runtime.snapshot.BindOutboxEventScope,
		Runtime:        runtime.siteConfig.ReadRuntimeState,
		WriteWindow:    runtime.snapshot.WriteWindow,
	})
	if err != nil {
		return RelayOutboxResult{}, err
	}
	return RelayOutboxResult{
		Relayed:     result.Relayed,
		Pending:     result.Pending,
		Quarantined: result.Quarantined,
	}, nil
}

func validateIdentityChanged(input PublishIdentityEventInput, op OperationContext) error {
	event := input.Event
	if !platform.ValidIdentifier(input.EventID) || !platform.ValidIdentifier(event.AccountID) ||
		!platform.ValidIdentifier(event.SiteID) || event.SiteID != op.Actor.SiteID ||
		event.PermissionRevision < 0 || event.CredentialRevision < 0 ||
		!platform.ValidIdentifier(op.Actor.SiteID) || !platform.ValidIdentifier(op.Actor.SubjectID) {
		return platform.NewError(platform.CodeValidation, false, "身份事件字段不合法")
	}
	if _, err := platform.ParseTimestamp(event.OccurredAt); err != nil {
		return platform.NewError(platform.CodeValidation, false, "身份事件时间不合法")
	}
	switch event.Reason {
	case "permissions", "status", "password", "server_reset", "restore", "session":
	default:
		return platform.NewError(platform.CodeValidation, false, "身份事件原因不合法")
	}
	return nil
}

func mapIdentityOutboxError(err error) error {
	var platformErr *platform.Error
	if errors.As(err, &platformErr) {
		if platformErr.Code == platform.CodeConflict || platformErr.Code == platform.CodeValidation {
			return platform.NewError(platform.CodeIntegrityError, false, "身份事件outbox不一致")
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份事件outbox不可用")
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "身份事件outbox不可用")
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func stringsContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
