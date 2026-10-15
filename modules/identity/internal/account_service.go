package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/serviceauth"
)

type CreateAccountInput struct {
	AccountName     AccountName         `json:"accountName"`
	InitialPassword Password            `json:"initialPassword"`
	Permissions     PermissionSelection `json:"permissions"`
}
type CreateAccountResult struct {
	Account Account `json:"account"`
}
type ReadSelfInput struct{}
type ReadSelfResult struct {
	Account            Account           `json:"account"`
	CredentialRevision platform.Revision `json:"credentialRevision"`
}

// CreateAccount 在权威事务内创建必须改密的子账号；索引由决定日志补齐。
func CreateAccount(ctx context.Context, op OperationContext, input CreateAccountInput) (CreateAccountResult, error) {
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "superadmin") {
		return CreateAccountResult{}, platform.NewError(platform.CodeForbidden, false, "仅超管可以创建账户")
	}
	normalized, err := NormalizeAccountName(NormalizeAccountNameInput{AccountName: input.AccountName})
	if err != nil {
		return CreateAccountResult{}, err
	}
	if _, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: input.Permissions}); err != nil {
		return CreateAccountResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return CreateAccountResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "createAccount", input)
	if err != nil {
		return CreateAccountResult{}, err
	}
	encoded, err := HashPassword(ctx, op, HashPasswordInput{Password: input.InitialPassword})
	if err != nil {
		return CreateAccountResult{}, err
	}
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return CreateAccountResult{}, err
	}
	store.accountMu.Lock()
	defer store.accountMu.Unlock()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return CreateAccountResult{}, identityDBError()
	}
	defer tx.Rollback()
	var result CreateAccountResult
	found, err := readIdentityReceipt(ctx, op, tx, digest, keyID, &result)
	if err != nil {
		return CreateAccountResult{}, err
	}
	if found {
		if err := tx.Rollback(); err != nil {
			return CreateAccountResult{}, identityDBError()
		}
		if err := syncAccountIndex(ctx, op, store); err != nil {
			return CreateAccountResult{}, err
		}
		return result, nil
	}
	if _, _, err := readActorAccount(ctx, op, tx, "superadmin", "identity:createAccount"); err != nil {
		return CreateAccountResult{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return CreateAccountResult{}, identityDBError()
	}
	if count >= maxIndexedAccounts {
		return CreateAccountResult{}, platform.NewError(platform.CodeRateLimited, false, "账户数量已达上限")
	}
	now := platform.Now()
	account := Account{AccountID: platform.NewIdentifier(), SiteID: op.Actor.SiteID, AccountName: normalized.Normalized,
		Permissions: selectedPermissionNames(input.Permissions), MustChangePassword: true, Status: AccountStatusActive,
		Revision: 1, PermissionRevision: 1, CreatedAt: now, UpdatedAt: now}
	if err := insertAccount(ctx, tx, account, encoded.EncodedHash); err != nil {
		return CreateAccountResult{}, err
	}
	if _, err := PublishIdentityEvent(WithIdentityTransaction(ctx, tx), op, PublishIdentityEventInput{
		EventID: platform.NewIdentifier(), Event: IdentityChanged{AccountID: account.AccountID, SiteID: account.SiteID,
			PermissionRevision: 1, CredentialRevision: 1, Reason: "permissions", OccurredAt: now}}); err != nil {
		return CreateAccountResult{}, err
	}
	result.Account = account
	if err := saveIdentityReceipt(ctx, op, tx, digest, keyID, result); err != nil {
		return CreateAccountResult{}, err
	}
	if err := commitAccountWrite(ctx, op, store, tx); err != nil {
		return CreateAccountResult{}, err
	}
	return result, nil
}

// ReadSelf 允许受限口令读取必要账户资料；当前会话权威失效即拒绝。
func ReadSelf(ctx context.Context, op OperationContext, input ReadSelfInput) (ReadSelfResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return ReadSelfResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadSelfResult{}, identityDBError()
	}
	defer tx.Rollback()
	snapshot, account, err := readActorAccount(ctx, op, tx, "account.self", "identity:readSelf")
	if err != nil {
		return ReadSelfResult{}, err
	}
	return ReadSelfResult{Account: account, CredentialRevision: snapshot.CurrentCredentialRevision}, nil
}

