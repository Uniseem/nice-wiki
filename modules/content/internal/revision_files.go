// Package content 实现正文文件及其提交日志的恢复规则。
package content

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

const (
	journalRelativePath = "journal/commits.jsonl"
	quarantineRelative  = "quarantine"
	maxJournalLineBytes = 64 << 10
	maxBodyBytes        = 64 << 20
)

// OperationContext 是服务间调用的可信操作上下文。
type OperationContext = platform.OperationContext

// Identifier 是不透明的稳定标识符。
type Identifier = platform.Identifier

// Hash 是小写 SHA-256 哈希。
type Hash = platform.Hash

// Timestamp 是 RFC3339 UTC 时间。
type Timestamp = platform.Timestamp

// CommitJournal 是正文提交意图。除 state 外的字段在同一 commitId 下不可改变。
type CommitJournal struct {
	CommitID     Identifier  `json:"commitId"`
	OperationID  Identifier  `json:"operationId"`
	DocumentID   Identifier  `json:"documentId"`
	RevisionID   Identifier  `json:"revisionId"`
	BodyHash     Hash        `json:"bodyHash"`
	ExpectedHead *Identifier `json:"expectedHead"`
	State        string      `json:"state"`
	CreatedAt    Timestamp   `json:"createdAt"`
}

// RecoverPreparedCommitsInput 没有字段；恢复参数由受信启动装配注入。
type RecoverPreparedCommitsInput struct{}

// RecoverPreparedCommitsResult 是一次恢复扫描的统计结果。
type RecoverPreparedCommitsResult struct {
	RecoveredCount   int `json:"recoveredCount"`
	QuarantinedCount int `json:"quarantinedCount"`
}

// RevisionRecoveryGate 保证恢复运行在独占启动段或 safe 屏障内。
// release 必须只释放本次调用取得的屏障。
type RevisionRecoveryGate interface {
	EnterRecovery(ctx context.Context) (release func() error, err error)
}

// RevisionRecoveryGateFunc 将函数适配为 RevisionRecoveryGate。
type RevisionRecoveryGateFunc func(context.Context) (func() error, error)

// EnterRecovery 实现 RevisionRecoveryGate。
func (f RevisionRecoveryGateFunc) EnterRecovery(ctx context.Context) (func() error, error) {
	return f(ctx)
}

// RevisionRecoveryState 提供本域私有事务状态的只读检查及隔离写入。
// 它不提供写 head 的方法，恢复函数因此不能擅自推进 head。
type RevisionRecoveryState interface {
	CurrentHead(ctx context.Context, documentID Identifier) (revisionID Identifier, err error)
	HasCommittedReceipt(ctx context.Context, commitID Identifier) (bool, error)
	QuarantineDocument(ctx context.Context, documentID Identifier, reason string) error
}

// RevisionRecoveryStore 是启动时注入的正文恢复依赖。
// ActiveRoot 必须是本域 active 数据根；JournalPath 为空时使用其下的标准路径。
type RevisionRecoveryStore struct {
	ActiveRoot  string
	JournalPath string
	SiteID      Identifier
	DataEpoch   Identifier
	Gate        RevisionRecoveryGate
	State       RevisionRecoveryState

	// ExclusiveStartup 只适用于尚未接入外部 safe 屏障的单进程启动阶段。
	// 正式屏障恢复应提供 Gate；未设置 Gate 且此字段为 false 时拒绝恢复。
	ExclusiveStartup bool
}

type revisionRecoveryContextKey struct{}

// WithRevisionRecoveryStore 将受信恢复依赖放入调用上下文，供启动入口使用。
func WithRevisionRecoveryStore(ctx context.Context, store RevisionRecoveryStore) context.Context {
	return context.WithValue(ctx, revisionRecoveryContextKey{}, store)
}

// startupRecoveryGate 保证显式声明的单进程启动恢复不会并发运行。
var startupRecoveryGate = make(chan struct{}, 1)

