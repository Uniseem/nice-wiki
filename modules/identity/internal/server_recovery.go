// Package identity 提供服务器本机恢复所需的账号索引与超管口令重置边界。
//
// 账号索引只保留可供人工查找的账号元数据，不复制 credential、session
// 或任何口令相关字段。重置口令只在内存中读取，回执提交后删除固定文件。
// 两类文件都位于 control 区，control 区不随业务数据恢复替换。
package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
)

const (
	accountIndexFileName    = "account-index.json"
	superadminResetFileName = "reset-superadmin.txt"
	maxAccountIndexBytes    = 16 << 20
	maxIndexedAccounts      = 10000
	maxSuperadminResetBytes = passwordMaxRunes * utf8.UTFMax
	superadminResetInterval = 5000 * time.Millisecond
)

var (
	accountIndexWriteMu sync.Mutex
	superadminResetMu   sync.Mutex
)

type serverAccountIndexContextKey struct{}

// serverAccountIndexContext 是本机运维调用的受信存储边界。数据根和活动
// 代次由进程装配层提供，不能由账号索引输入或浏览器请求自声明。
type serverAccountIndexContext struct {
	dataRoot  string
	dataEpoch platform.Identifier
}

// WithServerAccountIndexContext 为本机账号索引操作装入固定数据根和运行代次。
// 这是启动入口与后续身份业务装配之间的内部边界，不是HTTP输入。
func WithServerAccountIndexContext(ctx context.Context, dataRoot string, dataEpoch platform.Identifier) context.Context {
	return context.WithValue(ctx, serverAccountIndexContextKey{}, serverAccountIndexContext{
		dataRoot:  dataRoot,
		dataEpoch: dataEpoch,
	})
}

// ReadServerAccountIndexInput 是服务器运维查明文账号索引的输入。
type ReadServerAccountIndexInput struct{}

// ReadServerAccountIndexResult 是服务器运维查明文账号索引的结果。
type ReadServerAccountIndexResult struct {
	Index AccountIndex `json:"index"`
}

// WriteAccountIndexInput 是维持服务器查账号能力的输入。
type WriteAccountIndexInput struct {
	Accounts []Account         `json:"accounts"`
	Revision platform.Revision `json:"revision"`
}

// WriteAccountIndexResult 是维持服务器查账号能力的结果。
type WriteAccountIndexResult struct {
	Index AccountIndex `json:"index"`
}

// AccountIndex 是control/account-index.json的脱敏内容。
type AccountIndex struct {
	Revision platform.Revision   `json:"revision"`
	Accounts []AccountIndexEntry `json:"accounts"`
}

// AccountIndexEntry 是供服务器人工查找的最小账号记录。
type AccountIndexEntry struct {
	AccountID    platform.Identifier `json:"accountId"`
	AccountName  AccountName         `json:"accountName"`
	IsSuperadmin bool                `json:"isSuperadmin"`
	Status       AccountStatus       `json:"status"`
}

// ReadServerAccountIndex 读取并核对固定私有control索引。索引过期时从
// active/identity.db的权威accounts表重建；无论读取还是重建都不输出口令哈希。
func ReadServerAccountIndex(ctx context.Context, op OperationContext, input ReadServerAccountIndexInput) (ReadServerAccountIndexResult, error) {
	state, err := accountIndexContext(ctx, op, false)
	if err != nil {
		return ReadServerAccountIndexResult{}, err
	}

	index, _, err := readAccountIndexFile(state.dataRoot)
	missing := false
	if err != nil {
		// 固定索引文件尚不存在等同于最滞后的索引，由权威表首次建立；
		// 其他读取失败（损坏、权限不安全等）仍按原错误拒绝。
		if !isCode(err, platform.CodeDependencyUnavailable) || !accountIndexFileMissing(state.dataRoot) {
			return ReadServerAccountIndexResult{}, err
		}
		missing = true
	}
	authority, err := readAccountAuthority(ctx, state.dataRoot)
	if err != nil {
		return ReadServerAccountIndexResult{}, err
	}
	if !missing && accountIndexMatchesAuthority(index, authority) {
		return ReadServerAccountIndexResult{Index: index}, nil
	}

	// 只在索引缺失或结构完整但水位落后于权威表时重建；损坏索引在
	// 上面的读取阶段已经返回INTEGRITY_ERROR，不把未知状态当作允许。
	if !missing && index.Revision > authority.revision {
		return ReadServerAccountIndexResult{}, platform.NewError(platform.CodeIntegrityError, false, "账号索引revision领先权威记录")
	}
	rebuilt, err := WriteAccountIndex(ctx, op, WriteAccountIndexInput{
		Accounts: authority.accounts,
		Revision: authority.revision,
	})
	if err != nil {
		return ReadServerAccountIndexResult{}, err
	}
	return ReadServerAccountIndexResult{Index: rebuilt.Index}, nil
}