func readAccount(ctx context.Context, query sessionQueryer, accountID, siteID string) (Account, error) {
	var account Account
	err := query.QueryRowContext(ctx, `SELECT account_id,site_id,account_name,is_superadmin,must_change_password,status,
        revision,permission_revision,created_at,updated_at FROM accounts WHERE account_id=? AND site_id=?`, accountID, siteID).
		Scan(&account.AccountID, &account.SiteID, &account.AccountName, &account.IsSuperadmin, &account.MustChangePassword, &account.Status,
			&account.Revision, &account.PermissionRevision, &account.CreatedAt, &account.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, platform.NewError(platform.CodeNotFound, false, "账户不存在")
	}
	if err != nil {
		return Account{}, identityDBError()
	}
	rows, err := query.QueryContext(ctx, `SELECT permission FROM account_permissions WHERE account_id=? ORDER BY permission`, accountID)
	if err != nil {
		return Account{}, identityDBError()
	}
	account.Permissions = []string{}
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			rows.Close()
			return Account{}, identityDBError()
		}
		account.Permissions = append(account.Permissions, permission)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Account{}, identityDBError()
	}
	return account, nil
}

func readActorAccount(ctx context.Context, op OperationContext, query sessionQueryer, permission, action string) (sessionSnapshot, Account, error) {
	if op.Actor.Kind != "admin" {
		return sessionSnapshot{}, Account{}, platform.NewError(platform.CodeUnauthenticated, false, "需要账户会话")
	}
	snapshot, err := readSessionSnapshot(ctx, query, op.Actor.SessionID)
	if err != nil {
		return sessionSnapshot{}, Account{}, err
	}
	if snapshot.Session.AccountID != op.Actor.SubjectID || snapshot.Session.SiteID != op.Actor.SiteID || snapshot.Status != "active" ||
		op.Actor.SessionRevision <= 0 || op.Actor.SessionRevision != snapshot.Session.Revision ||
		op.Actor.PermissionRevision != snapshot.CurrentPermissionRevision {
		return sessionSnapshot{}, Account{}, platform.NewError(platform.CodeUnauthenticated, false, "当前账户会话已失效")
	}
	if err := validateSessionSnapshot(snapshot, op.DataEpoch, true); err != nil {
		return sessionSnapshot{}, Account{}, err
	}
	if snapshot.MustChangePassword && (permission != "account.self" ||
		(action != "identity:changePassword" && action != "identity:readSelf" && action != "identity:revokeSession")) {
		return sessionSnapshot{}, Account{}, platform.NewError(platform.CodeForbidden, false, "必须先修改初始口令")
	}
	auth, err := sessionAuthContext(ctx, query, snapshot)
	if err != nil {
		return sessionSnapshot{}, Account{}, err
	}
	if !hasGrant(auth.Grants, permission) {
		return sessionSnapshot{}, Account{}, platform.NewError(platform.CodeForbidden, false, "账户没有该操作权限")
	}
	account, err := readAccount(ctx, query, op.Actor.SubjectID, op.Actor.SiteID)
	return snapshot, account, err
}

