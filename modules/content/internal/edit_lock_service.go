package content

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/content/internal/gen"
)

const maxEditFence int64 = 9007199254740991

type editDocumentKey struct{ site, document Identifier }
type editLeaseCell struct {
	mu        sync.Mutex
	fence     int64
	exhausted bool
	lease     *gen.EditLease
}

// EditLockManager只实现真实租约和短事务guard。完整handler仍须即时授权、
// identity账户显示名、可靠通知/审计与原操作回执，不能直接把核心当HTTP成功。
type EditLockManager struct {
	store   *Store
	bootID  Identifier
	leader  *os.File
	life    sync.RWMutex
	closed  bool
	cellsMu sync.Mutex
	cells   map[editDocumentKey]*editLeaseCell
}

// AcquireLeaseInput的AccountName必须来自identity权威，而非浏览器自报。
type AcquireLeaseInput struct {
	DocumentID  Identifier
	AccountName string
}

// 下列业务Input按types.json唯一业务定义投影；生成的HTTP request DTO
// 拆开路径字段后会裁剪这些整包Input，未来handler仍需经严格边界显式映射。
type RenewEditLockInput struct {
	DocumentId        Identifier `json:"documentId"`
	LeaseId           Identifier `json:"leaseId"`
	FencingToken      int        `json:"fencingToken"`
	ServerBootId      Identifier `json:"serverBootId"`
	LastInputAt       time.Time  `json:"lastInputAt"`
	HeartbeatSequence int        `json:"heartbeatSequence"`
	InputSequence     int        `json:"inputSequence"`
}

type ReleaseEditLockInput struct {
	DocumentId   Identifier `json:"documentId"`
	LeaseId      Identifier `json:"leaseId"`
	ServerBootId Identifier `json:"serverBootId"`
}

type ForceReleaseEditLockInput struct {
	DocumentId      Identifier `json:"documentId"`
	ExpectedLeaseId Identifier `json:"expectedLeaseId"`
}

// EditLockSnapshot是供完整用例编排的内存事实，不代表通知或审计已经提交。
type EditLockSnapshot struct {
	SiteID       Identifier
	DocumentID   Identifier
	ServerBootID Identifier
	FencingToken int64
	Lease        *gen.EditLease
	Writable     bool
	Changed      bool
	Cause        string
}

// DraftFenceInput必须由实际保存输入和先前持久文档快照取得。
type DraftFenceInput struct {
	DocumentID          Identifier
	LeaseID             Identifier
	FencingToken        int64
	ServerBootID        Identifier
	ExpectedDeleteFence int64
}

// SaveFenceGuard同时持有manager生命读锁和doc mutex；只能跨本域短SQL事务，
// 不能跨HTTP/模型/正文文件IO。Release后不得再将这个guard用于提交。
type SaveFenceGuard struct {
	manager *EditLockManager
	cell    *editLeaseCell
	state   DocumentEditState
	lease   *gen.EditLease
	once    sync.Once
}

// NewEditLockManager持有control区单leader flock至Close，从不unlink锁文件。
func NewEditLockManager(store *Store) (*EditLockManager, error) {
	path := filepath.Join(filepath.Dir(store.root), "control", "content-leader.lock")
	if err := lifecycle.ValidatePrivatePath(filepath.Dir(path), "directory", false); err != nil {
		return nil, err
	}
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", true); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return nil, contentStorageError()
	}
	flags := syscall.O_RDWR | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	if missing {
		flags |= syscall.O_CREAT | syscall.O_EXCL
	}
	fd, err := syscall.Open(path, flags, 0o600)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "编辑leader文件不可打开")
	}
	file := os.NewFile(uintptr(fd), path)
	success := false
	defer func() {
		if !success {
			_ = file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil || (before != nil && !os.SameFile(before, opened)) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "编辑leader文件身份变化")
	}
	if err := validateEditLeader(file); err != nil {
		return nil, err
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, platform.NewError(platform.CodeLocked, true, "已有content编辑leader")
		}
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "编辑leader文件不可锁定")
	}
	if err := validateEditLeader(file); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, contentStorageError()
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return nil, contentStorageError()
	}
	manager := &EditLockManager{store: store, bootID: platform.NewIdentifier(), leader: file, cells: map[editDocumentKey]*editLeaseCell{}}
	success = true
	return manager, nil
}

func validateEditLeader(file *os.File) error {
	if err := lifecycle.ValidatePrivatePath(file.Name(), "regular-file", false); err != nil {
		return err
	}
	current, err := os.Lstat(file.Name())
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "编辑leader路径已变化")
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(current, opened) {
		return platform.NewError(platform.CodeIntegrityError, false, "编辑leader文件身份已变化")
	}
	return nil
}

