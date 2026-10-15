// Package content 提供正文提交的事务边界。
package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

// DraftRevision 是共享草稿的不可变正文版本。
type DraftRevision = gen.DraftRevision

// CommitDraftJournalInput 是草稿head提交的完整声明。
type CommitDraftJournalInput struct {
	Draft                DraftRevision `json:"draft"`
	Journal              CommitJournal `json:"journal"`
	ExpectedHead         *Identifier   `json:"expectedHead"`
	ExpectedDraftVersion gen.Revision  `json:"expectedDraftVersion"`
}

// CommitDraftJournalResult 是成功提交后的共享草稿版本。
type CommitDraftJournalResult struct {
	Draft DraftRevision `json:"draft"`
}

// DraftCommitRequest 是事务实现收到的已核验提交声明。
// BodyPath和CanonicalDigest只用于事务内定位与幂等，不得写入对外日志。
type DraftCommitRequest struct {
	Operation       OperationContext
	Input           CommitDraftJournalInput
	CanonicalDigest Hash
	BodyPath        string
}

// DraftCommitStore 是本域私有存储的装配边界。
// BeginDraftCommit必须打开同一个私有事务；事务实现负责数据库行CAS、稳定ID台账、
// COMMITTED journal、引用、receipt和outbox的原子提交；同head CAS拒绝历史retired ID复用。
type DraftCommitStore interface {
	RevisionRoot() string
	RevisionSiteID() Identifier
	BeginDraftCommit(context.Context, DraftCommitRequest) (DraftCommitTransaction, error)
}

// DraftCommitTransaction 是草稿提交的单事务操作集。
type DraftCommitTransaction interface {
	// FindReceipt按operationId和规范摘要查询已提交回执。命中时不得再次改head。
	FindReceipt(context.Context, Identifier, Hash) (CommitDraftJournalResult, bool, error)
	// ValidateFence在事务内核对当前编辑锁、fence和删除/屏障状态。
	ValidateFence(context.Context, CommitDraftJournalInput) error
	// Commit原子完成head/version CAS、ledger、COMMITTED journal、refs、receipt和outbox。
	Commit(context.Context, DraftCommitRequest) (CommitDraftJournalResult, error)
	// Rollback释放事务；已经成功Commit后不得调用。
	Rollback(context.Context) error
}

type draftCommitStoreContextKey struct{}

// WithDraftCommitStore 将受信本域私有提交存储放入调用上下文。
func WithDraftCommitStore(ctx context.Context, store DraftCommitStore) context.Context {
	return context.WithValue(ctx, draftCommitStoreContextKey{}, store)
}

func draftCommitStoreFromContext(ctx context.Context) (DraftCommitStore, bool) {
	store, ok := ctx.Value(draftCommitStoreContextKey{}).(DraftCommitStore)
	return store, ok
}

// CommitDraftJournal核验不可变正文文件后，以单个私有事务提交共享draft head。
// 具体数据库和outbox写入由DraftCommitTransaction实现，避免本域函数绕过事务边界。
func CommitDraftJournal(ctx context.Context, op OperationContext, input CommitDraftJournalInput) (result CommitDraftJournalResult, retErr error) {
	if err := validateDraftCommitContext(ctx, op); err != nil {
		return result, err
	}
	if err := validateDraftCommitInput(op, input); err != nil {
		return result, err
	}

	store, ok := draftCommitStoreFromContext(ctx)
	if !ok || store == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交存储未装配")
	}
	root, siteID, err := validateDraftCommitStore(store, op)
	if err != nil {
		return result, err
	}

	digest, err := draftCommitDigest(input)
	if err != nil {
		return result, platform.NewError(platform.CodeInternal, false, "正文提交摘要计算失败")
	}
	bodyPath := revisionBodyPath(root, siteID, input.Draft.DocumentId, input.Draft.RevisionId)
	request := DraftCommitRequest{
		Operation:       op,
		Input:           input,
		CanonicalDigest: digest,
		BodyPath:        bodyPath,
	}

	tx, err := store.BeginDraftCommit(ctx, request)
	if err != nil {
		return result, normalizeDraftCommitError(err, "正文提交事务不可用")
	}
	if tx == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交事务未装配")
	}
	finished := false
	defer func() {
		if finished {
			return
		}
		if rollbackErr := tx.Rollback(context.Background()); retErr == nil && rollbackErr != nil {
			retErr = normalizeDraftCommitError(rollbackErr, "正文提交事务回滚失败")
			result = CommitDraftJournalResult{}
		}
	}()

	// 已有receipt必须在正文核验前返回，保证客户端超时重试不重复改head。
	if receipt, found, err := tx.FindReceipt(ctx, op.OperationID, digest); err != nil {
		return result, normalizeDraftCommitError(err, "正文提交回执不可读")
	} else if found {
		finished = true
		return receipt, nil
	}

	if err := tx.ValidateFence(ctx, input); err != nil {
		return result, normalizeDraftCommitError(err, "正文提交fence校验失败")
	}
	if err := verifyPreparedRevisionFile(ctx, bodyPath, input.Draft); err != nil {
		return result, err
	}
	if err := checkDraftCommitContext(ctx, op); err != nil {
		return result, err
	}

	result, err = tx.Commit(ctx, request)
	if err != nil {
		return CommitDraftJournalResult{}, normalizeDraftCommitError(err, "正文提交事务失败")
	}
	finished = true
	return result, nil
}