// WriteAccountIndex 从完整账号元数据中抽取最小索引，并以同目录临时文件、
// fsync、rename、目录fsync的顺序提交。低revision永不覆盖较新的索引。
func WriteAccountIndex(ctx context.Context, op OperationContext, input WriteAccountIndexInput) (WriteAccountIndexResult, error) {
	accountIndexWriteMu.Lock()
	defer accountIndexWriteMu.Unlock()
	state, err := accountIndexContext(ctx, op, true)
	if err != nil {
		return WriteAccountIndexResult{}, err
	}
	index, encoded, err := encodeAccountIndex(input)
	if err != nil {
		return WriteAccountIndexResult{}, err
	}

	controlDir, err := ensurePrivateControlDir(state.dataRoot)
	if err != nil {
		return WriteAccountIndexResult{}, err
	}
	path := filepath.Join(controlDir, accountIndexFileName)
	current, _, readErr := readAccountIndexFile(state.dataRoot)
	if readErr != nil && !isCode(readErr, platform.CodeDependencyUnavailable) {
		return WriteAccountIndexResult{}, readErr
	}
	if readErr == nil {
		if current.Revision > index.Revision {
			return WriteAccountIndexResult{Index: current}, nil
		}
		if current.Revision == index.Revision {
			canonicalCurrent, marshalErr := json.MarshalIndent(current, "", "  ")
			if marshalErr != nil {
				return WriteAccountIndexResult{}, platform.NewError(platform.CodeIntegrityError, false, "现有账号索引不可编码")
			}
			canonicalCurrent = append(canonicalCurrent, '\n')
			if string(canonicalCurrent) == string(encoded) || platform.HashBytes(canonicalCurrent) == platform.HashBytes(encoded) {
				return WriteAccountIndexResult{Index: current}, nil
			}
			return WriteAccountIndexResult{}, platform.NewError(platform.CodeIntegrityError, false, "同revision账号索引内容不一致")
		}
	} else if isCode(readErr, platform.CodeDependencyUnavailable) {
		// readAccountIndexFile将缺失文件也映射为DEPENDENCY_UNAVAILABLE；
		// 只有固定文件不存在时才允许首次建立。
		if _, statErr := os.Stat(path); statErr == nil || !os.IsNotExist(statErr) {
			return WriteAccountIndexResult{}, readErr
		}
	}

	if err := contextOperationError(ctx, op); err != nil {
		return WriteAccountIndexResult{}, err
	}
	temporary, err := os.CreateTemp(controlDir, ".account-index-*.tmp")
	if err != nil {
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引临时文件不可创建")
	}
	temporaryName := temporary.Name()
	keepTemporary := true
	defer func() {
		if keepTemporary {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引权限不可设置")
	}
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引临时文件不可写入")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引fsync失败")
	}
	if err := temporary.Close(); err != nil {
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引临时文件不可关闭")
	}
	if err := contextOperationError(ctx, op); err != nil {
		return WriteAccountIndexResult{}, err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引替换失败")
	}
	keepTemporary = false
	directory, err := os.Open(controlDir)
	if err != nil {
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引目录不可打开")
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引目录fsync失败")
	}
	return WriteAccountIndexResult{Index: index}, nil
}

type accountAuthority struct {
	accounts []Account
	revision platform.Revision
}

func readAccountAuthority(ctx context.Context, dataRoot string) (accountAuthority, error) {
	path := filepath.Join(dataRoot, "active", databaseName)
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(3000)")
	if err != nil {
		return accountAuthority{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号权威库不可打开")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return accountAuthority{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号权威库不可用")
	}
	return readAccountAuthorityQuery(ctx, db)
}