func insertAccount(ctx context.Context, tx *sql.Tx, account Account, encodedHash string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO accounts(account_id,site_id,account_name,is_superadmin,must_change_password,status,revision,permission_revision,created_at,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?)`, account.AccountID, account.SiteID, account.AccountName, account.IsSuperadmin, account.MustChangePassword,
		account.Status, account.Revision, account.PermissionRevision, account.CreatedAt, account.UpdatedAt)
	if err != nil {
		if stringsContains(err.Error(), "UNIQUE constraint failed") {
			return platform.NewError(platform.CodeConflict, false, "账号名或唯一超管已被占用")
		}
		return identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credentials(account_id,password_hash,credential_revision) VALUES(?,?,1)`, account.AccountID, encodedHash); err != nil {
		return identityDBError()
	}
	for _, permission := range account.Permissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_permissions(account_id,permission) VALUES(?,?)`, account.AccountID, permission); err != nil {
			return identityDBError()
		}
	}
	return nil
}

func selectedPermissionNames(input PermissionSelection) []string {
	if input == nil {
		input = allFunctionPermissions
	}
	result := make([]string, 0, len(input))
	for _, permission := range input {
		result = append(result, string(permission))
	}
	return result
}

func privateIdentityDigest(ctx context.Context, op OperationContext, action string, input any) (platform.Hash, string, error) {
	runtime, err := sessionRuntimeFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	raw, err := json.Marshal(struct {
		Caller string `json:"caller"`
		Epoch  string `json:"dataEpoch"`
		Input  any    `json:"input"`
	}{op.CallerService, op.DataEpoch, input})
	if err != nil {
		return "", "", platform.NewError(platform.CodeInternal, false, "私有操作摘要不可编码")
	}
	digest, err := serviceauth.PrivateOperationDigest(runtime.Key, "identity.operation.v1/"+action, raw)
	clear(raw)
	return digest, runtime.Key.KeyID, err
}

func readIdentityReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, digest, keyID string, result any) (bool, error) {
	var previousDigest, previousKey, resultHash, state string
	var raw sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT input_digest,digest_key_id,result_hash,state,result_json FROM operation_receipts
        WHERE site_id=? AND actor_id=? AND operation_id=?`, op.Actor.SiteID, op.Actor.SubjectID, op.OperationID).
		Scan(&previousDigest, &previousKey, &resultHash, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, identityDBError()
	}
	if previousKey != keyID {
		return false, platform.NewError(platform.CodeConflict, false, "私有回执摘要需要原密钥版本复核")
	}
	if previousDigest != digest {
		return false, platform.NewError(platform.CodeConflict, false, "幂等操作对应不同输入")
	}
	if state != "committed" || !raw.Valid || platform.HashBytes([]byte(raw.String)) != resultHash || json.Unmarshal([]byte(raw.String), result) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "私有操作安全回执不完整")
	}
	return true, nil
}

func saveIdentityReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, digest, keyID string, safeResult any) error {
	raw, err := json.Marshal(safeResult)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "私有操作安全回执不可编码")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipts(operation_id,site_id,actor_id,input_digest,digest_key_id,result_hash,state,created_at,result_json)
        VALUES(?,?,?,?,?,?,'committed',?,?)`, op.OperationID, op.Actor.SiteID, op.Actor.SubjectID, digest, keyID, platform.HashBytes(raw), platform.Now(), string(raw))
	if err != nil {
		return identityDBError()
	}
	return nil
}

type accountIndexDecision struct {
	DataEpoch string `json:"dataEpoch"`
}

func commitAccountWrite(ctx context.Context, op OperationContext, store *Store, tx *sql.Tx) error {
	authority, err := readAccountAuthorityQuery(ctx, tx)
	if err != nil {
		return err
	}
	if _, _, err := encodeAccountIndex(WriteAccountIndexInput{Accounts: authority.accounts, Revision: authority.revision}); err != nil {
		return err
	}
	id, err := store.AppendDecision("identity.account-index", accountIndexDecision{DataEpoch: op.DataEpoch})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return identityDBError()
	}
	if err := syncAccountIndex(ctx, op, store); err != nil {
		return err
	}
	return store.CompleteDecision(id)
}

func syncAccountIndex(ctx context.Context, op OperationContext, store *Store) error {
	authority, err := readAccountAuthority(ctx, store.dataRoot)
	if err != nil {
		return err
	}
	indexCtx := WithServerAccountIndexContext(ctx, store.dataRoot, op.DataEpoch, platform.PhaseRunning, false)
	result, err := WriteAccountIndex(indexCtx, op, WriteAccountIndexInput{Accounts: authority.accounts, Revision: authority.revision})
	if err != nil {
		return err
	}
	if result.Index.Revision != authority.revision {
		return platform.NewError(platform.CodeIntegrityError, false, "账号索引领先活动权威，须恢复流程处理")
	}
	return nil
}

func recoverAccountIndex(store *Store, entry journalEntry) error {
	var decision accountIndexDecision
	if json.Unmarshal(entry.Payload, &decision) != nil || !platform.ValidIdentifier(decision.DataEpoch) {
		return platform.NewError(platform.CodeIntegrityError, false, "账号索引决定损坏")
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return identityDBError()
	}
	if count != 0 {
		ctx := context.Background()
		op := OperationContext{CallerService: "identity", OperationID: platform.NewIdentifier(), DataEpoch: decision.DataEpoch,
			Deadline: time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)}
		if err := syncAccountIndex(ctx, op, store); err != nil {
			return err
		}
	} else if !accountIndexFileMissing(store.dataRoot) {
		return platform.NewError(platform.CodeIntegrityError, false, "空账户权威存在控制索引，禁止重新开户")
	}
	return store.CompleteDecision(entry.ID)
}

func identityDBError() error {
	return platform.NewError(platform.CodeDependencyUnavailable, true, "身份私有数据库操作失败")
}

func hasGrant(grants []string, wanted string) bool {
	for _, grant := range grants {
		if grant == wanted {
			return true
		}
	}
	return false
}

type ListAccountsInput struct {
	Page SessionPageRequest `json:"page"`
}
type ListAccountsResult struct {
	Accounts []Account       `json:"accounts"`
	PageInfo SessionPageInfo `json:"pageInfo"`
}
type accountPageCursor struct {
	AccountName string `json:"accountName"`
	AccountID   string `json:"accountId"`
}

type SetAccountStatusInput struct {
	AccountID        platform.Identifier `json:"accountId"`
	Status           AccountStatus       `json:"status"`
	ExpectedRevision platform.Revision   `json:"expectedRevision"`
}
type SetAccountStatusResult struct {
	Account Account `json:"account"`
}
type SetAccountPermissionsInput struct {
	AccountID        platform.Identifier `json:"accountId"`
	Permissions      PermissionSelection `json:"permissions"`
	ExpectedRevision platform.Revision   `json:"expectedRevision"`
}
type SetAccountPermissionsResult struct {
	Account Account `json:"account"`
}
type ResetAccountPasswordInput struct {
	AccountID        platform.Identifier `json:"accountId"`
	InitialPassword  Password            `json:"initialPassword"`
	ExpectedRevision platform.Revision   `json:"expectedRevision"`
}
type ResetAccountPasswordResult struct {
	Account Account `json:"account"`
}

// ListAccounts 在同一授权快照中分页列出子账号；墓碑保留，凭据不进入结果。
func ListAccounts(ctx context.Context, op OperationContext, input ListAccountsInput) (ListAccountsResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return ListAccountsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ListAccountsResult{}, identityDBError()
	}
	defer tx.Rollback()
	if err := requireAccountManagement(ctx, op, tx, "identity:listAccounts"); err != nil {
		return ListAccountsResult{}, err
	}
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	scope := "identity.accounts.v1|" + op.Actor.SiteID + "|" + op.Actor.SubjectID + "|" + op.DataEpoch + "|subaccounts"
	position := accountPageCursor{}
	key, signed := ctx.Value(sessionCursorKey{}).(platform.SigningKey)
	if input.Page.Cursor != "" {
		if !signed {
			return ListAccountsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户游标签名器尚未装配")
		}
		raw, err := serviceauth.VerifyCursor(key, scope, input.Page.Cursor)
		if err != nil {
			return ListAccountsResult{}, err
		}
		if json.Unmarshal(raw, &position) != nil {
			return ListAccountsResult{}, platform.NewError(platform.CodeValidation, false, "账户游标载荷不合法")
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT account_id FROM accounts WHERE site_id=? AND is_superadmin=0
        AND (account_name>? OR (account_name=? AND account_id>?)) ORDER BY account_name,account_id LIMIT ?`,
		op.Actor.SiteID, position.AccountName, position.AccountName, position.AccountID, limit+1)
	if err != nil {
		return ListAccountsResult{}, identityDBError()
	}
	ids := make([]string, 0, limit+1)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return ListAccountsResult{}, identityDBError()
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ListAccountsResult{}, identityDBError()
	}
	result := ListAccountsResult{Accounts: make([]Account, 0, limit)}
	if len(ids) > limit {
		result.PageInfo.HasMore = true
		ids = ids[:limit]
	}
	for _, id := range ids {
		account, err := readAccount(ctx, tx, id, op.Actor.SiteID)
		if err != nil {
			return ListAccountsResult{}, err
		}
		result.Accounts = append(result.Accounts, account)
	}
	if result.PageInfo.HasMore {
		if !signed {
			return ListAccountsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户游标签名器尚未装配")
		}
		last := result.Accounts[len(result.Accounts)-1]
		raw, _ := json.Marshal(accountPageCursor{AccountName: last.AccountName, AccountID: last.AccountID})
		result.PageInfo.NextCursor, err = serviceauth.SignCursor(key, scope, raw)
		if err != nil {
			return ListAccountsResult{}, err
		}
	}
	return result, nil
}

