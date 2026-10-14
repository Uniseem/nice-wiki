// Package identity 提供服务器本机恢复所需的账号索引读写边界。
//
// 账号索引只保留可供人工查找的账号元数据，不复制 credential、session
// 或任何口令相关字段。文件位于 control 区，control 区不随业务数据恢复替换。
package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
)

const (
	accountIndexFileName = "account-index.json"
	maxAccountIndexBytes = 4 << 20
	maxIndexedAccounts   = 10000
)

type serverAccountIndexContextKey struct{}

// serverAccountIndexContext 是本机运维调用的受信存储边界。数据根和活动
// 代次由进程装配层提供，不能由账号索引输入或浏览器请求自声明。
type serverAccountIndexContext struct {
	dataRoot    string
	dataEpoch   platform.Identifier
	phase       string
	maintenance bool
}

// WithServerAccountIndexContext 为本机账号索引操作装入固定数据根和运行代次。
// 这是启动入口与后续身份业务装配之间的内部边界，不是HTTP输入。
func WithServerAccountIndexContext(ctx context.Context, dataRoot string, dataEpoch platform.Identifier, phase string, maintenance bool) context.Context {
	return context.WithValue(ctx, serverAccountIndexContextKey{}, serverAccountIndexContext{
		dataRoot:    dataRoot,
		dataEpoch:   dataEpoch,
		phase:       phase,
		maintenance: maintenance,
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
	Revision platform.Revision       `json:"revision"`
	Accounts []AccountIndexEntry     `json:"accounts"`
}

// AccountIndexEntry 是供服务器人工查找的最小账号记录。
type AccountIndexEntry struct {
	AccountID    platform.Identifier `json:"accountId"`
	AccountName  AccountName         `json:"accountName"`
	IsSuperadmin bool                 `json:"isSuperadmin"`
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
	if err != nil {
		return ReadServerAccountIndexResult{}, err
	}
	authority, err := readAccountAuthority(ctx, state.dataRoot)
	if err != nil {
		return ReadServerAccountIndexResult{}, err
	}
	if accountIndexMatchesAuthority(index, authority) {
		return ReadServerAccountIndexResult{Index: index}, nil
	}

	// 只在索引本身结构完整、但水位落后于权威表时重建；损坏索引在
	// 上面的读取阶段已经返回INTEGRITY_ERROR，不把未知状态当作允许。
	if index.Revision > authority.revision {
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
	state, err := accountIndexContext(ctx, op, true)
	if err != nil {
		return WriteAccountIndexResult{}, err
	}
	index, err := buildAccountIndex(input)
	if err != nil {
		return WriteAccountIndexResult{}, err
	}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return WriteAccountIndexResult{}, platform.NewError(platform.CodeInternal, false, "账号索引不可编码")
	}
	encoded = append(encoded, '\n')

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
	rows, err := db.QueryContext(ctx, `SELECT account_id, account_name, is_superadmin, status, revision FROM accounts ORDER BY account_id`)
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
		if account.Revision > result.revision {
			result.revision = account.Revision
		}
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
	if !ok || !filepath.IsAbs(state.dataRoot) || !platform.ValidIdentifier(state.dataEpoch) || state.phase == "" {
		return serverAccountIndexContext{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账号索引运行权威未装配")
	}
	if state.dataEpoch != op.DataEpoch {
		return serverAccountIndexContext{}, platform.NewError(platform.CodeVersionMismatch, true, "账号索引数据代次不一致")
	}
	if writing && (state.maintenance || state.phase != platform.PhaseRunning) {
		return serverAccountIndexContext{}, platform.NewError(platform.CodeLocked, true, "恢复或维护期间禁止写账号索引")
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

func isCode(err error, code string) bool {
	var platformErr *platform.Error
	return errors.As(err, &platformErr) && platformErr.Code == code
}