// readAccountAuthorityQuery 可读取同一未提交账号事务，供提交前索引编码上限核对。
func readAccountAuthorityQuery(ctx context.Context, query sessionQueryer) (accountAuthority, error) {
	rows, err := query.QueryContext(ctx, `SELECT account_id, account_name, is_superadmin, status, revision FROM accounts WHERE site_id NOT LIKE 'home:%' ORDER BY account_id`)
	if err != nil {
		return accountAuthority{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号权威记录不可读")
	}
	defer rows.Close()
	result := accountAuthority{accounts: make([]Account, 0)}
	seenIDs := map[string]struct{}{}
	seenNames := map[string]struct{}{}
	superadminCount := 0
	for rows.Next() {
		if len(result.accounts) >= maxIndexedAccounts {
			return accountAuthority{}, platform.NewError(platform.CodeIntegrityError, false, "账号权威记录数量超限")
		}
		var account Account
		var superadmin int
		if err := rows.Scan(&account.AccountID, &account.AccountName, &superadmin, &account.Status, &account.Revision); err != nil {
			return accountAuthority{}, platform.NewError(platform.CodeIntegrityError, false, "账号权威记录损坏")
		}
		if superadmin != 0 && superadmin != 1 {
			return accountAuthority{}, platform.NewError(platform.CodeIntegrityError, false, "账号超管标记损坏")
		}
		account.IsSuperadmin = superadmin == 1
		if account.IsSuperadmin {
			superadminCount++
		}
		if err := validateIndexAccount(account, seenIDs, seenNames); err != nil {
			return accountAuthority{}, platform.NewError(platform.CodeIntegrityError, false, "账号权威记录不合法")
		}
		result.accounts = append(result.accounts, account)
		// 账号集合水位取 Σ(revision+1)：账户只做墓碑不物理删除、revision 只增，
		// 因此新建账户或任一账户状态变化都会严格推高水位；取最大值会让新建的
		// revision=1 子账号与旧索引同水位却内容不同，被误判为完整性错误。
		result.revision += account.Revision + 1
	}
	if err := rows.Err(); err != nil {
		return accountAuthority{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号权威记录读取失败")
	}
	if len(result.accounts) == 0 || superadminCount != 1 {
		return accountAuthority{}, platform.NewError(platform.CodeDependencyUnavailable, true, "尚未存在账号权威记录")
	}
	return result, nil
}

func accountIndexContext(ctx context.Context, op OperationContext, writing bool) (serverAccountIndexContext, error) {
	if err := contextOperationError(ctx, op); err != nil {
		return serverAccountIndexContext{}, err
	}
	state, ok := ctx.Value(serverAccountIndexContextKey{}).(serverAccountIndexContext)
	if !ok || !filepath.IsAbs(state.dataRoot) || !platform.ValidIdentifier(state.dataEpoch) {
		return serverAccountIndexContext{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引运行权威未装配")
	}
	if state.dataEpoch != op.DataEpoch {
		return serverAccountIndexContext{}, platform.NewError(platform.CodeVersionMismatch, true, "账号索引数据代次不一致")
	}
	return state, nil
}

func contextOperationError(ctx context.Context, op OperationContext) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "账号索引上下文缺失")
	}
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "账号索引操作已超时")
		}
		return platform.NewError(platform.CodeCancelled, true, "账号索引操作已取消")
	default:
	}
	if !platform.ValidIdentifier(op.DataEpoch) || op.Deadline == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引缺少运行代次或操作期限")
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil || !deadline.After(time.Now().UTC()) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "账号索引操作已超过期限")
	}
	return nil
}

func buildAccountIndex(input WriteAccountIndexInput) (AccountIndex, error) {
	if input.Revision < 0 || len(input.Accounts) < 1 || len(input.Accounts) > maxIndexedAccounts {
		return AccountIndex{}, platform.NewError(platform.CodeValidation, false, "账号索引输入范围不合法")
	}
	ids := map[string]struct{}{}
	names := map[string]struct{}{}
	entries := make([]AccountIndexEntry, 0, len(input.Accounts))
	for _, account := range input.Accounts {
		if err := validateIndexAccount(account, ids, names); err != nil {
			return AccountIndex{}, err
		}
		entries = append(entries, AccountIndexEntry{
			AccountID:    account.AccountID,
			AccountName:  account.AccountName,
			IsSuperadmin: account.IsSuperadmin,
			Status:       account.Status,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].AccountID < entries[j].AccountID })
	return AccountIndex{Revision: input.Revision, Accounts: entries}, nil
}

func validateIndexAccount(account Account, ids, names map[string]struct{}) error {
	if !platform.ValidIdentifier(account.AccountID) || account.AccountName == "" || !validAccountStatus(account.Status) || account.Revision < 0 {
		return platform.NewError(platform.CodeValidation, false, "账号索引账号元数据不合法")
	}
	normalized, err := NormalizeAccountName(NormalizeAccountNameInput{AccountName: account.AccountName})
	if err != nil || normalized.Normalized != account.AccountName {
		return platform.NewError(platform.CodeIntegrityError, false, "账号索引账号名不合法")
	}
	if _, exists := ids[account.AccountID]; exists {
		return platform.NewError(platform.CodeIntegrityError, false, "账号索引包含重复账户")
	}
	if _, exists := names[account.AccountName]; exists {
		return platform.NewError(platform.CodeIntegrityError, false, "账号索引包含重复账号名")
	}
	ids[account.AccountID] = struct{}{}
	names[account.AccountName] = struct{}{}
	return nil
}

func accountIndexMatchesAuthority(index AccountIndex, authority accountAuthority) bool {
	if index.Revision != authority.revision || len(index.Accounts) != len(authority.accounts) {
		return false
	}
	expected, err := buildAccountIndex(WriteAccountIndexInput{Accounts: authority.accounts, Revision: authority.revision})
	if err != nil {
		return false
	}
	left, _ := json.Marshal(index)
	right, _ := json.Marshal(expected)
	return string(left) == string(right)
}