func (manager *EditLockManager) lockEditCell(ctx context.Context, op OperationContext, documentID Identifier) (*editLeaseCell, error) {
	// 存在性前置只约束map分配；调用方拿doc mutex后仍重读真实生命周期权威。
	if _, err := manager.store.ReadDocumentEditState(ctx, op, documentID); err != nil {
		return nil, err
	}
	key := editDocumentKey{site: op.Actor.SiteID, document: documentID}
	manager.cellsMu.Lock()
	defer manager.cellsMu.Unlock()
	cell := manager.cells[key]
	if cell == nil {
		cell = &editLeaseCell{}
		manager.cells[key] = cell
	}
	return cell, nil
}

// AcquireLease在真实文档状态检查后分配或复用账户共享锁；新tab不重置活动。
func (manager *EditLockManager) AcquireLease(ctx context.Context, op OperationContext, input AcquireLeaseInput) (EditLockSnapshot, error) {
	manager.life.RLock()
	defer manager.life.RUnlock()
	if manager.closed {
		return EditLockSnapshot{}, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	cell, err := manager.lockEditCell(ctx, op, input.DocumentID)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	cell.mu.Lock()
	defer cell.mu.Unlock()
	state, err := manager.store.ReadDocumentEditState(ctx, op, input.DocumentID)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	if err := requireEditableDocument(state); err != nil {
		return EditLockSnapshot{}, err
	}
	now := time.Now().UTC()
	changed, cause, err := refreshEditLease(cell, now)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	if cell.lease != nil {
		return manager.editLockSnapshot(op, input.DocumentID, cell, changed, cause), nil
	}
	if err := advanceEditFence(cell); err != nil {
		return EditLockSnapshot{}, err
	}
	cell.lease = &gen.EditLease{DocumentId: input.DocumentID, HolderAccountId: op.Actor.SubjectID, HolderAccountName: input.AccountName, LeaseId: platform.NewIdentifier(), FencingToken: int(cell.fence), ServerBootId: manager.bootID, ExpiresAt: now.Add(2 * time.Minute), LastInputAt: now, IdleWarningAt: nil, HeartbeatSequence: 0, InputSequence: 0}
	return manager.editLockSnapshot(op, input.DocumentID, cell, true, "acquired"), nil
}

// RenewLease分别接受单调心跳和输入；旧心跳不续期，lastInputAt不参与权威。
func (manager *EditLockManager) RenewLease(ctx context.Context, op OperationContext, input RenewEditLockInput) (EditLockSnapshot, error) {
	manager.life.RLock()
	defer manager.life.RUnlock()
	if manager.closed {
		return EditLockSnapshot{}, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	cell, err := manager.lockEditCell(ctx, op, input.DocumentId)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	cell.mu.Lock()
	defer cell.mu.Unlock()
	state, err := manager.store.ReadDocumentEditState(ctx, op, input.DocumentId)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	if err := requireEditableDocument(state); err != nil {
		return EditLockSnapshot{}, err
	}
	now := time.Now().UTC()
	if err := manager.matchEditLease(cell, op, DraftFenceInput{DocumentID: input.DocumentId, LeaseID: input.LeaseId, FencingToken: int64(input.FencingToken), ServerBootID: input.ServerBootId}, now); err != nil {
		return EditLockSnapshot{}, err
	}
	if input.HeartbeatSequence <= cell.lease.HeartbeatSequence {
		return manager.editLockSnapshot(op, input.DocumentId, cell, false, "unchanged"), nil
	}
	cell.lease.HeartbeatSequence = input.HeartbeatSequence
	if input.InputSequence > cell.lease.InputSequence {
		cell.lease.InputSequence = input.InputSequence
		cell.lease.LastInputAt = now
		cell.lease.IdleWarningAt = nil
	}
	cell.lease.ExpiresAt = now.Add(2 * time.Minute)
	changed, cause, err := refreshEditLease(cell, now)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	if cell.lease != nil && cell.lease.IdleWarningAt != nil {
		deadline := cell.lease.IdleWarningAt.Add(2 * time.Minute)
		if deadline.Before(cell.lease.ExpiresAt) {
			cell.lease.ExpiresAt = deadline
		}
	}
	return manager.editLockSnapshot(op, input.DocumentId, cell, changed, cause), nil
}

func (manager *EditLockManager) ReleaseLease(ctx context.Context, op OperationContext, input ReleaseEditLockInput) (EditLockSnapshot, error) {
	manager.life.RLock()
	defer manager.life.RUnlock()
	if manager.closed {
		return EditLockSnapshot{}, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	cell, err := manager.lockEditCell(ctx, op, input.DocumentId)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	cell.mu.Lock()
	defer cell.mu.Unlock()
	if _, err := manager.store.ReadDocumentEditState(ctx, op, input.DocumentId); err != nil {
		return EditLockSnapshot{}, err
	}
	if cell.lease == nil {
		return manager.editLockSnapshot(op, input.DocumentId, cell, false, "released"), nil
	}
	if cell.lease.HolderAccountId != op.Actor.SubjectID {
		return EditLockSnapshot{}, platform.NewError(platform.CodeForbidden, false, "不能释放其他账户编辑锁")
	}
	if cell.lease.LeaseId != input.LeaseId || cell.lease.ServerBootId != input.ServerBootId {
		return EditLockSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "编辑锁已变化")
	}
	if err := advanceEditFence(cell); err != nil {
		return EditLockSnapshot{}, err
	}
	cell.lease = nil
	return manager.editLockSnapshot(op, input.DocumentId, cell, true, "released"), nil
}

// ForceReleaseLease只实现已授权超管编排中的原子内存步骤；不是完整HTTP回执。
func (manager *EditLockManager) ForceReleaseLease(ctx context.Context, op OperationContext, input ForceReleaseEditLockInput) (EditLockSnapshot, error) {
	manager.life.RLock()
	defer manager.life.RUnlock()
	if manager.closed {
		return EditLockSnapshot{}, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	cell, err := manager.lockEditCell(ctx, op, input.DocumentId)
	if err != nil {
		return EditLockSnapshot{}, err
	}
	cell.mu.Lock()
	defer cell.mu.Unlock()
	if _, err := manager.store.ReadDocumentEditState(ctx, op, input.DocumentId); err != nil {
		return EditLockSnapshot{}, err
	}
	if cell.lease == nil {
		return manager.editLockSnapshot(op, input.DocumentId, cell, false, "released"), nil
	}
	if cell.lease.LeaseId != input.ExpectedLeaseId {
		return EditLockSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "待强释编辑锁已变化")
	}
	if err := advanceEditFence(cell); err != nil {
		return EditLockSnapshot{}, err
	}
	cell.lease = nil
	return manager.editLockSnapshot(op, input.DocumentId, cell, true, "forced"), nil
}

// ExpireLeases只返回真实已发生的内存变化；完整后台编排还须可靠发通知。
func (manager *EditLockManager) ExpireLeases(ctx context.Context, op OperationContext) ([]EditLockSnapshot, error) {
	manager.life.RLock()
	defer manager.life.RUnlock()
	if manager.closed {
		return nil, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	if op.CallerService != "content" || op.Actor.Kind != "service" {
		return nil, platform.NewError(platform.CodeForbidden, false, "编辑锁过期仅由本域后台执行")
	}
	manager.cellsMu.Lock()
	keys := make([]editDocumentKey, 0, len(manager.cells))
	for key := range manager.cells {
		keys = append(keys, key)
	}
	manager.cellsMu.Unlock()
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].site != keys[j].site {
			return keys[i].site < keys[j].site
		}
		return keys[i].document < keys[j].document
	})
	result := make([]EditLockSnapshot, 0)
	now := time.Now().UTC()
	for _, key := range keys {
		if err := checkContextOnly(ctx); err != nil {
			return nil, err
		}
		manager.cellsMu.Lock()
		cell := manager.cells[key]
		manager.cellsMu.Unlock()
		cell.mu.Lock()
		changed, cause, err := refreshEditLease(cell, now)
		siteOp := op
		siteOp.Actor.SiteID = key.site
		if changed {
			result = append(result, manager.editLockSnapshot(siteOp, key.document, cell, true, cause))
		}
		cell.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// AcquireSaveFence在提交前持同doc mutex重查真实文档/delete/隔离与当前租约。
func (manager *EditLockManager) AcquireSaveFence(ctx context.Context, op OperationContext, input DraftFenceInput) (*SaveFenceGuard, error) {
	guard, err := manager.AcquireDocumentFence(ctx, op, input.DocumentID)
	if err != nil {
		return nil, err
	}
	if err := requireEditableDocument(guard.state); err != nil {
		guard.ReleaseFence()
		return nil, err
	}
	if guard.state.DeleteFence != input.ExpectedDeleteFence {
		guard.ReleaseFence()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "文档删除fence已变化")
	}
	if err := manager.matchEditLease(guard.cell, op, input, time.Now().UTC()); err != nil {
		guard.ReleaseFence()
		return nil, err
	}
	guard.lease = copyEditLease(guard.cell.lease)
	return guard, nil
}

// AcquireDocumentFence供已授权生命周期用例在同doc mutex内检查和改变持久状态。
// 不创建文档、不替代状态CAS，也不代替用例的通知/权限/事务提交。
func (manager *EditLockManager) AcquireDocumentFence(ctx context.Context, op OperationContext, documentID Identifier) (*SaveFenceGuard, error) {
	manager.life.RLock()
	if manager.closed {
		manager.life.RUnlock()
		return nil, platform.NewError(platform.CodeLocked, false, "编辑leader已关闭")
	}
	cell, err := manager.lockEditCell(ctx, op, documentID)
	if err != nil {
		manager.life.RUnlock()
		return nil, err
	}
	cell.mu.Lock()
	state, err := manager.store.ReadDocumentEditState(ctx, op, documentID)
	if err != nil {
		cell.mu.Unlock()
		manager.life.RUnlock()
		return nil, err
	}
	return &SaveFenceGuard{manager: manager, cell: cell, state: state}, nil
}

func (guard *SaveFenceGuard) DocumentState() DocumentEditState { return guard.state }
func (guard *SaveFenceGuard) GuardLease() *gen.EditLease       { return copyEditLease(guard.lease) }
func (guard *SaveFenceGuard) ReleaseFence() {
	guard.once.Do(func() { guard.cell.mu.Unlock(); guard.manager.life.RUnlock() })
}

func (manager *EditLockManager) matchEditLease(cell *editLeaseCell, op OperationContext, input DraftFenceInput, now time.Time) error {
	lease := cell.lease
	if cell.exhausted || lease == nil || lease.HolderAccountId != op.Actor.SubjectID || lease.LeaseId != input.LeaseID || lease.ServerBootId != input.ServerBootID || lease.ServerBootId != manager.bootID || int64(lease.FencingToken) != input.FencingToken || editLeaseExpired(lease, now) {
		return platform.NewError(platform.CodeLocked, false, "编辑租约失效或fence已变化")
	}
	return nil
}

func requireEditableDocument(state DocumentEditState) error {
	if state.Quarantined {
		return platform.NewError(platform.CodeIntegrityError, false, "文档已隔离，不能编辑")
	}
	if state.State != "draft" && state.State != "published" {
		return platform.NewError(platform.CodeLocked, false, "文档已删除或不接受编辑")
	}
	return nil
}

func advanceEditFence(cell *editLeaseCell) error {
	if cell.exhausted || cell.fence >= maxEditFence {
		cell.exhausted = true
		cell.lease = nil
		return platform.NewError(platform.CodeLocked, false, "编辑fence已耗尽")
	}
	cell.fence++
	return nil
}

func editLeaseExpired(lease *gen.EditLease, now time.Time) bool {
	return !now.Before(lease.ExpiresAt) || !now.Before(lease.LastInputAt.Add(17*time.Minute))
}

func refreshEditLease(cell *editLeaseCell, now time.Time) (bool, string, error) {
	if cell.lease == nil {
		return false, "unchanged", nil
	}
	if editLeaseExpired(cell.lease, now) {
		if err := advanceEditFence(cell); err != nil {
			return false, "", err
		}
		cell.lease = nil
		return true, "expired", nil
	}
	if cell.lease.IdleWarningAt == nil && !now.Before(cell.lease.LastInputAt.Add(15*time.Minute)) {
		warning := cell.lease.LastInputAt.Add(15 * time.Minute)
		cell.lease.IdleWarningAt = &warning
		return true, "idle_warning", nil
	}
	return false, "unchanged", nil
}

func copyEditLease(lease *gen.EditLease) *gen.EditLease {
	if lease == nil {
		return nil
	}
	copy := *lease
	if lease.IdleWarningAt != nil {
		warning := *lease.IdleWarningAt
		copy.IdleWarningAt = &warning
	}
	return &copy
}

func (manager *EditLockManager) editLockSnapshot(op OperationContext, documentID Identifier, cell *editLeaseCell, changed bool, cause string) EditLockSnapshot {
	return EditLockSnapshot{SiteID: op.Actor.SiteID, DocumentID: documentID, ServerBootID: manager.bootID, FencingToken: cell.fence, Lease: copyEditLease(cell.lease), Writable: cell.lease != nil && cell.lease.HolderAccountId == op.Actor.SubjectID, Changed: changed, Cause: cause}
}

// CloseEditLocks先排空所有操作/guard，才释放leader文件，不允许新旧leader重叠提交。
func (manager *EditLockManager) CloseEditLocks() error {
	manager.life.Lock()
	defer manager.life.Unlock()
	if manager.closed {
		return nil
	}
	manager.closed = true
	lockErr := syscall.Flock(int(manager.leader.Fd()), syscall.LOCK_UN)
	closeErr := manager.leader.Close()
	if lockErr != nil || closeErr != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "编辑leader释放失败")
	}
	return nil
}
