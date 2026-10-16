package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"
	"unicode"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/content/internal/gen"
)

// 业务Input保留路径字段；生成HTTP DTO在严格边界校验后映射到本域输入。
type AcquireEditLockInput struct {
	DocumentId Identifier `json:"documentId"`
	TabId      Identifier `json:"tabId"`
}
type AcquireEditLockResult = gen.AcquireEditLockResult
type RenewEditLockResult = gen.RenewEditLockResult
type ReleaseEditLockResult = gen.ReleaseEditLockResult
type ForceReleaseEditLockResult = gen.ForceReleaseEditLockResult
type ExpireIdleLocksInput struct {
	Now time.Time `json:"now"`
}
type ExpireIdleLocksResult struct {
	ExpiredDocumentIds []Identifier `json:"expiredDocumentIds"`
}

// EditLockRuntime只由已认证服务装配；通用身份/请求/epoch边界仍由平台入口建立。
type EditLockRuntime struct {
	Manager     *EditLockManager
	Identity    *gen.IdentityClient
	WriteWindow platform.WriteWindowConfig
}
type editLockRuntimeKey struct{}

func WithEditLockRuntime(ctx context.Context, runtime EditLockRuntime) context.Context {
	return context.WithValue(ctx, editLockRuntimeKey{}, runtime)
}
func editLockRuntimeFromContext(ctx context.Context) (EditLockRuntime, error) {
	runtime, ok := ctx.Value(editLockRuntimeKey{}).(EditLockRuntime)
	if !ok || runtime.Manager == nil || runtime.Identity == nil || runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return runtime, platform.NewError(platform.CodeDependencyUnavailable, true, "编辑锁用例依赖未装配")
	}
	return runtime, nil
}

func AcquireEditLock(ctx context.Context, op OperationContext, input AcquireEditLockInput) (AcquireEditLockResult, error) {
	runtime, err := editLockRuntimeFromContext(ctx)
	if err != nil {
		return AcquireEditLockResult{}, err
	}
	name, err := authorizeEditLock(ctx, op, runtime.Identity, "content.edit", "acquireEditLock", input.DocumentId)
	if err != nil {
		return AcquireEditLockResult{}, err
	}
	snapshot, err := runtime.Manager.AcquireLease(ctx, op, AcquireLeaseInput{DocumentID: input.DocumentId, TabID: input.TabId, AccountName: name})
	if err != nil {
		return AcquireEditLockResult{}, err
	}
	return AcquireEditLockResult{Lease: *snapshot.Lease, Writable: snapshot.Writable}, nil
}
func RenewEditLock(ctx context.Context, op OperationContext, input RenewEditLockInput) (RenewEditLockResult, error) {
	runtime, err := editLockRuntimeFromContext(ctx)
	if err != nil {
		return RenewEditLockResult{}, err
	}
	if _, err := authorizeEditLock(ctx, op, runtime.Identity, "content.edit", "renewEditLock", input.DocumentId); err != nil {
		return RenewEditLockResult{}, err
	}
	snapshot, err := runtime.Manager.RenewLease(ctx, op, input)
	if err != nil {
		return RenewEditLockResult{}, err
	}
	return RenewEditLockResult{Lease: *snapshot.Lease, IdleWarning: snapshot.Lease.IdleWarningAt != nil}, nil
}
func ReleaseEditLock(ctx context.Context, op OperationContext, input ReleaseEditLockInput) (ReleaseEditLockResult, error) {
	runtime, err := editLockRuntimeFromContext(ctx)
	if err != nil {
		return ReleaseEditLockResult{}, err
	}
	if _, err := authorizeEditLock(ctx, op, runtime.Identity, "content.edit", "releaseEditLock", input.DocumentId); err != nil {
		return ReleaseEditLockResult{}, err
	}
	_, err = runtime.Manager.ReleaseLease(ctx, op, input)
	return ReleaseEditLockResult{Released: err == nil}, err
}
func ForceReleaseEditLock(ctx context.Context, op OperationContext, input ForceReleaseEditLockInput) (ForceReleaseEditLockResult, error) {
	runtime, err := editLockRuntimeFromContext(ctx)
	if err != nil {
		return ForceReleaseEditLockResult{}, err
	}
	if _, err := authorizeEditLock(ctx, op, runtime.Identity, "superadmin", "forceReleaseEditLock", input.DocumentId); err != nil {
		return ForceReleaseEditLockResult{}, err
	}
	_, err = runtime.Manager.ForceReleaseLease(ctx, op, input)
	return ForceReleaseEditLockResult{Released: err == nil}, err
}
func ExpireIdleLocks(ctx context.Context, op OperationContext, input ExpireIdleLocksInput) (ExpireIdleLocksResult, error) {
	runtime, err := editLockRuntimeFromContext(ctx)
	if err != nil {
		return ExpireIdleLocksResult{}, err
	}
	if op.CallerService != "content" || op.Actor.Kind != "service" || op.Actor.SubjectID != "content" {
		return ExpireIdleLocksResult{}, platform.NewError(platform.CodeForbidden, false, "编辑锁过期仅允许本域后台")
	}
	// now由本域后台调度器的服务时钟提供，同一operationId重试保持原时刻。
	snapshots, err := runtime.Manager.ExpireLeases(ctx, op, input.Now)
	if err != nil {
		return ExpireIdleLocksResult{}, err
	}
	result := ExpireIdleLocksResult{ExpiredDocumentIds: make([]Identifier, 0)}
	for _, snapshot := range snapshots {
		if snapshot.Cause == "expired" {
			result.ExpiredDocumentIds = append(result.ExpiredDocumentIds, snapshot.DocumentID)
		}
	}
	return result, nil
}