func readAccountIndexFile(dataRoot string) (AccountIndex, []byte, error) {
	controlDir, err := checkPrivateControlDir(dataRoot)
	if err != nil {
		return AccountIndex{}, nil, err
	}
	path := filepath.Join(controlDir, accountIndexFileName)
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AccountIndex{}, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引不存在")
		}
		return AccountIndex{}, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引不可检查")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return AccountIndex{}, nil, platform.NewError(platform.CodeIntegrityError, false, "账号索引文件权限或类型不合法")
	}
	file, err := os.Open(path)
	if err != nil {
		return AccountIndex{}, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引不可读取")
	}
	defer file.Close()
	limited := io.LimitReader(file, maxAccountIndexBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return AccountIndex{}, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引读取失败")
	}
	if len(raw) > maxAccountIndexBytes {
		return AccountIndex{}, nil, platform.NewError(platform.CodeIntegrityError, false, "账号索引过大")
	}
	if !utf8.Valid(raw) {
		return AccountIndex{}, nil, platform.NewError(platform.CodeIntegrityError, false, "账号索引不是有效UTF-8")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var index AccountIndex
	if err := decoder.Decode(&index); err != nil {
		return AccountIndex{}, nil, platform.NewError(platform.CodeIntegrityError, false, "账号索引格式不合法")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return AccountIndex{}, nil, platform.NewError(platform.CodeIntegrityError, false, "账号索引包含多余内容")
	}
	if err := validateAccountIndex(index); err != nil {
		return AccountIndex{}, nil, platform.NewError(platform.CodeIntegrityError, false, "账号索引内容不合法")
	}
	return index, raw, nil
}

func validateAccountIndex(index AccountIndex) error {
	if index.Revision < 0 || len(index.Accounts) < 1 || len(index.Accounts) > maxIndexedAccounts {
		return errors.New("账号索引范围不合法")
	}
	ids := map[string]struct{}{}
	names := map[string]struct{}{}
	superadminCount := 0
	for _, entry := range index.Accounts {
		if err := validateIndexAccount(Account{
			AccountID:    entry.AccountID,
			AccountName:  entry.AccountName,
			IsSuperadmin: entry.IsSuperadmin,
			Status:       entry.Status,
		}, ids, names); err != nil {
			return err
		}
		if entry.IsSuperadmin {
			superadminCount++
		}
	}
	if superadminCount != 1 {
		return errors.New("账号索引超管数量不合法")
	}
	return nil
}

func ensurePrivateControlDir(dataRoot string) (string, error) {
	if !filepath.IsAbs(dataRoot) {
		return "", platform.NewError(platform.CodeValidation, false, "账号索引数据根不合法")
	}
	controlDir := filepath.Join(filepath.Clean(dataRoot), "control")
	if err := os.MkdirAll(controlDir, 0o700); err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引控制目录不可建立")
	}
	info, err := os.Lstat(controlDir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return "", platform.NewError(platform.CodeIntegrityError, false, "账号索引控制目录不安全")
	}
	return controlDir, nil
}

func checkPrivateControlDir(dataRoot string) (string, error) {
	if !filepath.IsAbs(dataRoot) {
		return "", platform.NewError(platform.CodeValidation, false, "账号索引数据根不合法")
	}
	controlDir := filepath.Join(filepath.Clean(dataRoot), "control")
	info, err := os.Lstat(controlDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引控制目录不存在")
		}
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引控制目录不可检查")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return "", platform.NewError(platform.CodeIntegrityError, false, "账号索引控制目录不安全")
	}
	return controlDir, nil
}

// accountIndexFileMissing 判断固定索引文件（或其control目录）尚未建立。
func accountIndexFileMissing(dataRoot string) bool {
	_, err := os.Lstat(filepath.Join(filepath.Clean(dataRoot), "control", accountIndexFileName))
	return os.IsNotExist(err)
}

func isCode(err error, code string) bool {
	var platformErr *platform.Error
	return errors.As(err, &platformErr) && platformErr.Code == code
}

// encodeAccountIndex 由提交前检查与实际安装共用唯一编码和大小约束。
func encodeAccountIndex(input WriteAccountIndexInput) (AccountIndex, []byte, error) {
	index, err := buildAccountIndex(input)
	if err != nil {
		return AccountIndex{}, nil, err
	}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return AccountIndex{}, nil, platform.NewError(platform.CodeInternal, false, "账号索引不可编码")
	}
	raw = append(raw, '\n')
	if len(raw) > maxAccountIndexBytes {
		return AccountIndex{}, nil, platform.NewError(platform.CodeRateLimited, false, "账号索引达到容量上限")
	}
	return index, raw, nil
}

// ObserveSuperadminResetFileInput 是检测服务器超管重置文件的输入。
type ObserveSuperadminResetFileInput struct{}

// ServerResetCandidate 是固定重置文件的内存候选，不写入回执或日志。
type ServerResetCandidate struct {
	CandidateID platform.Identifier `json:"candidateId"`
	Password    Password            `json:"password"`
	FileHash    platform.Hash       `json:"fileHash"`
	ObservedAt  platform.Timestamp  `json:"observedAt"`
}

// ObserveSuperadminResetFileResult 是检测服务器超管重置文件的结果。
// 文件不存在时 candidate 为 null。
type ObserveSuperadminResetFileResult struct {
	Candidate *ServerResetCandidate `json:"candidate"`
}

