package content

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	pending   *pendingEditLock
}

// EditLockManager持有单leader与文档互斥；完整用例在同锁下持久原回执、通知和审计，
// 候选事务确认前不发布内存租约。HTTP与后台入口仍由启动装配负责。
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
	TabID       Identifier
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

// EditLockSnapshot持久于原回执；仅execute完成通知/审计事务后才返回成功。
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

// AcquireLease只接受完整用例取得的identity权威姓名。
func (manager *EditLockManager) AcquireLease(ctx context.Context, op OperationContext, input AcquireLeaseInput) (EditLockSnapshot, error) {
	return manager.executeEditLock(ctx, op, editLockCommand{action: "acquire", documentID: input.DocumentID, accountName: input.AccountName, input: AcquireEditLockInput{DocumentId: input.DocumentID, TabId: input.TabID}})
}

func (manager *EditLockManager) RenewLease(ctx context.Context, op OperationContext, input RenewEditLockInput) (EditLockSnapshot, error) {
	return manager.executeEditLock(ctx, op, editLockCommand{action: "renew", documentID: input.DocumentId, input: input})
}

func (manager *EditLockManager) ReleaseLease(ctx context.Context, op OperationContext, input ReleaseEditLockInput) (EditLockSnapshot, error) {
	return manager.executeEditLock(ctx, op, editLockCommand{action: "release", documentID: input.DocumentId, input: input})
}

func (manager *EditLockManager) ForceReleaseLease(ctx context.Context, op OperationContext, input ForceReleaseEditLockInput) (EditLockSnapshot, error) {
	return manager.executeEditLock(ctx, op, editLockCommand{action: "force", documentID: input.DocumentId, input: input})
}

// ExpireLeases由完整后台用例限定本站，逐文档提交；同次后台重试复用每篇原回执。
func (manager *EditLockManager) ExpireLeases(ctx context.Context, op OperationContext, now time.Time) ([]EditLockSnapshot, error) {
	return manager.expireEditLocks(ctx, op, now)
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
	if err := manager.resolvePendingEditLock(ctx, cell); err != nil {
		cell.mu.Unlock()
		manager.life.RUnlock()
		return nil, err
	}
	state, err := manager.store.ReadDocumentEditState(ctx, op, documentID)
	if err != nil {
		cell.mu.Unlock()
		manager.life.RUnlock()
		return nil, err
	}
	return &SaveFenceGuard{manager: manager, cell: cell, state: state}, nil
}

func (guard *SaveFenceGuard) DocumentState() DocumentEditState { return guard.state }
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