// authorizeEditLock逐次核最新会话与权限。账户名仅取identity同一权威响应，
// 作为纯文本显示；拒绝控制字符，禁止浏览器自报或把名称当作HTML。
func authorizeEditLock(ctx context.Context, op OperationContext, client *gen.IdentityClient, permission, action string, documentID Identifier) (string, error) {
	if op.Actor.Kind != "admin" {
		return "", platform.NewError(platform.CodeForbidden, false, "编辑锁仅允许管理账户")
	}
	body := struct {
		SubjectID                  Identifier `json:"subjectId"`
		SessionID                  Identifier `json:"sessionId"`
		SiteID                     Identifier `json:"siteId"`
		Permission                 string     `json:"permission"`
		ResourceID                 Identifier `json:"resourceId"`
		ExpectedPermissionRevision int64      `json:"expectedPermissionRevision"`
		Action                     string     `json:"action"`
	}{op.Actor.SubjectID, op.Actor.SessionID, op.Actor.SiteID, permission, documentID, op.Actor.PermissionRevision, "content:" + action}
	raw, err := client.CheckAuthorization(platform.WithOperationContext(ctx, op), body)
	if err != nil {
		return "", err
	}
	var decision struct {
		Authorized  *bool                `json:"authorized"`
		AccountName string               `json:"accountName"`
		Auth        platform.AuthContext `json:"auth"`
	}
	if json.Unmarshal(raw, &decision) != nil || decision.Authorized == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "编辑授权响应不可用")
	}
	if !*decision.Authorized {
		return "", platform.NewError(platform.CodeForbidden, false, "没有编辑操作权限")
	}
	if decision.Auth.SubjectID != op.Actor.SubjectID || decision.Auth.SiteID != op.Actor.SiteID || decision.Auth.SessionID != op.Actor.SessionID || decision.Auth.SessionRevision != op.Actor.SessionRevision || decision.Auth.PermissionRevision != op.Actor.PermissionRevision {
		return "", platform.NewError(platform.CodeUnauthenticated, false, "编辑授权主体或版本已变化")
	}
	if !utf8.ValidString(decision.AccountName) || utf8.RuneCountInString(decision.AccountName) < 1 || utf8.RuneCountInString(decision.AccountName) > 64 {
		return "", platform.NewError(platform.CodeIntegrityError, false, "编辑账户显示名不合法")
	}
	for _, r := range decision.AccountName {
		if unicode.IsControl(r) {
			return "", platform.NewError(platform.CodeIntegrityError, false, "编辑账户显示名含控制字符")
		}
	}
	return decision.AccountName, nil
}