// ApplySuperadminResetInput 是用服务器文件恢复唯一超管口令的输入。
type ApplySuperadminResetInput struct {
	Candidate ServerResetCandidate `json:"candidate"`
}

// ApplySuperadminResetResult 是用服务器文件恢复唯一超管口令的结果。
type ApplySuperadminResetResult struct {
	Completed bool `json:"completed"`
}

type superadminResetAccount struct {
	AccountID platform.Identifier
	SiteID    platform.Identifier
	Status    AccountStatus
}

type superadminResetDigestInput struct {
	FileHash platform.Hash `json:"fileHash"`
}

// ObserveSuperadminResetFile 读取固定重置文件。存在合法候选时交给
// ApplySuperadminReset；本函数不启动后台循环。
func ObserveSuperadminResetFile(ctx context.Context, op OperationContext, input ObserveSuperadminResetFileInput) (ObserveSuperadminResetFileResult, error) {
	superadminResetMu.Lock()
	defer superadminResetMu.Unlock()
	if err := checkSuperadminResetContext(ctx, op); err != nil {
		return ObserveSuperadminResetFileResult{}, err
	}
	store, err := superadminResetStore(ctx)
	if err != nil {
		return ObserveSuperadminResetFileResult{}, err
	}
	candidate, missing, err := readSuperadminResetFile(store.dataRoot)
	if err != nil || missing {
		return ObserveSuperadminResetFileResult{}, err
	}
	op.OperationID = candidate.CandidateID
	if _, err := applySuperadminResetLocked(ctx, op, ApplySuperadminResetInput{Candidate: *candidate}); err != nil {
		return ObserveSuperadminResetFileResult{}, err
	}
	return ObserveSuperadminResetFileResult{Candidate: candidate}, nil
}

// ApplySuperadminReset 用固定文件恢复当前唯一超管口令，并撤销其全部会话。
// operationId 必须是该文件的 fileHash；不接受后台令牌作为授权。
func ApplySuperadminReset(ctx context.Context, op OperationContext, input ApplySuperadminResetInput) (ApplySuperadminResetResult, error) {
	superadminResetMu.Lock()
	defer superadminResetMu.Unlock()
	return applySuperadminResetLocked(ctx, op, input)
}

func applySuperadminResetLocked(ctx context.Context, op OperationContext, input ApplySuperadminResetInput) (ApplySuperadminResetResult, error) {
	if err := checkSuperadminResetContext(ctx, op); err != nil {
		return ApplySuperadminResetResult{}, err
	}
	if !validSuperadminResetCandidate(input.Candidate) || op.OperationID != input.Candidate.CandidateID {
		return ApplySuperadminResetResult{}, platform.NewError(platform.CodeIntegrityError, false, "重置候选与文件哈希不一致")
	}
	store, err := superadminResetStore(ctx)
	if err != nil {
		return ApplySuperadminResetResult{}, err
	}
	current, missing, err := readSuperadminResetFile(store.dataRoot)
	if err != nil {
		if isCode(err, platform.CodeValidation) {
			return ApplySuperadminResetResult{}, platform.NewError(platform.CodeIntegrityError, false, "重置文件与候选不一致")
		}
		return ApplySuperadminResetResult{}, err
	}
	if !missing && !resetFileMatches(current, input.Candidate) {
		return ApplySuperadminResetResult{}, platform.NewError(platform.CodeIntegrityError, false, "重置文件与候选不一致")
	}
	account, found, err := lookupSuperadminReset(ctx, store, op, input.Candidate.FileHash)
	if err != nil {
		return ApplySuperadminResetResult{}, err
	}
	if account.SiteID != op.Actor.SiteID {
		return ApplySuperadminResetResult{}, platform.NewError(platform.CodeForbidden, false, "超管重置站点与可信scope不一致")
	}
	if found {
		return finishSuperadminResetFile(store.dataRoot, input.Candidate.FileHash, missing)
	}
	if missing {
		return ApplySuperadminResetResult{}, platform.NewError(platform.CodeIntegrityError, false, "重置文件缺失且没有原回执")
	}
	if err := ctx.Err(); err != nil {
		return ApplySuperadminResetResult{}, identityContextError(err)
	}
	resetOp := superadminResetOperation(op, account, input.Candidate.FileHash)
	digest, keyID, err := privateIdentityDigest(ctx, resetOp, "applySuperadminReset", superadminResetDigestInput{FileHash: input.Candidate.FileHash})
	if err != nil {
		return ApplySuperadminResetResult{}, err
	}
	hashed, err := HashPassword(ctx, resetOp, HashPasswordInput{Password: input.Candidate.Password})
	if err != nil {
		if isCode(err, platform.CodeValidation) {
			return ApplySuperadminResetResult{}, platform.NewError(platform.CodeIntegrityError, false, "重置口令与口令规则不一致")
		}
		return ApplySuperadminResetResult{}, err
	}
	alreadyDone, err := commitSuperadminReset(ctx, store, resetOp, account, hashed.EncodedHash, digest, keyID)
	if err != nil {
		return ApplySuperadminResetResult{}, err
	}
	if alreadyDone {
		return finishSuperadminResetFile(store.dataRoot, input.Candidate.FileHash, false)
	}
	if err := consumeSuperadminResetFile(store.dataRoot, input.Candidate.FileHash); err != nil {
		return ApplySuperadminResetResult{}, err
	}
	return ApplySuperadminResetResult{Completed: true}, nil
}