func validateDraftCommitContext(ctx context.Context, op OperationContext) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "正文提交上下文为空")
	}
	if !platform.ValidIdentifier(op.OperationID) || !platform.ValidIdentifier(op.RequestID) {
		return platform.NewError(platform.CodeValidation, false, "正文提交操作标识不合法")
	}
	if !platform.ValidIdentifier(op.DataEpoch) || !platform.ValidIdentifier(op.Actor.SiteID) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交运行代次或站点不可用")
	}
	if op.Deadline == "" {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "正文提交缺少操作期限")
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "正文提交操作期限不合法")
	}
	if !time.Now().UTC().Before(deadline) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "正文提交已超过期限")
	}
	return checkDraftCommitContext(ctx, op)
}

func checkDraftCommitContext(ctx context.Context, op OperationContext) error {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "正文提交已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "正文提交已取消")
	default:
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil || !time.Now().UTC().Before(deadline) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "正文提交已超过期限")
	}
	return nil
}

func validateDraftCommitInput(op OperationContext, input CommitDraftJournalInput) error {
	draft := input.Draft
	journal := input.Journal
	if !platform.ValidIdentifier(draft.DocumentId) || !platform.ValidIdentifier(draft.RevisionId) ||
		!platform.ValidIdentifier(draft.AuthorAccountId) || !platform.ValidIdentifier(journal.CommitID) ||
		!platform.ValidIdentifier(journal.OperationID) || !platform.ValidIdentifier(journal.DocumentID) ||
		!platform.ValidIdentifier(journal.RevisionID) || !platform.ValidHash(draft.BodyHash) ||
		!platform.ValidHash(journal.BodyHash) {
		return platform.NewError(platform.CodeIntegrityError, false, "正文提交标识或哈希不合法")
	}
	if journal.OperationID != op.OperationID || journal.DocumentID != draft.DocumentId ||
		journal.RevisionID != draft.RevisionId || journal.BodyHash != draft.BodyHash ||
		journal.State != "prepared" {
		return platform.NewError(platform.CodeIntegrityError, false, "正文提交日志与草稿不一致")
	}
	if !sameIdentifierPointer(input.ExpectedHead, draft.PreviousRevisionId) ||
		(input.ExpectedHead != nil && !platform.ValidIdentifier(*input.ExpectedHead)) {
		return platform.NewError(platform.CodeVersionMismatch, false, "正文提交旧head不一致")
	}
	if !sameIdentifierPointer(input.ExpectedHead, journal.ExpectedHead) {
		return platform.NewError(platform.CodeIntegrityError, false, "正文提交日志旧head不一致")
	}
	if input.ExpectedDraftVersion < 0 || draft.DraftVersion < 1 || draft.DraftVersion != input.ExpectedDraftVersion+1 {
		return platform.NewError(platform.CodeVersionMismatch, false, "正文提交草稿版本不连续")
	}
	if draft.Object.Owner != "content" || !platform.ValidIdentifier(draft.Object.ObjectId) ||
		!platform.ValidHash(draft.Object.Sha256) || draft.Object.Sha256 != draft.BodyHash ||
		draft.Object.ByteSize < 0 || draft.Object.ByteSize > maxBodyBytes ||
		draft.Object.MediaType != "application/json" || draft.Object.Version < 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "正文对象引用不合法")
	}
	if !platform.ValidIdentifier(journal.CommitID) || journal.CreatedAt == "" {
		return platform.NewError(platform.CodeIntegrityError, false, "正文提交日志字段不完整")
	}
	if _, err := platform.ParseTimestamp(journal.CreatedAt); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "正文提交日志时间不合法")
	}
	return nil
}

func validateDraftCommitStore(store DraftCommitStore, op OperationContext) (string, Identifier, error) {
	root := store.RevisionRoot()
	if root == "" || !filepath.IsAbs(root) {
		return "", "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交数据根不可用")
	}
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交数据根不可用")
	}
	siteID := store.RevisionSiteID()
	if !platform.ValidIdentifier(siteID) || siteID != op.Actor.SiteID {
		return "", "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交站点作用域不可用")
	}
	return root, siteID, nil
}

func verifyPreparedRevisionFile(ctx context.Context, path string, draft DraftRevision) error {
	if err := checkDraftCommitIOContext(ctx); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return platform.NewError(platform.CodeIntegrityError, false, "PREPARED正文不存在")
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, "PREPARED正文不可检查")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxBodyBytes {
		return platform.NewError(platform.CodeIntegrityError, false, "PREPARED正文文件类型或大小不合法")
	}
	if int64(draft.Object.ByteSize) != info.Size() {
		return platform.NewError(platform.CodeIntegrityError, false, "PREPARED正文大小不一致")
	}
	file, err := os.Open(path)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "PREPARED正文不可读")
	}
	hasher := sha256.New()
	readSize, copyErr := io.CopyN(hasher, revisionContextReader{ctx: ctx, reader: file}, maxBodyBytes+1)
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "PREPARED正文读取失败")
	}
	if syncErr := file.Sync(); syncErr != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "PREPARED正文持久性校验失败")
	}
	if closeErr := file.Close(); closeErr != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "PREPARED正文关闭失败")
	}
	if readSize != int64(draft.Object.ByteSize) {
		return platform.NewError(platform.CodeIntegrityError, false, "PREPARED正文读取大小不一致")
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != string(draft.BodyHash) {
		return platform.NewError(platform.CodeIntegrityError, false, "PREPARED正文哈希不一致")
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "PREPARED正文目录不可持久化")
	}
	return nil
}

func checkDraftCommitIOContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "正文提交已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "正文提交已取消")
	default:
		return nil
	}
}

func draftCommitDigest(input CommitDraftJournalInput) (Hash, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return Hash(hex.EncodeToString(digest[:])), nil
}

func sameIdentifierPointer(left, right *Identifier) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func normalizeDraftCommitError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	var platformErr *platform.Error
	if errors.As(err, &platformErr) {
		return err
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "%s", fallback)
}