type editLockCommand struct {
	action      string
	documentID  Identifier
	accountName string
	now         time.Time
	input       any
}
type editLockReceiptKey struct {
	operation  OperationContext
	scope      string
	digest     Hash
	documentID Identifier
}
type pendingEditLock struct {
	key        editLockReceiptKey
	candidate  *editLeaseCell
	resultHash Hash
}

func (manager *EditLockManager) executeEditLock(ctx context.Context, op OperationContext, command editLockCommand) (EditLockSnapshot, error) {
	runtime, err := editLockRuntimeFromContext(ctx)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	if runtime.Manager != manager {
		return EditLockSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "编辑锁装配不一致")
	}
	raw, err := platform.CanonicalJSON(struct {
		Action string `json:"action"`
		Input  any    `json:"input"`
	}{command.action, command.input})
	if err != nil {
		return EditLockSnapshot{}, err
	}
	scope := "user"
	if command.action == "expire" {
		scope = "expire:" + command.documentID
	}
	key := editLockReceiptKey{operation: op, scope: scope, digest: platform.HashBytes(raw), documentID: command.documentID}
	if op.DataEpoch != manager.store.epoch {
		return EditLockSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "编辑操作不属于已打开代次")
	}
	prior, found, err := manager.store.readEditLockReceipt(ctx, manager.store.db, key)
	if err != nil || found {
		return prior, err
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	defer cancel()
	if view.DataEpoch != manager.store.epoch || view.DataEpoch != op.DataEpoch {
		return EditLockSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "编辑操作代次已变化")
	}
	manager.life.RLock()
	defer manager.life.RUnlock()
	if manager.closed {
		return EditLockSnapshot{}, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	cell, err := manager.lockEditCell(work, op, command.documentID)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	cell.mu.Lock()
	defer cell.mu.Unlock()
	if err := manager.resolvePendingEditLock(work, cell); err != nil {
		return EditLockSnapshot{}, err
	}
	// 原回执在任何新租约/文档生命周期分支之前读取，不复活历史内存租约。
	prior, found, err = manager.store.readEditLockReceipt(work, manager.store.db, key)
	if err != nil || found {
		return prior, err
	}
	state, err := manager.store.ReadDocumentEditState(work, op, command.documentID)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	candidate := &editLeaseCell{fence: cell.fence, exhausted: cell.exhausted, lease: copyEditLease(cell.lease)}
	previous := copyEditLease(cell.lease)
	now := time.Now().UTC()
	if command.action == "expire" {
		now = command.now
	}
	snapshot, err := manager.planEditLock(op, command, state, candidate, now)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	resultHash, err := manager.store.commitEditLock(work, key, state, snapshot, previous)
	if err != nil {
		// SQL提交的返回可能丢失；保留候选并关闭该doc保存，后续只按真实回执解决。
		// 无回执时丢弃候选，有回执时才安装；重启永不重建租约。
		if resultHash != "" {
			cell.pending = &pendingEditLock{key: key, candidate: candidate, resultHash: resultHash}
		}
		return EditLockSnapshot{}, err
	}
	cell.fence, cell.exhausted, cell.lease = candidate.fence, candidate.exhausted, candidate.lease
	return snapshot, nil
}

