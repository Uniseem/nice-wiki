package trafficguard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/traffic-guard/internal/gen"
)

// GuardRuntime 只由本域可信装配提供；预算key独立于可轮换的服务签名key。
// 预算key及前期别名必须覆盖30日ban过渡，不随业务数据快照替换。
type GuardRuntime struct {
	Store                 *PolicyStore
	Identity              platform.AuthorizationClient
	WriteWindow           platform.WriteWindowConfig
	Runtime               func(context.Context) (platform.RuntimeView, error)
	Snapshot              lifecycle.SnapshotAPI
	SigningKey            platform.SigningKey
	BudgetKey             platform.SigningKey
	PreviousBudgetKeys    []platform.SigningKey
	Anchors               platform.TrustAnchors
	ValidateIdentityEvent func(json.RawMessage) error
	Turnstile             TurnstileConfig
	Jobs                  *gen.JobsClient
}
type guardRuntimeKey struct{}
type guardTransactionKey struct{}
type guardAuthorizationKey struct{}
type guardAuthorizationMark struct {
	operationID string
	action      string
}

func WithGuardRuntime(ctx context.Context, runtime GuardRuntime) context.Context {
	return context.WithValue(WithPolicyStore(ctx, runtime.Store), guardRuntimeKey{}, runtime)
}

type guardTransaction struct {
	runtime GuardRuntime
	store   *PolicyStore
	tx      *sql.Tx
	ctx     context.Context
	op      OperationContext
	site    string
	now     time.Time
	owned   bool
	cancel  context.CancelFunc
}

// beginGuardTransaction 在任何本地锁之前建立当前写窗口；组合准入复用同一事务。
func beginGuardTransaction(ctx context.Context, op OperationContext, write bool) (*guardTransaction, error) {
	if existing, ok := ctx.Value(guardTransactionKey{}).(*guardTransaction); ok {
		nested := *existing
		nested.owned = false
		return &nested, nil
	}
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.Store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控运行依赖未装配")
	}
	if op.Actor.SiteID == "" {
		return nil, platform.NewError(platform.CodeForbidden, false, "可信站点上下文缺失")
	}
	work := ctx
	cancel := context.CancelFunc(func() {})
	if write {
		if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控写窗口未装配")
		}
		var view platform.RuntimeView
		var err error
		work, view, cancel, err = lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
		if err != nil {
			return nil, err
		}
		if view.DataEpoch != op.DataEpoch {
			cancel()
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "风控写入代次已变化")
		}
	}
	store := runtime.Store
	store.mu.RLock()
	if store.db == nil || !store.JournalRecovered {
		store.mu.RUnlock()
		cancel()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控私有库尚未恢复")
	}
	if store.openedDataEpoch != op.DataEpoch {
		store.mu.RUnlock()
		cancel()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "风控打开代次已变化")
	}
	tx, err := store.db.BeginTx(work, nil)
	if err != nil {
		store.mu.RUnlock()
		cancel()
		return nil, loginDBError()
	}
	transaction := &guardTransaction{runtime: runtime, store: store, tx: tx, ctx: work, op: op, site: op.Actor.SiteID, now: time.Now().UTC(), owned: true, cancel: cancel}
	transaction.ctx = context.WithValue(work, guardTransactionKey{}, transaction)
	return transaction, nil
}
func (t *guardTransaction) close() {
	if t.owned {
		_ = t.tx.Rollback()
		t.store.mu.RUnlock()
		t.cancel()
	}
}
func (t *guardTransaction) commit() error {
	if !t.owned {
		return nil
	}
	if err := t.tx.Commit(); err != nil {
		return loginDBError()
	}
	return nil
}

func authorizeGuardManagement(ctx context.Context, op OperationContext, action string) error {
	if checked, ok := ctx.Value(guardAuthorizationKey{}).(guardAuthorizationMark); ok && checked.operationID == op.OperationID && checked.action == action {
		return nil
	}
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.Identity == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "风控即时授权客户端未装配")
	}
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" {
		return platform.NewError(platform.CodeForbidden, false, "需要当前后台管理权限")
	}
	return apivalidate.NewIdentityAuthorizer(runtime.Identity)(ctx, op, "guard.manage", "traffic-guard:"+action)
}

func readGuardReceipt(t *guardTransaction, kind string, input any, result any) (bool, error) {
	account := ""
	if t.op.Actor.Kind == "admin" {
		account = t.op.Actor.SubjectID
	}
	canonical, err := platform.CanonicalJSON(map[string]any{"account": account, "input": input})
	if err != nil {
		return false, platform.NewError(platform.CodeValidation, false, "风控操作输入无法编码")
	}
	var digest string
	var raw []byte
	err = t.tx.QueryRowContext(t.ctx, `SELECT input_hash,result_json FROM guard_operation_receipts WHERE site_id=? AND data_epoch=? AND caller=? AND operation_kind=? AND operation_id=?`, t.site, t.op.DataEpoch, t.op.CallerService, kind, t.op.OperationID).Scan(&digest, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, loginDBError()
	}
	if digest != platform.HashBytes(canonical) {
		return false, platform.NewError(platform.CodeConflict, false, "同操作已绑定其他风控输入")
	}
	if json.Unmarshal(raw, result) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "风控原回执损坏")
	}
	return true, nil
}
func saveGuardReceipt(t *guardTransaction, kind string, input any, result any) error {
	account := ""
	if t.op.Actor.Kind == "admin" {
		account = t.op.Actor.SubjectID
	}
	canonical, err := platform.CanonicalJSON(map[string]any{"account": account, "input": input})
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "风控操作输入无法编码")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "风控回执无法编码")
	}
	_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_operation_receipts(site_id,data_epoch,caller,operation_kind,operation_id,input_hash,result_json,created_at_ns) VALUES(?,?,?,?,?,?,?,?)`, t.site, t.op.DataEpoch, t.op.CallerService, kind, t.op.OperationID, platform.HashBytes(canonical), raw, t.now.UnixNano())
	if err != nil {
		return loginDBError()
	}
	return nil
}
func enqueueGuardEvent(t *guardTransaction, eventType, subject string, revision int64, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "风控事件无法编码")
	}
	return outbox.EnqueueEvent(t.tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: eventType, EventVersion: 1, Producer: "traffic-guard", SubjectID: subject, Revision: revision, OccurredAt: t.now.Format(time.RFC3339Nano), DataEpoch: t.op.DataEpoch, Payload: raw})
}
func readGuardPolicy(t *guardTransaction) (GuardPolicy, int64, error) {
	var raw []byte
	var revision int64
	var epoch string
	err := t.tx.QueryRowContext(t.ctx, `SELECT policy_json,revision,data_epoch FROM guard_policy WHERE site_id=?`, t.site).Scan(&raw, &revision, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return GuardPolicy{}, 0, platform.NewError(platform.CodeNotFound, false, "当前站点尚未保存风控策略")
	}
	if err != nil {
		return GuardPolicy{}, 0, loginDBError()
	}
	if epoch != t.op.DataEpoch {
		return GuardPolicy{}, 0, platform.NewError(platform.CodeVersionMismatch, false, "风控策略代次不一致")
	}
	var policy GuardPolicy
	if json.Unmarshal(raw, &policy) != nil {
		return GuardPolicy{}, 0, platform.NewError(platform.CodeIntegrityError, false, "风控策略损坏")
	}
	if policy.QaPerNetworkPerMinute == nil {
		rate := 600
		policy.QaPerNetworkPerMinute = &rate
	}
	return policy, revision, nil
}