// RecoverPreparedCommits 在独占启动段或 safe 屏障内恢复正文提交 journal。
//
// 已提交修订（COMMITTED，或 PREPARED 且已有 committed receipt）核对正文与
// 哈希；只有每篇文档最新的已提交修订才要求当前 head 指向它，校验通过的
// PREPARED 补记 COMMITTED。函数没有推进 head 的能力：已提交正文缺失、哈希
// 不符或最新修订与 head 不一致时隔离并关闭文档；没有 receipt 的孤儿
// PREPARED 只隔离该次提交。
func RecoverPreparedCommits(ctx context.Context, op OperationContext, input RecoverPreparedCommitsInput) (result RecoverPreparedCommitsResult, retErr error) {
	if err := checkRecoveryContext(ctx, op); err != nil {
		return result, err
	}

	store, ok := recoveryStoreFromContext(ctx)
	if !ok {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复依赖未装配")
	}
	siteID, journalPath, err := validateRecoveryStore(store, op)
	if err != nil {
		return result, err
	}

	release, err := enterRecoveryGate(ctx, store)
	if err != nil {
		return result, err
	}
	defer func() {
		if releaseErr := release(); retErr == nil && releaseErr != nil {
			result = RecoverPreparedCommitsResult{}
			retErr = releaseErr
		}
	}()

	if err := checkRecoveryContext(ctx, op); err != nil {
		return result, err
	}
	if err := ensureActiveRoot(store.ActiveRoot); err != nil {
		return result, err
	}

	entries, invalidJournal, err := scanCommitJournal(ctx, store, journalPath)
	if err != nil {
		return result, err
	}
	if invalidJournal {
		return result, platform.NewError(platform.CodeIntegrityError, false, "正文提交日志存在不合法记录")
	}
	if len(entries) == 0 {
		return result, nil
	}
	if store.State == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复事务状态未装配")
	}

	// 第一遍：确定每条提交的有效状态。PREPARED 是否已提交只以私有事务
	// receipt 为准；同一文档按 journal 顺序最后一个有效提交才是当前 head 的来源。
	effective := make([]entryRecovery, 0, len(entries))
	latestCommit := make(map[Identifier]Identifier)
	for _, entry := range entries {
		if err := checkRecoveryContext(ctx, op); err != nil {
			return RecoverPreparedCommitsResult{}, err
		}
		item := entryRecovery{journal: entry.journal}
		switch entry.journal.State {
		case "committed":
			item.committed = true
		case "prepared":
			hasReceipt, receiptErr := store.State.HasCommittedReceipt(ctx, entry.journal.CommitID)
			if receiptErr != nil {
				return RecoverPreparedCommitsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交回执不可读")
			}
			item.committed = hasReceipt
			item.pending = true
		default:
			continue
		}
		if item.committed {
			latestCommit[entry.journal.DocumentID] = entry.journal.CommitID
		}
		effective = append(effective, item)
	}

	integrityFound := false
	for _, item := range effective {
		if err := checkRecoveryContext(ctx, op); err != nil {
			return RecoverPreparedCommitsResult{}, err
		}
		isLatest := latestCommit[item.journal.DocumentID] == item.journal.CommitID
		recovered, quarantined, integrity, err := recoverJournalEntry(ctx, store, siteID, journalPath, item, isLatest)
		if err != nil {
			return RecoverPreparedCommitsResult{}, err
		}
		result.RecoveredCount += recovered
		result.QuarantinedCount += quarantined
		if integrity {
			// 继续扫描同一批，尽可能隔离全部损坏文档；绝不返回部分成功。
			integrityFound = true
		}
	}

	if integrityFound {
		return RecoverPreparedCommitsResult{}, platform.NewError(platform.CodeIntegrityError, false, "正文提交内容或head不一致")
	}
	return result, nil
}

type journalEntry struct {
	journal CommitJournal
}

// entryRecovery 是一条提交在恢复时的有效状态：committed 表示已有 COMMITTED
// 记录或已提交 receipt；pending 表示 journal 仍停在 PREPARED。
type entryRecovery struct {
	journal   CommitJournal
	committed bool
	pending   bool
}

func recoveryStoreFromContext(ctx context.Context) (RevisionRecoveryStore, bool) {
	store, ok := ctx.Value(revisionRecoveryContextKey{}).(RevisionRecoveryStore)
	return store, ok
}

func checkRecoveryContext(ctx context.Context, op OperationContext) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "恢复上下文为空")
	}
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "正文恢复已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "正文恢复已取消")
	default:
	}
	if op.DataEpoch == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复缺少活动数据代次")
	}
	if op.Deadline == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复缺少操作期限")
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复操作期限不可用")
	}
	if !time.Now().UTC().Before(deadline) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "正文恢复已超过期限")
	}
	return nil
}