func finishSuperadminResetFile(dataRoot string, fileHash platform.Hash, missing bool) (ApplySuperadminResetResult, error) {
	if missing {
		return ApplySuperadminResetResult{Completed: true}, nil
	}
	if err := consumeSuperadminResetFile(dataRoot, fileHash); err != nil {
		return ApplySuperadminResetResult{}, err
	}
	return ApplySuperadminResetResult{Completed: true}, nil
}

func checkSuperadminResetContext(ctx context.Context, op OperationContext) error {
	if err := checkIdentityRepositoryContext(ctx, op); err != nil {
		return err
	}
	if op.CallerService != "identity" || op.Actor.Kind != "service" || !platform.ValidIdentifier(op.Actor.SiteID) {
		return platform.NewError(platform.CodeForbidden, false, "超管重置只允许本机身份进程")
	}
	return nil
}

func superadminResetStore(ctx context.Context) (*Store, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok || store.db == nil || store.dataRoot == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "超管重置存储尚未装配")
	}
	return store, nil
}

func validSuperadminResetCandidate(candidate ServerResetCandidate) bool {
	if !platform.ValidHash(candidate.FileHash) || candidate.CandidateID != candidate.FileHash || !validPassword(candidate.Password) {
		return false
	}
	_, err := platform.ParseTimestamp(candidate.ObservedAt)
	return err == nil
}

func resetFileMatches(file *ServerResetCandidate, given ServerResetCandidate) bool {
	if file == nil || file.FileHash != given.FileHash || file.CandidateID != given.CandidateID {
		return false
	}
	left := []byte(file.Password)
	right := []byte(given.Password)
	if len(left) != len(right) {
		clear(left)
		clear(right)
		return false
	}
	matched := subtle.ConstantTimeCompare(left, right) == 1
	clear(left)
	clear(right)
	return matched
}

func superadminResetOperation(op OperationContext, account superadminResetAccount, fileHash platform.Hash) OperationContext {
	op.CallerService = "identity"
	op.OperationID = fileHash
	op.Actor.Kind = "service"
	op.Actor.SiteID = account.SiteID
	op.Actor.SubjectID = account.AccountID
	op.Actor.SessionID = ""
	op.Actor.Grants = nil
	return op
}

func lookupSuperadminReset(ctx context.Context, store *Store, op OperationContext, fileHash platform.Hash) (superadminResetAccount, bool, error) {
	readTx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return superadminResetAccount{}, false, identityDBError()
	}
	defer readTx.Rollback()
	account, err := readUniqueSuperadmin(ctx, readTx)
	if err != nil {
		return superadminResetAccount{}, false, err
	}
	resetOp := superadminResetOperation(op, account, fileHash)
	found, err := readSuperadminResetReceipt(ctx, readTx, resetOp)
	if err != nil {
		return superadminResetAccount{}, false, err
	}
	return account, found, nil
}

// readSuperadminResetFile 只读取 identity 私有 control/reset-superadmin.txt。
// 权限异常沿用同一 control 区规则：拒绝符号链接、目录、硬链接和组/其他权限位。
// 合同未写死八进制模式，因此不另选具体文件权限。内容按 Password 校验且不 trim。
func readSuperadminResetFile(dataRoot string) (*ServerResetCandidate, bool, error) {
	path, err := superadminResetPath(dataRoot)
	if err != nil {
		return nil, false, err
	}
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", true); err != nil {
		var platformErr *platform.Error
		if errors.As(err, &platformErr) && platformErr.Message == "私有权威路径不可读取" {
			return nil, false, platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件不可检查")
		}
		return nil, false, platform.NewError(platform.CodeValidation, false, "重置文件类型或路径不合法")
	}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件不可检查")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, false, platform.NewError(platform.CodeValidation, false, "重置文件类型或权限不合法")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件不可读取")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxSuperadminResetBytes+1))
	if err != nil {
		return nil, false, platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件读取失败")
	}
	defer clear(raw)
	if len(raw) > maxSuperadminResetBytes || !utf8.Valid(raw) || !validPassword(string(raw)) {
		return nil, false, platform.NewError(platform.CodeValidation, false, "重置文件不是合法口令")
	}
	fileHash := platform.HashBytes(raw)
	observedAt := info.ModTime().UTC().Format(time.RFC3339Nano)
	return &ServerResetCandidate{
		CandidateID: fileHash,
		Password:    string(raw),
		FileHash:    fileHash,
		ObservedAt:  observedAt,
	}, false, nil
}