type accountManagementOperation struct {
	store  *Store
	tx     *sql.Tx
	digest string
	keyID  string
}

// beginAccountManagement 为管理写入建立同一权威事务；超管标志取账户row。
func beginAccountManagement(ctx context.Context, op OperationContext, store *Store, digest, keyID, action string) (*accountManagementOperation, error) {
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return nil, err
	}
	store.accountMu.Lock()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		store.accountMu.Unlock()
		return nil, identityDBError()
	}
	work := &accountManagementOperation{store: store, tx: tx, digest: digest, keyID: keyID}
	if err := requireAccountManagement(ctx, op, tx, action); err != nil {
		work.closeAccountManagement()
		return nil, err
	}
	return work, nil
}

func (work *accountManagementOperation) closeAccountManagement() {
	_ = work.tx.Rollback()
	work.store.accountMu.Unlock()
}

func requireAccountManagement(ctx context.Context, op OperationContext, query sessionQueryer, action string) error {
	_, actor, err := readActorAccount(ctx, op, query, "account.manage", action)
	if err != nil {
		return err
	}
	if !actor.IsSuperadmin {
		return platform.NewError(platform.CodeForbidden, false, "账户管理仅允许唯一超管")
	}
	return nil
}

// replayAccountManagement 只回放安全原结果，派生索引从当前权威重建，不重做旧业务。
func (work *accountManagementOperation) replayAccountManagement(ctx context.Context, op OperationContext, result any) (bool, error) {
	found, err := readIdentityReceipt(ctx, op, work.tx, work.digest, work.keyID, result)
	if err != nil || !found {
		return found, err
	}
	if err := work.tx.Rollback(); err != nil {
		return false, identityDBError()
	}
	if err := syncAccountIndex(ctx, op, work.store); err != nil {
		return false, err
	}
	return true, nil
}

func (work *accountManagementOperation) commitAccountManagement(ctx context.Context, op OperationContext, result any, accountChanged bool) error {
	if err := saveIdentityReceipt(ctx, op, work.tx, work.digest, work.keyID, result); err != nil {
		return err
	}
	if accountChanged {
		return commitAccountWrite(ctx, op, work.store, work.tx)
	}
	if err := work.tx.Commit(); err != nil {
		return identityDBError()
	}
	return nil
}

func readManagedAccount(ctx context.Context, op OperationContext, query sessionQueryer, accountID string) (Account, error) {
	account, err := readAccount(ctx, query, accountID, op.Actor.SiteID)
	if err != nil {
		return Account{}, err
	}
	if account.Status == AccountStatusDeleted {
		return Account{}, platform.NewError(platform.CodeForbidden, false, "已删除账户仅保留署名，不能再次管理")
	}
	return account, nil
}