func validateRecoveryStore(store RevisionRecoveryStore, op OperationContext) (Identifier, string, error) {
	if store.ActiveRoot == "" || !filepath.IsAbs(store.ActiveRoot) {
		return "", "", platform.NewError(platform.CodeValidation, false, "正文恢复数据根不合法")
	}
	root, err := filepath.Abs(filepath.Clean(store.ActiveRoot))
	if err != nil {
		return "", "", platform.NewError(platform.CodeValidation, false, "正文恢复数据根不合法")
	}
	store.ActiveRoot = root

	siteID := store.SiteID
	if siteID == "" {
		siteID = op.Actor.SiteID
	}
	if !platform.ValidIdentifier(siteID) {
		return "", "", platform.NewError(platform.CodeValidation, false, "正文恢复站点标识不合法")
	}
	if store.DataEpoch != "" && store.DataEpoch != op.DataEpoch {
		return "", "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复数据代次不匹配")
	}

	journalPath := store.JournalPath
	if journalPath == "" {
		journalPath = filepath.Join(root, journalRelativePath)
	} else {
		if !filepath.IsAbs(journalPath) {
			return "", "", platform.NewError(platform.CodeValidation, false, "正文提交日志路径不合法")
		}
		journalPath, err = filepath.Abs(filepath.Clean(journalPath))
		if err != nil {
			return "", "", platform.NewError(platform.CodeValidation, false, "正文提交日志路径不合法")
		}
	}
	if !pathWithin(root, journalPath) || filepath.Clean(journalPath) == root {
		return "", "", platform.NewError(platform.CodeValidation, false, "正文提交日志必须位于本域数据根内")
	}
	return siteID, journalPath, nil
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func enterRecoveryGate(ctx context.Context, store RevisionRecoveryStore) (func() error, error) {
	if store.Gate != nil {
		release, err := store.Gate.EnterRecovery(ctx)
		if err != nil {
			return nil, err
		}
		if release == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复屏障未返回释放句柄")
		}
		return release, nil
	}
	if !store.ExclusiveStartup {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复未在独占启动或安全屏障内")
	}
	select {
	case startupRecoveryGate <- struct{}{}:
		return func() error {
			<-startupRecoveryGate
			return nil
		}, nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, platform.NewError(platform.CodeDeadlineExceeded, true, "等待正文恢复屏障已超过期限")
		}
		return nil, platform.NewError(platform.CodeCancelled, true, "等待正文恢复屏障已取消")
	}
}

func ensureActiveRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本域活动数据根不可读")
	}
	if !info.IsDir() {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本域活动数据根不是目录")
	}
	return nil
}

func scanCommitJournal(ctx context.Context, store RevisionRecoveryStore, journalPath string) ([]journalEntry, bool, error) {
	file, err := os.Open(journalPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, platform.NewError(platform.CodeDependencyUnavailable, true, "正文提交日志不可读")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4<<10), maxJournalLineBytes)
	latest := make(map[Identifier]journalEntry)
	order := make([]Identifier, 0)
	invalid := false
	for scanner.Scan() {
		if err := checkContextOnly(ctx); err != nil {
			return nil, false, err
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		journal, decodeErr := decodeCommitJournal(line)
		if decodeErr != nil {
			invalid = true
			if err := quarantineInvalidJournalMarker(ctx, store, line); err != nil {
				return nil, false, err
			}
			continue
		}
		entry, exists := latest[journal.CommitID]
		if !exists {
			latest[journal.CommitID] = journalEntry{journal: journal}
			order = append(order, journal.CommitID)
			continue
		}
		if !sameJournalIdentity(entry.journal, journal) || !validJournalTransition(entry.journal.State, journal.State) {
			invalid = true
			continue
		}
		entry.journal = journal
		latest[journal.CommitID] = entry
	}
	if err := scanner.Err(); err != nil {
		return nil, false, platform.NewError(platform.CodeIntegrityError, false, "正文提交日志记录过长或读取失败")
	}

	entries := make([]journalEntry, 0, len(order))
	for _, commitID := range order {
		entries = append(entries, latest[commitID])
	}
	return entries, invalid, nil
}