func superadminResetPath(dataRoot string) (string, error) {
	dir, err := resetControlDir(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, superadminResetFileName), nil
}

func resetControlDir(dataRoot string) (string, error) {
	dir, err := checkPrivateControlDir(dataRoot)
	if err == nil {
		return dir, nil
	}
	if isCode(err, platform.CodeDependencyUnavailable) {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件控制目录不可用")
	}
	return "", platform.NewError(platform.CodeValidation, false, "重置文件控制目录不安全")
}

// readUniqueSuperadmin 以账户权威表为准，不读取账号索引。
func readUniqueSuperadmin(ctx context.Context, query sessionQueryer) (superadminResetAccount, error) {
	rows, err := query.QueryContext(ctx, `
		SELECT account_id, site_id, status
		FROM accounts WHERE is_superadmin = 1`)
	if err != nil {
		return superadminResetAccount{}, identityDBError()
	}
	defer rows.Close()
	var found superadminResetAccount
	count := 0
	for rows.Next() {
		count++
		if count > 1 {
			return superadminResetAccount{}, platform.NewError(platform.CodeIntegrityError, false, "唯一超管权威不唯一")
		}
		if err := rows.Scan(&found.AccountID, &found.SiteID, &found.Status); err != nil {
			return superadminResetAccount{}, platform.NewError(platform.CodeIntegrityError, false, "唯一超管权威记录损坏")
		}
	}
	if err := rows.Err(); err != nil {
		return superadminResetAccount{}, identityDBError()
	}
	if count == 0 {
		return superadminResetAccount{}, platform.NewError(platform.CodeIntegrityError, false, "缺少唯一超管权威")
	}
	if found.Status != AccountStatusActive || !platform.ValidIdentifier(found.AccountID) || !platform.ValidIdentifier(found.SiteID) {
		return superadminResetAccount{}, platform.NewError(platform.CodeIntegrityError, false, "唯一超管权威不合法")
	}
	return found, nil
}

func readSuperadminResetReceipt(ctx context.Context, query sessionQueryer, op OperationContext) (bool, error) {
	var resultHash, state string
	var raw sql.NullString
	err := query.QueryRowContext(ctx, `
		SELECT result_hash, state, result_json
		FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND operation_id = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, op.OperationID).Scan(&resultHash, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, identityDBError()
	}
	if state != "committed" || !raw.Valid || platform.HashBytes([]byte(raw.String)) != resultHash {
		return false, platform.NewError(platform.CodeIntegrityError, false, "超管重置回执不一致")
	}
	var result ApplySuperadminResetResult
	if json.Unmarshal([]byte(raw.String), &result) != nil || !result.Completed {
		return false, platform.NewError(platform.CodeIntegrityError, false, "超管重置回执不一致")
	}
	return true, nil
}

func commitSuperadminReset(ctx context.Context, store *Store, op OperationContext, account superadminResetAccount, encodedHash, digest, keyID string) (bool, error) {
	store.accountMu.Lock()
	defer store.accountMu.Unlock()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return false, identityDBError()
	}
	defer tx.Rollback()
	current, err := readUniqueSuperadmin(ctx, tx)
	if err != nil {
		return false, err
	}
	if current.AccountID != account.AccountID || current.SiteID != account.SiteID {
		return false, platform.NewError(platform.CodeIntegrityError, false, "唯一超管权威已变化")
	}
	found, err := readSuperadminResetReceipt(ctx, tx, op)
	if err != nil {
		return false, err
	}
	if found {
		return true, nil
	}
	var revision platform.Revision
	err = tx.QueryRowContext(ctx, `SELECT credential_revision FROM credentials WHERE account_id = ?`, account.AccountID).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return false, platform.NewError(platform.CodeIntegrityError, false, "超管凭据权威缺失")
	}
	if err != nil {
		return false, identityDBError()
	}
	changed, err := tx.ExecContext(ctx, `
		UPDATE credentials
		SET password_hash = ?, credential_revision = credential_revision + 1
		WHERE account_id = ? AND credential_revision = ?`, encodedHash, account.AccountID, revision)
	if err != nil {
		return false, identityDBError()
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "超管凭据版本已变化")
	}
	// 索引不含强改标记。不提升账户 revision，避免一次口令恢复改写账号索引。
	changed, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET must_change_password = 1, updated_at = ?
		WHERE account_id = ? AND site_id = ? AND is_superadmin = 1 AND status = ?`,
		platform.Now(), account.AccountID, account.SiteID, AccountStatusActive)
	if err != nil {
		return false, identityDBError()
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return false, platform.NewError(platform.CodeIntegrityError, false, "唯一超管权威无法更新")
	}
	if _, err := RevokeAllSessions(WithIdentityTransaction(ctx, tx), op, RevokeAllSessionsInput{AccountID: account.AccountID, Reason: "server_reset"}); err != nil {
		return false, err
	}
	if err := saveIdentityReceipt(ctx, op, tx, digest, keyID, ApplySuperadminResetResult{Completed: true}); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, identityDBError()
	}
	return false, nil
}