func (manager *EditLockManager) planEditLock(op OperationContext, command editLockCommand, state DocumentEditState, candidate *editLeaseCell, now time.Time) (EditLockSnapshot, error) {
	switch command.action {
	case "acquire":
		if err := requireEditableDocument(state); err != nil {
			return EditLockSnapshot{}, err
		}
		changed, cause, err := refreshEditLease(candidate, now)
		if err != nil {
			return EditLockSnapshot{}, err
		}
		if candidate.lease != nil {
			return manager.editLockSnapshot(op, command.documentID, candidate, changed, cause), nil
		}
		if err := advanceEditFence(candidate); err != nil {
			return EditLockSnapshot{}, err
		}
		candidate.lease = &gen.EditLease{DocumentId: command.documentID, HolderAccountId: op.Actor.SubjectID, HolderAccountName: command.accountName, LeaseId: platform.NewIdentifier(), FencingToken: int(candidate.fence), ServerBootId: manager.bootID, ExpiresAt: now.Add(2 * time.Minute), LastInputAt: now, HeartbeatSequence: 0, InputSequence: 0}
		return manager.editLockSnapshot(op, command.documentID, candidate, true, "acquired"), nil
	case "renew":
		if err := requireEditableDocument(state); err != nil {
			return EditLockSnapshot{}, err
		}
		input := command.input.(RenewEditLockInput)
		if err := manager.matchEditLease(candidate, op, DraftFenceInput{DocumentID: input.DocumentId, LeaseID: input.LeaseId, FencingToken: int64(input.FencingToken), ServerBootID: input.ServerBootId}, now); err != nil {
			return EditLockSnapshot{}, err
		}
		if input.HeartbeatSequence <= candidate.lease.HeartbeatSequence {
			return manager.editLockSnapshot(op, command.documentID, candidate, false, "unchanged"), nil
		}
		candidate.lease.HeartbeatSequence = input.HeartbeatSequence
		warningCleared := candidate.lease.IdleWarningAt != nil && input.InputSequence > candidate.lease.InputSequence
		if input.InputSequence > candidate.lease.InputSequence {
			candidate.lease.InputSequence, candidate.lease.LastInputAt, candidate.lease.IdleWarningAt = input.InputSequence, now, nil
		}
		candidate.lease.ExpiresAt = now.Add(2 * time.Minute)
		changed, cause, err := refreshEditLease(candidate, now)
		if err != nil {
			return EditLockSnapshot{}, err
		}
		if candidate.lease.IdleWarningAt != nil {
			deadline := candidate.lease.IdleWarningAt.Add(2 * time.Minute)
			if deadline.Before(candidate.lease.ExpiresAt) {
				candidate.lease.ExpiresAt = deadline
			}
		}
		if warningCleared {
			changed, cause = true, "active"
		}
		return manager.editLockSnapshot(op, command.documentID, candidate, changed, cause), nil
	case "release", "force":
		cause := "released"
		if candidate.lease == nil {
			return manager.editLockSnapshot(op, command.documentID, candidate, false, cause), nil
		}
		if command.action == "release" {
			input := command.input.(ReleaseEditLockInput)
			if candidate.lease.HolderAccountId != op.Actor.SubjectID {
				return EditLockSnapshot{}, platform.NewError(platform.CodeForbidden, false, "不能释放其他账户编辑锁")
			}
			if candidate.lease.LeaseId != input.LeaseId || candidate.lease.ServerBootId != input.ServerBootId {
				return EditLockSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "编辑锁已变化")
			}
		} else {
			input := command.input.(ForceReleaseEditLockInput)
			if candidate.lease.LeaseId != input.ExpectedLeaseId {
				return EditLockSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "待强释编辑锁已变化")
			}
			cause = "forced"
		}
		if err := advanceEditFence(candidate); err != nil {
			return EditLockSnapshot{}, err
		}
		candidate.lease = nil
		return manager.editLockSnapshot(op, command.documentID, candidate, true, cause), nil
	case "expire":
		changed, cause, err := refreshEditLease(candidate, now)
		if err != nil {
			return EditLockSnapshot{}, err
		}
		return manager.editLockSnapshot(op, command.documentID, candidate, changed, cause), nil
	}
	return EditLockSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "编辑动作未登记")
}

func (manager *EditLockManager) resolvePendingEditLock(ctx context.Context, cell *editLeaseCell) error {
	if cell.pending == nil {
		return nil
	}
	pending := cell.pending
	result, found, err := manager.store.readEditLockReceipt(ctx, manager.store.db, pending.key)
	if err != nil {
		return err
	}
	if found {
		raw, err := json.Marshal(result)
		if err != nil || platform.HashBytes(raw) != pending.resultHash {
			return platform.NewError(platform.CodeIntegrityError, false, "编辑提交回执与待确认候选不一致")
		}
		cell.fence, cell.exhausted, cell.lease = pending.candidate.fence, pending.candidate.exhausted, pending.candidate.lease
	}
	cell.pending = nil
	return nil
}

type editLockQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) readEditLockReceipt(ctx context.Context, query editLockQuery, key editLockReceiptKey) (EditLockSnapshot, bool, error) {
	op := key.operation
	var digest, documentID, resultHash string
	var raw []byte
	err := query.QueryRowContext(ctx, `SELECT request_hash,document_id,result_json,result_hash FROM edit_lock_receipts WHERE caller_service=? AND actor_id=? AND site_id=? AND data_epoch=? AND operation_id=? AND scope=?`, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID, key.scope).Scan(&digest, &documentID, &raw, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return EditLockSnapshot{}, false, nil
	}
	if err != nil {
		return EditLockSnapshot{}, false, contentStorageError()
	}
	if digest != key.digest || documentID != key.documentID {
		return EditLockSnapshot{}, false, platform.NewError(platform.CodeConflict, false, "编辑操作标识已绑定其他请求")
	}
	var result EditLockSnapshot
	if platform.HashBytes(raw) != resultHash || decodeStorageJSON(raw, &result) != nil || result.SiteID != op.Actor.SiteID || result.DocumentID != documentID {
		return EditLockSnapshot{}, false, platform.NewError(platform.CodeIntegrityError, false, "编辑原回执不完整")
	}
	return result, true, nil
}

// commitEditLock返回非空hash仅表示已尝试Commit；该窗口错误必须保留pending。
func (s *Store) commitEditLock(ctx context.Context, key editLockReceiptKey, state DocumentEditState, snapshot EditLockSnapshot, previous *gen.EditLease) (Hash, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", contentStorageError()
	}
	defer tx.Rollback()
	// 同doc已被mutex串行且只有单leader；此处只补两个不同doc复用全局scope/op的冲突检查。
	// 同doc原回执已在锁内读取，found=true不会到达本事务，不添加候选回放分支。
	if _, _, err := s.readEditLockReceipt(ctx, tx, key); err != nil {
		return "", err
	}
	// 与生命周期提交共用doc mutex，并在本次短事务再核权威行和隔离。
	var revision, deleteFence int64
	var lifecycleState string
	var quarantined bool
	if err := tx.QueryRowContext(ctx, `SELECT d.state,d.revision,d.delete_fence,EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=d.site_id AND q.document_id=d.document_id) FROM documents d WHERE d.site_id=? AND d.document_id=?`, state.SiteID, state.DocumentID).Scan(&lifecycleState, &revision, &deleteFence, &quarantined); err != nil {
		return "", contentStorageError()
	}
	if revision != state.Revision || deleteFence != state.DeleteFence || lifecycleState != state.State || quarantined != state.Quarantined {
		return "", platform.NewError(platform.CodeVersionMismatch, false, "文档编辑权威已变化")
	}
	op := key.operation
	if snapshot.Changed {
		if err := s.enqueueEditLockChange(ctx, tx, op, snapshot, previous); err != nil {
			return "", err
		}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "编辑结果无法编码")
	}
	hash := platform.HashBytes(raw)
	if _, err := tx.ExecContext(ctx, `INSERT INTO edit_lock_receipts(caller_service,actor_id,site_id,data_epoch,operation_id,scope,document_id,request_hash,result_json,result_hash) VALUES(?,?,?,?,?,?,?,?,?,?)`, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID, key.scope, key.documentID, key.digest, string(raw), hash); err != nil {
		return "", contentStorageError()
	}
	if err := tx.Commit(); err != nil {
		return hash, contentStorageError()
	}
	return hash, nil
}

// EditLockChangedPayload不含正文、姓名和可用于续租/保存的leaseId。
type EditLockChangedPayload struct {
	SiteID            Identifier  `json:"siteId"`
	DocumentID        Identifier  `json:"documentId"`
	ServerBootID      Identifier  `json:"serverBootId"`
	FencingToken      int64       `json:"fencingToken"`
	Cause             string      `json:"cause"`
	HolderAccountID   *Identifier `json:"holderAccountId"`
	AffectedAccountID *Identifier `json:"affectedAccountId"`
	ExpiresAt         *time.Time  `json:"expiresAt"`
	IdleWarningAt     *time.Time  `json:"idleWarningAt"`
}