func decodeCommitJournal(line []byte) (CommitJournal, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	var journal CommitJournal
	if err := decoder.Decode(&journal); err != nil {
		return CommitJournal{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return CommitJournal{}, errors.New("正文提交日志包含多个JSON值")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(line, &fields); err != nil || fields == nil {
		return CommitJournal{}, errors.New("正文提交日志不是对象")
	}
	for _, field := range []string{"commitId", "operationId", "documentId", "revisionId", "bodyHash", "expectedHead", "state", "createdAt"} {
		if _, ok := fields[field]; !ok {
			return CommitJournal{}, errors.New("正文提交日志缺少字段")
		}
	}
	if !validCommitJournal(journal) {
		return CommitJournal{}, errors.New("正文提交日志字段不合法")
	}
	return journal, nil
}

func validCommitJournal(journal CommitJournal) bool {
	if !platform.ValidIdentifier(journal.CommitID) ||
		!platform.ValidIdentifier(journal.OperationID) ||
		!platform.ValidIdentifier(journal.DocumentID) ||
		!platform.ValidIdentifier(journal.RevisionID) ||
		!platform.ValidHash(journal.BodyHash) {
		return false
	}
	if journal.ExpectedHead != nil && !platform.ValidIdentifier(*journal.ExpectedHead) {
		return false
	}
	switch journal.State {
	case "prepared", "committed", "aborted", "quarantined":
	default:
		return false
	}
	_, err := platform.ParseTimestamp(journal.CreatedAt)
	return err == nil
}

func sameJournalIdentity(left, right CommitJournal) bool {
	if left.CommitID != right.CommitID || left.OperationID != right.OperationID ||
		left.DocumentID != right.DocumentID || left.RevisionID != right.RevisionID ||
		left.BodyHash != right.BodyHash || left.CreatedAt != right.CreatedAt {
		return false
	}
	if left.ExpectedHead == nil || right.ExpectedHead == nil {
		return left.ExpectedHead == nil && right.ExpectedHead == nil
	}
	return *left.ExpectedHead == *right.ExpectedHead
}

func validJournalTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "prepared":
		return to == "committed" || to == "aborted" || to == "quarantined"
	case "committed":
		return to == "quarantined"
	default:
		return false
	}
}

// recoverJournalEntry 恢复单条提交。已提交修订必须正文与哈希一致，且只有
// 每篇文档最新的已提交修订才要求当前 head 指向它——更早的修订本来就已
// 被后续提交取代；不一致或缺失时隔离文档并关闭公开（合同步骤2）。没有已
// 提交 receipt 的 PREPARED 是未完成的孤儿，只隔离该次提交，不推进 head，
// 也不关闭文档（合同步骤3）。
func recoverJournalEntry(ctx context.Context, store RevisionRecoveryStore, siteID Identifier, journalPath string, item entryRecovery, isLatest bool) (recovered, quarantined int, integrity bool, err error) {
	journal := item.journal
	if !item.committed {
		if err := quarantineCommit(ctx, store, siteID, journalPath, journal, "PREPARED提交没有已提交回执", false); err != nil {
			return 0, 0, false, err
		}
		return 0, 1, false, nil
	}
	valid, reason, checkErr := verifyCommittedBodyAndHead(ctx, store, siteID, journal, isLatest)
	if checkErr != nil {
		return 0, 0, false, checkErr
	}
	if !valid {
		if err := quarantineCommit(ctx, store, siteID, journalPath, journal, reason, true); err != nil {
			return 0, 0, false, err
		}
		return 0, 1, true, nil
	}
	if item.pending {
		if err := appendJournalState(ctx, journalPath, journal, "committed"); err != nil {
			return 0, 0, false, err
		}
	}
	return 1, 0, false, nil
}

func verifyCommittedBodyAndHead(ctx context.Context, store RevisionRecoveryStore, siteID Identifier, journal CommitJournal, checkHead bool) (bool, string, error) {
	bodyPath := revisionBodyPath(store.ActiveRoot, siteID, journal.DocumentID, journal.RevisionID)
	status, actualHash, err := readRevisionHash(bodyPath)
	if err != nil {
		return false, "COMMITTED正文不可读", err
	}
	switch status {
	case bodyMissing:
		return false, "COMMITTED正文缺失", nil
	case bodyInvalid:
		return false, "COMMITTED正文文件不合法", nil
	case bodyValid:
		if actualHash != journal.BodyHash {
			return false, "COMMITTED正文哈希不符", nil
		}
	}

	if !checkHead {
		return true, "", nil
	}
	currentHead, err := store.State.CurrentHead(ctx, journal.DocumentID)
	if err != nil {
		return false, "正文head不可读", platform.NewError(platform.CodeDependencyUnavailable, true, "正文head不可读")
	}
	if currentHead != journal.RevisionID {
		// expectedHead 只用于说明 CAS 前置值，绝不能在恢复中代替当前 head。
		return false, "COMMITTED正文head不一致", nil
	}
	return true, "", nil
}

const (
	bodyMissing = iota
	bodyInvalid
	bodyValid
)