// SetAccountStatus 原子改变子账号状态并撤销所有设备；恢复启用不复活旧会话。
func SetAccountStatus(ctx context.Context, op OperationContext, input SetAccountStatusInput) (SetAccountStatusResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return SetAccountStatusResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "setAccountStatus", input)
	if err != nil {
		return SetAccountStatusResult{}, err
	}
	work, err := beginAccountManagement(ctx, op, store, digest, keyID, "identity:setAccountStatus")
	if err != nil {
		return SetAccountStatusResult{}, err
	}
	defer work.closeAccountManagement()
	var result SetAccountStatusResult
	found, err := work.replayAccountManagement(ctx, op, &result)
	if err != nil {
		return SetAccountStatusResult{}, err
	}
	if found {
		return result, nil
	}
	account, err := readManagedAccount(ctx, op, work.tx, input.AccountID)
	if err != nil {
		return SetAccountStatusResult{}, err
	}
	if _, err := AssertSuperadminImmutable(AssertSuperadminImmutableInput{Account: account, RequestedStatus: input.Status}); err != nil {
		return SetAccountStatusResult{}, err
	}
	if account.Revision != input.ExpectedRevision {
		return SetAccountStatusResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	if account.Status == input.Status {
		result.Account = account
		if err := work.commitAccountManagement(ctx, op, result, false); err != nil {
			return SetAccountStatusResult{}, err
		}
		return result, nil
	}
	now := platform.Now()
	changed, err := work.tx.ExecContext(ctx, `UPDATE accounts SET status=?,revision=revision+1,permission_revision=permission_revision+1,updated_at=?
        WHERE account_id=? AND site_id=? AND revision=?`, input.Status, now, account.AccountID, account.SiteID, input.ExpectedRevision)
	if err != nil {
		return SetAccountStatusResult{}, identityDBError()
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return SetAccountStatusResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	account.Status = input.Status
	account.Revision++
	account.PermissionRevision++
	account.UpdatedAt = now
	if _, err := RevokeAllSessions(WithIdentityTransaction(ctx, work.tx), op, RevokeAllSessionsInput{AccountID: account.AccountID, Reason: "status"}); err != nil {
		return SetAccountStatusResult{}, err
	}
	result.Account = account
	if err := work.commitAccountManagement(ctx, op, result, true); err != nil {
		return SetAccountStatusResult{}, err
	}
	return result, nil
}

// SetAccountPermissions 只改变既有功能选择；旧access/委托立即失效，refresh可升级。
func SetAccountPermissions(ctx context.Context, op OperationContext, input SetAccountPermissionsInput) (SetAccountPermissionsResult, error) {
	if _, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: input.Permissions}); err != nil {
		return SetAccountPermissionsResult{}, err
	}
	names := selectedPermissionNames(input.Permissions)
	sort.Strings(names)
	input.Permissions = toFunctionPermissions(names)
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return SetAccountPermissionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "setAccountPermissions", input)
	if err != nil {
		return SetAccountPermissionsResult{}, err
	}
	work, err := beginAccountManagement(ctx, op, store, digest, keyID, "identity:setAccountPermissions")
	if err != nil {
		return SetAccountPermissionsResult{}, err
	}
	defer work.closeAccountManagement()
	var result SetAccountPermissionsResult
	found, err := work.replayAccountManagement(ctx, op, &result)
	if err != nil {
		return SetAccountPermissionsResult{}, err
	}
	if found {
		return result, nil
	}
	account, err := readManagedAccount(ctx, op, work.tx, input.AccountID)
	if err != nil {
		return SetAccountPermissionsResult{}, err
	}
	if account.IsSuperadmin {
		return SetAccountPermissionsResult{}, platform.NewError(platform.CodeForbidden, false, "唯一超管权限不可修改")
	}
	if account.Revision != input.ExpectedRevision {
		return SetAccountPermissionsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	if slices.Equal(account.Permissions, names) {
		result.Account = account
		if err := work.commitAccountManagement(ctx, op, result, false); err != nil {
			return SetAccountPermissionsResult{}, err
		}
		return result, nil
	}
	now := platform.Now()
	changed, err := work.tx.ExecContext(ctx, `UPDATE accounts SET revision=revision+1,permission_revision=permission_revision+1,updated_at=?
        WHERE account_id=? AND site_id=? AND revision=?`, now, account.AccountID, account.SiteID, input.ExpectedRevision)
	if err != nil {
		return SetAccountPermissionsResult{}, identityDBError()
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return SetAccountPermissionsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	if _, err := work.tx.ExecContext(ctx, `DELETE FROM account_permissions WHERE account_id=?`, account.AccountID); err != nil {
		return SetAccountPermissionsResult{}, identityDBError()
	}
	for _, permission := range names {
		if _, err := work.tx.ExecContext(ctx, `INSERT INTO account_permissions(account_id,permission) VALUES(?,?)`, account.AccountID, permission); err != nil {
			return SetAccountPermissionsResult{}, identityDBError()
		}
	}
	account.Permissions = names
	account.Revision++
	account.PermissionRevision++
	account.UpdatedAt = now
	var credentialRevision platform.Revision
	if err := work.tx.QueryRowContext(ctx, `SELECT credential_revision FROM credentials WHERE account_id=?`, account.AccountID).Scan(&credentialRevision); err != nil {
		return SetAccountPermissionsResult{}, identityDBError()
	}
	if _, err := PublishIdentityEvent(WithIdentityTransaction(ctx, work.tx), op, PublishIdentityEventInput{EventID: platform.NewIdentifier(), Event: IdentityChanged{
		AccountID: account.AccountID, SiteID: account.SiteID, PermissionRevision: account.PermissionRevision, CredentialRevision: credentialRevision, Reason: "permissions", OccurredAt: now}}); err != nil {
		return SetAccountPermissionsResult{}, err
	}
	result.Account = account
	if err := work.commitAccountManagement(ctx, op, result, true); err != nil {
		return SetAccountPermissionsResult{}, err
	}
	return result, nil
}

// ResetAccountPassword 将新初始口令只写Argon2哈希，原始新口令仅参与私有HMAC绑定。
func ResetAccountPassword(ctx context.Context, op OperationContext, input ResetAccountPasswordInput) (ResetAccountPasswordResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "resetAccountPassword", input)
	if err != nil {
		return ResetAccountPasswordResult{}, err
	}
	readTx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ResetAccountPasswordResult{}, identityDBError()
	}
	if err := requireAccountManagement(ctx, op, readTx, "identity:resetAccountPassword"); err != nil {
		readTx.Rollback()
		return ResetAccountPasswordResult{}, err
	}
	var result ResetAccountPasswordResult
	found, err := readIdentityReceipt(ctx, op, readTx, digest, keyID, &result)
	if err != nil {
		readTx.Rollback()
		return ResetAccountPasswordResult{}, err
	}
	if found {
		readTx.Rollback()
		store.accountMu.Lock()
		err := syncAccountIndex(ctx, op, store)
		store.accountMu.Unlock()
		if err != nil {
			return ResetAccountPasswordResult{}, err
		}
		return result, nil
	}
	account, err := readManagedAccount(ctx, op, readTx, input.AccountID)
	readTx.Rollback()
	if err != nil {
		return ResetAccountPasswordResult{}, err
	}
	if account.IsSuperadmin {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeForbidden, false, "超管重置只能走服务器恢复")
	}
	if account.Revision != input.ExpectedRevision {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	hashed, err := HashPassword(ctx, op, HashPasswordInput{Password: input.InitialPassword})
	if err != nil {
		return ResetAccountPasswordResult{}, err
	}
	work, err := beginAccountManagement(ctx, op, store, digest, keyID, "identity:resetAccountPassword")
	if err != nil {
		return ResetAccountPasswordResult{}, err
	}
	defer work.closeAccountManagement()
	found, err = work.replayAccountManagement(ctx, op, &result)
	if err != nil {
		return ResetAccountPasswordResult{}, err
	}
	if found {
		return result, nil
	}
	account, err = readManagedAccount(ctx, op, work.tx, input.AccountID)
	if err != nil {
		return ResetAccountPasswordResult{}, err
	}
	if account.IsSuperadmin {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeForbidden, false, "超管重置只能走服务器恢复")
	}
	if account.Revision != input.ExpectedRevision {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	var credentialRevision platform.Revision
	if err := work.tx.QueryRowContext(ctx, `SELECT credential_revision FROM credentials WHERE account_id=?`, account.AccountID).Scan(&credentialRevision); err != nil {
		return ResetAccountPasswordResult{}, identityDBError()
	}
	changed, err := work.tx.ExecContext(ctx, `UPDATE credentials SET password_hash=?,credential_revision=credential_revision+1 WHERE account_id=? AND credential_revision=?`, hashed.EncodedHash, account.AccountID, credentialRevision)
	if err != nil {
		return ResetAccountPasswordResult{}, identityDBError()
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeVersionMismatch, false, "凭据版本已变化")
	}
	now := platform.Now()
	changed, err = work.tx.ExecContext(ctx, `UPDATE accounts SET must_change_password=1,revision=revision+1,updated_at=? WHERE account_id=? AND site_id=? AND revision=?`, now, account.AccountID, account.SiteID, input.ExpectedRevision)
	if err != nil {
		return ResetAccountPasswordResult{}, identityDBError()
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return ResetAccountPasswordResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账户版本已变化")
	}
	account.MustChangePassword = true
	account.Revision++
	account.UpdatedAt = now
	if _, err := RevokeAllSessions(WithIdentityTransaction(ctx, work.tx), op, RevokeAllSessionsInput{AccountID: account.AccountID, Reason: "admin_reset"}); err != nil {
		return ResetAccountPasswordResult{}, err
	}
	result.Account = account
	if err := work.commitAccountManagement(ctx, op, result, true); err != nil {
		return ResetAccountPasswordResult{}, err
	}
	return result, nil
}