func (s *Store) enqueueEditLockChange(ctx context.Context, tx *sql.Tx, op OperationContext, snapshot EditLockSnapshot, previous *gen.EditLease) error {
	var revision int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO edit_lock_event_revisions(site_id,document_id,revision) VALUES(?,?,1) ON CONFLICT(site_id,document_id) DO UPDATE SET revision=revision+1 WHERE revision<9007199254740991 RETURNING revision`, snapshot.SiteID, snapshot.DocumentID).Scan(&revision); err != nil {
		return contentStorageError()
	}
	payload := EditLockChangedPayload{SiteID: snapshot.SiteID, DocumentID: snapshot.DocumentID, ServerBootID: snapshot.ServerBootID, FencingToken: snapshot.FencingToken, Cause: snapshot.Cause}
	if snapshot.Lease != nil {
		payload.HolderAccountID = &snapshot.Lease.HolderAccountId
		payload.ExpiresAt = &snapshot.Lease.ExpiresAt
		payload.IdleWarningAt = snapshot.Lease.IdleWarningAt
	}
	if previous != nil {
		payload.AffectedAccountID = &previous.HolderAccountId
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "编辑事件无法编码")
	}
	eventID, now := platform.NewIdentifier(), platform.Now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO edit_lock_audit(event_id,site_id,document_id,actor_id,actor_kind,operation_id,data_epoch,server_boot_id,fencing_token,action,affected_account_id,occurred_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, eventID, snapshot.SiteID, snapshot.DocumentID, op.Actor.SubjectID, op.Actor.Kind, op.OperationID, op.DataEpoch, snapshot.ServerBootID, snapshot.FencingToken, snapshot.Cause, payload.AffectedAccountID, now); err != nil {
		return contentStorageError()
	}
	return outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: eventID, EventType: "content.edit-lock.changed", EventVersion: 1, Producer: "content", SubjectID: snapshot.DocumentID, Revision: revision, OccurredAt: now, DataEpoch: op.DataEpoch, Payload: raw})
}

func (manager *EditLockManager) expireEditLocks(ctx context.Context, op OperationContext, now time.Time) ([]EditLockSnapshot, error) {
	if err := manager.store.checkRevisionScope(ctx, op); err != nil {
		return nil, err
	}
	// 将同一后台操作已完成的文档纳入遍历，局部失败后重试仍回原结果。
	rows, err := manager.store.db.QueryContext(ctx, `SELECT document_id FROM edit_lock_receipts WHERE caller_service=? AND actor_id=? AND site_id=? AND data_epoch=? AND operation_id=? AND scope LIKE 'expire:%'`, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID)
	if err != nil {
		return nil, contentStorageError()
	}
	idsByValue := map[Identifier]bool{}
	for rows.Next() {
		var id Identifier
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, contentStorageError()
		}
		idsByValue[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, contentStorageError()
	}
	manager.life.RLock()
	if manager.closed {
		manager.life.RUnlock()
		return nil, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	manager.cellsMu.Lock()
	current := make(map[Identifier]*editLeaseCell)
	for key, cell := range manager.cells {
		if key.site == op.Actor.SiteID {
			current[key.document] = cell
		}
	}
	manager.cellsMu.Unlock()
	for id, cell := range current {
		cell.mu.Lock()
		active := cell.lease != nil || cell.pending != nil
		cell.mu.Unlock()
		if active {
			idsByValue[id] = true
		}
	}
	manager.life.RUnlock()
	ids := make([]Identifier, 0, len(idsByValue))
	for id := range idsByValue {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]EditLockSnapshot, 0)
	for _, id := range ids {
		if len(result) >= 10000 {
			break
		}
		input := struct {
			DocumentID Identifier `json:"documentId"`
			Now        time.Time  `json:"now"`
		}{id, now}
		snapshot, err := manager.executeEditLock(ctx, op, editLockCommand{action: "expire", documentID: id, now: now, input: input})
		if err != nil {
			return nil, err
		}
		if snapshot.Changed {
			result = append(result, snapshot)
		}
	}
	return result, nil
}