func readRevisionHash(path string) (status int, hash Hash, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return bodyMissing, "", nil
		}
		return bodyInvalid, "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文文件不可检查")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxBodyBytes {
		return bodyInvalid, "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return bodyMissing, "", nil
		}
		return bodyInvalid, "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文文件不可读")
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.CopyN(hasher, file, maxBodyBytes+1); err != nil && !errors.Is(err, io.EOF) {
		return bodyInvalid, "", platform.NewError(platform.CodeDependencyUnavailable, true, "正文文件读取失败")
	}
	return bodyValid, Hash(hex.EncodeToString(hasher.Sum(nil))), nil
}

// quarantineCommit 把该次提交的正文移入隔离区并记 quarantined；closeDocument
// 为 true 时（已提交正文损坏或 head 不一致）同时隔离文档并关闭公开。
func quarantineCommit(ctx context.Context, store RevisionRecoveryStore, siteID Identifier, journalPath string, journal CommitJournal, reason string, closeDocument bool) error {
	if store.State == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复事务状态未装配")
	}
	if err := checkContextOnly(ctx); err != nil {
		return err
	}
	bodyPath := revisionBodyPath(store.ActiveRoot, siteID, journal.DocumentID, journal.RevisionID)
	if err := moveBodyToQuarantine(ctx, store.ActiveRoot, siteID, journal, bodyPath); err != nil {
		return err
	}
	if err := checkContextOnly(ctx); err != nil {
		return err
	}
	if closeDocument {
		if err := store.State.QuarantineDocument(ctx, journal.DocumentID, reason); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "正文文档隔离状态不可写")
		}
	}
	if err := appendJournalState(ctx, journalPath, journal, "quarantined"); err != nil {
		return err
	}
	return nil
}

func moveBodyToQuarantine(ctx context.Context, root, siteID string, journal CommitJournal, bodyPath string) error {
	if err := checkContextOnly(ctx); err != nil {
		return err
	}
	if _, err := os.Lstat(bodyPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, "待隔离正文不可检查")
	}
	directory := filepath.Join(root, quarantineRelative, "revisions", siteID, journal.DocumentID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "建立正文隔离目录失败")
	}
	base := journal.RevisionID + "." + journal.CommitID + ".json"
	for suffix := 0; suffix < 1000; suffix++ {
		name := base
		if suffix > 0 {
			name = journal.RevisionID + "." + journal.CommitID + "." + strconv.Itoa(suffix) + ".json"
		}
		destination := filepath.Join(directory, name)
		if _, err := os.Lstat(destination); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "正文隔离目标不可检查")
		}
		if err := os.Rename(bodyPath, destination); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return platform.NewError(platform.CodeDependencyUnavailable, true, "正文隔离失败")
		}
		if err := syncDirectory(filepath.Dir(bodyPath)); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "正文目录持久化失败")
		}
		if err := syncDirectory(directory); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "正文隔离目录持久化失败")
		}
		return nil
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "正文隔离目标冲突过多")
}

func appendJournalState(ctx context.Context, journalPath string, journal CommitJournal, state string) error {
	if err := checkContextOnly(ctx); err != nil {
		return err
	}
	journal.State = state
	data, err := json.Marshal(journal)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "正文恢复记录编码失败")
	}
	data = append(data, '\n')
	file, err := os.OpenFile(journalPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复日志不可写")
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复日志写入失败")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复日志持久化失败")
	}
	if err := file.Close(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文恢复日志关闭失败")
	}
	if err := syncDirectory(filepath.Dir(journalPath)); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "正文日志目录持久化失败")
	}
	return nil
}

func quarantineInvalidJournalMarker(ctx context.Context, store RevisionRecoveryStore, line []byte) error {
	if err := checkContextOnly(ctx); err != nil {
		return err
	}
	directory := filepath.Join(store.ActiveRoot, quarantineRelative, "journal")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "建立日志隔离目录失败")
	}
	digest := sha256.Sum256(line)
	name := "invalid-" + hex.EncodeToString(digest[:]) + ".json"
	path := filepath.Join(directory, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, "日志隔离标记不可写")
	}
	marker := []byte("{\"reason\":\"invalid commit journal entry\",\"lineHash\":\"" + hex.EncodeToString(digest[:]) + "\"}\n")
	if _, err := file.Write(marker); err != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "日志隔离标记写入失败")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "日志隔离标记持久化失败")
	}
	if err := file.Close(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "日志隔离标记关闭失败")
	}
	if err := syncDirectory(directory); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "日志隔离目录持久化失败")
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

func revisionBodyPath(root, siteID, documentID, revisionID string) string {
	return filepath.Join(root, siteID, documentID, revisionID+".json")
}

func checkContextOnly(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "正文恢复已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "正文恢复已取消")
	default:
		return nil
	}
}