// consumeSuperadminResetFile 在回执提交后只删除哈希仍匹配的固定文件。
// 文件已消失视为消费完成；哈希或类型变化时保留文件，避免删掉新的重置请求。
func consumeSuperadminResetFile(dataRoot string, fileHash platform.Hash) error {
	candidate, missing, err := readSuperadminResetFile(dataRoot)
	if missing {
		return nil
	}
	if err != nil {
		if isCode(err, platform.CodeValidation) {
			return platform.NewError(platform.CodeIntegrityError, false, "重置文件不能按原哈希删除")
		}
		return err
	}
	if candidate.FileHash != fileHash {
		return platform.NewError(platform.CodeIntegrityError, false, "重置文件哈希已变化")
	}
	path, err := superadminResetPath(dataRoot)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件不可检查")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "重置文件类型或权限不合法")
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return platform.NewError(platform.CodeDependencyUnavailable, true, "重置文件删除失败")
	}
	return nil
}

// NewBackgroundTasks 只返回具有真实运行权威的后台入口；缺任一依赖时不登记后台任务。
func NewBackgroundTasks(store *Store, siteConfig platform.SiteConfigClient, dataEpoch platform.Identifier) ([]platform.BackgroundTask, error) {
	if store == nil || store.db == nil || siteConfig == nil || !platform.ValidIdentifier(dataEpoch) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "身份后台任务真实运行依赖尚未装配")
	}
	return []platform.BackgroundTask{
		{
			Name: "identity.superadmin-reset",
			Run: func(ctx context.Context) error {
				return runSuperadminResetObserver(ctx, store, siteConfig, dataEpoch)
			},
		},
		{
			Name: "identity.site-purge",
			Run: func(ctx context.Context) error {
				return runSitePurge(ctx, store, siteConfig, dataEpoch)
			},
		},
	}, nil
}

// runSuperadminResetObserver 先用只读发现取得唯一超管所属站点，再把真实
// identity 操作放入公共恢复循环；业务文件读取和凭据写入只在准入与写窗口同时允许时执行。
func runSuperadminResetObserver(ctx context.Context, store *Store, siteConfig platform.SiteConfigClient, dataEpoch platform.Identifier) error {
	siteID, err := waitForIdentitySite(ctx, store, superadminResetInterval)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	op := identityBackgroundOperation(siteID, dataEpoch, time.Now().UTC().Add(time.Minute))
	return lifecycle.RunRecoveryLoop(platform.WithOperationContext(ctx, op), platform.RecoveryLoopConfig{
		AdmitBatch:  lifecycle.SimpleAdmission(siteConfig.ReadRuntimeState, dataEpoch),
		PollMillis:  int(superadminResetInterval / time.Millisecond),
		Runtime:     siteConfig.ReadRuntimeState,
		WriteWindow: lifecycle.SimpleWriteWindow(siteConfig.ReadRuntimeState),
		Recover: func(work context.Context) error {
			op, ok := platform.OperationContextFrom(work)
			if !ok {
				return platform.NewError(platform.CodeDependencyUnavailable, true, "超管重置后台缺少受信操作上下文")
			}
			work = WithIdentityStore(work, store)
			_, err := ObserveSuperadminResetFile(work, op, ObserveSuperadminResetFileInput{})
			return err
		},
	})
}

// waitForIdentitySite 只做本域权威读取，不触碰reset文件或产生业务写入。
// 超管出现后其站点身份不可变，因此后续真实后台循环可固定绑定该scope。
func waitForIdentitySite(ctx context.Context, store *Store, interval time.Duration) (platform.Identifier, error) {
	for {
		var count int
		if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE is_superadmin = 1`).Scan(&count); err != nil {
			return "", identityDBError()
		}
		switch count {
		case 0:
		case 1:
			account, err := readUniqueSuperadmin(ctx, store.db)
			if err != nil {
				return "", err
			}
			return account.SiteID, nil
		default:
			return "", platform.NewError(platform.CodeIntegrityError, false, "唯一超管权威不唯一")
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", nil
		case <-timer.C:
		}
	}
}

func identityBackgroundOperation(siteID, dataEpoch platform.Identifier, deadline time.Time) platform.OperationContext {
	operationID := platform.NewIdentifier()
	deadlineString := deadline.UTC().Format(time.RFC3339Nano)
	return platform.OperationContext{
		RequestID:       operationID,
		OperationID:     operationID,
		RootOperationID: operationID,
		CallerService:   "identity",
		Actor: platform.AuthContext{
			SubjectID: "identity",
			Kind:      "service",
			SiteID:    siteID,
			Grants:    []string{},
			ExpiresAt: deadlineString,
		},
		Deadline:  deadlineString,
		DataEpoch: dataEpoch,
	}
}
