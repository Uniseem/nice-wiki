package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/identity/internal/gen"
)

// ReadRegistrationStateInput 是决定后台显示首次注册或登录的输入。
type ReadRegistrationStateInput struct{}

// ReadRegistrationStateResult 是注册状态的脱敏快照。
type ReadRegistrationStateResult struct {
	RegistrationOpen bool                 `json:"registrationOpen"`
	Initialized      bool                 `json:"initialized"`
	SiteID           *platform.Identifier `json:"siteId"`
}

// RegisterFirstSuperadminInput 是首个注册账户输入。
type RegisterFirstSuperadminInput struct {
	AccountName          AccountName `json:"accountName"`
	Password             Password    `json:"password"`
	PasswordConfirmation Password    `json:"passwordConfirmation"`
}

// SessionTokens 是注册结果需要返回给 cookie adapter 的敏感会话结果。
// 由 IssueSessionTokens 签发，敏感字段不进入普通回执、日志或前端状态。
type SessionTokens struct {
	SessionID          platform.Identifier  `json:"sessionId"`
	AccessToken        string               `json:"accessToken"`
	RefreshToken       string               `json:"refreshToken"`
	CSRFToken          string               `json:"csrfToken"`
	Auth               platform.AuthContext `json:"auth"`
	MustChangePassword bool                 `json:"mustChangePassword"`
	ExpiresAt          platform.Timestamp   `json:"expiresAt"`
}

// RegisterFirstSuperadminResult 是首个注册账户结果。
type RegisterFirstSuperadminResult struct {
	Account   Account       `json:"account"`
	Tokens    SessionTokens `json:"tokens"`
	NextRoute string        `json:"nextRoute"`
}

// InitializationStatus 是 site-config 读取的最小可信边界。
type InitializationStatus struct {
	Initialized bool
	SiteID      *platform.Identifier
}

// RegistrationRuntime 由装配层注入 site-config 的初始化状态读取；不把
// 未知状态转换为允许注册。
type RegistrationRuntime interface {
	ReadInitializationStatus(context.Context, OperationContext) (InitializationStatus, error)
}

type registrationRuntimeContextKey struct{}

// WithRegistrationRuntime 注入 site-config 初始化状态读取边界。
func WithRegistrationRuntime(ctx context.Context, runtime RegistrationRuntime) context.Context {
	return context.WithValue(ctx, registrationRuntimeContextKey{}, runtime)
}

func registrationRuntimeFromContext(ctx context.Context) RegistrationRuntime {
	runtime, _ := ctx.Value(registrationRuntimeContextKey{}).(RegistrationRuntime)
	return runtime
}

// ReadRegistrationState 读取唯一超管索引与 site-config 初始化状态。
// 注册开关只由账户权威决定；初始化状态不会在已有账户后重开注册。
func ReadRegistrationState(ctx context.Context, op OperationContext, input ReadRegistrationStateInput) (ReadRegistrationStateResult, error) {
	_ = input
	if err := validateIdentityOperation(ctx, op); err != nil {
		return ReadRegistrationStateResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	runtime := registrationRuntimeFromContext(ctx)
	if !ok || runtime == nil {
		return ReadRegistrationStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份注册依赖尚未装配")
	}
	initialization, err := runtime.ReadInitializationStatus(ctx, op)
	if err != nil {
		return ReadRegistrationStateResult{}, mapRegistrationDependencyError(err)
	}

	var accountCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountCount); err != nil {
		return ReadRegistrationStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户权威表不可读")
	}
	result := ReadRegistrationStateResult{
		RegistrationOpen: accountCount == 0,
		Initialized:      initialization.Initialized,
	}
	if accountCount == 0 {
		_, claimed, err := readBootstrapClaim(ctx, store.control)
		if err != nil {
			return ReadRegistrationStateResult{}, err
		}
		result.RegistrationOpen = !claimed
		return result, nil
	}
	var siteID string
	var superadminCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(site_id), '') FROM accounts WHERE is_superadmin = 1`).Scan(&superadminCount, &siteID); err != nil {
		return ReadRegistrationStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "超管索引不可读")
	}
	if superadminCount != 1 || !platform.ValidIdentifier(siteID) {
		return ReadRegistrationStateResult{}, platform.NewError(platform.CodeIntegrityError, false, "唯一超管索引不一致")
	}
	result.SiteID = &siteID
	return result, nil
}

// RegisterFirstSuperadmin 先持久control不可逆决定，再提交active唯一超管与会话。
// 跨库和索引以安全回执及控制决定恢复，不把cookie明文放入普通回执。
func RegisterFirstSuperadmin(ctx context.Context, op OperationContext, input RegisterFirstSuperadminInput) (RegisterFirstSuperadminResult, error) {
	ctx, op, err := anonymousIdentityOperation(ctx, op)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	normalized, err := NormalizeAccountName(NormalizeAccountNameInput{AccountName: input.AccountName})
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if subtle.ConstantTimeCompare([]byte(input.Password), []byte(input.PasswordConfirmation)) != 1 {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeValidation, false, "两次口令不一致")
	}
	attempt, err := beginIdentityLogin(ctx, op, normalized.Normalized, "bootstrap", "", "")
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	outcomeSent := false
	defer func() {
		if !outcomeSent {
			_ = finishIdentityLogin(ctx, op, attempt, "aborted")
		}
	}()
	store, ok := identityStoreFromContext(ctx)
	runtime := registrationRuntimeFromContext(ctx)
	if !ok || runtime == nil {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "注册依赖尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "registerFirstSuperadmin", input)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	initialized, err := runtime.ReadInitializationStatus(ctx, op)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if initialized.Initialized || initialized.SiteID != nil {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeConflict, false, "站点已有初始化身份，不能重新注册")
	}
	hashed, err := HashPassword(ctx, op, HashPasswordInput{Password: input.Password})
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	store.accountMu.Lock()
	defer store.accountMu.Unlock()
	claim, err := reserveBootstrap(ctx, op, store, digest, keyID)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	defer tx.Rollback()
	var receipt sensitiveOperationReceipt
	found, err := readIdentityReceipt(ctx, op, tx, digest, keyID, &receipt)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if found {
		tx.Rollback()
		if err := markBootstrapCommitted(ctx, store, claim); err != nil {
			return RegisterFirstSuperadminResult{}, err
		}
		if err := syncAccountIndex(ctx, op, store); err != nil {
			return RegisterFirstSuperadminResult{}, err
		}
		outcomeSent = true
		return RegisterFirstSuperadminResult{}, sensitiveReplayError()
	}
	if claim.State == "committed" {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeConflict, false, "站点已完成首次注册，禁止重开")
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	if count != 0 {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeConflict, false, "首次注册已关闭")
	}
	account := Account{AccountID: claim.AccountID, SiteID: claim.SiteID, AccountName: normalized.Normalized, IsSuperadmin: true,
		Permissions: selectedPermissionNames(nil), Status: AccountStatusActive, Revision: 1, PermissionRevision: 1,
		CreatedAt: claim.ClaimedAt, UpdatedAt: platform.Now()}
	if err := insertAccount(ctx, tx, account, hashed.EncodedHash); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := createPlatformOwner(ctx, tx, account); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	session, auth, err := insertSession(ctx, tx, account, 1, "")
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	inner := WithIdentityTransaction(ctx, tx)
	issued, err := IssueSessionTokens(inner, op, IssueSessionTokensInput{Session: session, Auth: auth})
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	eventOp := op
	eventOp.Actor = platform.AuthContext{Kind: "service", SubjectID: account.AccountID, SiteID: account.SiteID}
	if _, err := PublishIdentityEvent(inner, eventOp, PublishIdentityEventInput{EventID: platform.NewIdentifier(), Event: IdentityChanged{
		AccountID: account.AccountID, SiteID: account.SiteID, PermissionRevision: 1, CredentialRevision: 1, Reason: "permissions", OccurredAt: account.UpdatedAt}}); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	receipt = sensitiveOperationReceipt{AccountID: account.AccountID, SessionID: session.SessionID, CredentialRevision: 1}
	if err := saveIdentityReceipt(ctx, op, tx, digest, keyID, receipt); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	outcomeSent = true
	if err := finishIdentityLogin(ctx, op, attempt, "authenticated"); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := commitAccountWrite(ctx, op, store, tx); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := markBootstrapCommitted(ctx, store, claim); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	return RegisterFirstSuperadminResult{Account: account, Tokens: issued.Tokens, NextRoute: "/admin/setup"}, nil
}

func validateIdentityOperation(ctx context.Context, op OperationContext) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "操作上下文缺失")
	}
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "操作期限已到")
		}
		return platform.NewError(platform.CodeCancelled, true, "操作已取消")
	}
	if op.DataEpoch == "" {
		return platform.NewError(platform.CodeValidation, false, "数据代次缺失")
	}
	if op.Deadline == "" {
		return platform.NewError(platform.CodeValidation, false, "操作期限缺失")
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return err
	}
	if !deadline.After(time.Now().UTC()) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "操作期限已到")
	}
	return nil
}

func mapRegistrationDependencyError(err error) error {
	if err == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "注册状态依赖不可用")
	}
	if problem, ok := err.(*platform.Error); ok {
		return problem
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "注册状态依赖不可用")
}

type bootstrapClaim struct {
	OperationID   string
	CallerService string
	ActorID       string
	ActorSiteID   string
	RequestDigest string
	DigestKeyID   string
	DataEpoch     string
	AccountID     string
	SiteID        string
	ClaimedAt     string
	State         string
}

func readBootstrapClaim(ctx context.Context, control *sql.DB) (bootstrapClaim, bool, error) {
	var claim bootstrapClaim
	err := control.QueryRowContext(ctx, `SELECT operation_id,caller_service,actor_id,actor_site_id,request_digest,digest_key_id,
        data_epoch,account_id,site_id,claimed_at,state FROM bootstrap_claim WHERE singleton=1`).
		Scan(&claim.OperationID, &claim.CallerService, &claim.ActorID, &claim.ActorSiteID, &claim.RequestDigest, &claim.DigestKeyID,
			&claim.DataEpoch, &claim.AccountID, &claim.SiteID, &claim.ClaimedAt, &claim.State)
	if errors.Is(err, sql.ErrNoRows) {
		return bootstrapClaim{}, false, nil
	}
	if err != nil {
		return bootstrapClaim{}, false, identityDBError()
	}
	return claim, true, nil
}

func reserveBootstrap(ctx context.Context, op OperationContext, store *Store, digest, keyID string) (bootstrapClaim, error) {
	claim, found, err := readBootstrapClaim(ctx, store.control)
	if err != nil {
		return bootstrapClaim{}, err
	}
	if found {
		if claim.OperationID != op.OperationID || claim.CallerService != op.CallerService || claim.RequestDigest != digest ||
			claim.DigestKeyID != keyID || claim.DataEpoch != op.DataEpoch || claim.ActorID != op.Actor.SubjectID || claim.ActorSiteID != op.Actor.SiteID {
			return bootstrapClaim{}, platform.NewError(platform.CodeConflict, false, "首次注册已由另一不可逆决定保留")
		}
		return claim, nil
	}
	claim = bootstrapClaim{OperationID: op.OperationID, CallerService: op.CallerService, ActorID: op.Actor.SubjectID, ActorSiteID: op.Actor.SiteID,
		RequestDigest: digest, DigestKeyID: keyID, DataEpoch: op.DataEpoch, AccountID: platform.NewIdentifier(), SiteID: platform.NewIdentifier(), ClaimedAt: platform.Now(), State: "reserved"}
	_, err = store.control.ExecContext(ctx, `INSERT INTO bootstrap_claim(singleton,schema_version,operation_id,caller_service,actor_id,actor_site_id,
        request_digest,digest_key_id,data_epoch,account_id,site_id,claimed_at,state) VALUES(1,1,?,?,?,?,?,?,?,?,?,?,?)`,
		claim.OperationID, claim.CallerService, claim.ActorID, claim.ActorSiteID, claim.RequestDigest, claim.DigestKeyID, claim.DataEpoch,
		claim.AccountID, claim.SiteID, claim.ClaimedAt, claim.State)
	if err != nil {
		return bootstrapClaim{}, identityDBError()
	}
	return claim, nil
}

func markBootstrapCommitted(ctx context.Context, store *Store, claim bootstrapClaim) error {
	changed, err := store.control.ExecContext(ctx, `UPDATE bootstrap_claim SET state='committed' WHERE singleton=1 AND operation_id=?
        AND request_digest=? AND digest_key_id=? AND data_epoch=?`, claim.OperationID, claim.RequestDigest, claim.DigestKeyID, claim.DataEpoch)
	if err != nil {
		return identityDBError()
	}
	if count, err := changed.RowsAffected(); err != nil || count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "首次注册控制决定不一致")
	}
	return nil
}

func recoverBootstrapClaim(store *Store) error {
	ctx := context.Background()
	claim, found, err := readBootstrapClaim(ctx, store.control)
	if err != nil || !found {
		return err
	}
	var accountID, siteID string
	err = store.db.QueryRow(`SELECT account_id,site_id FROM accounts WHERE is_superadmin=1`).Scan(&accountID, &siteID)
	if errors.Is(err, sql.ErrNoRows) {
		if claim.State == "committed" {
			return platform.NewError(platform.CodeIntegrityError, false, "已注册控制决定缺少活动超管，禁止重开注册")
		}
		return nil
	}
	if err != nil {
		return identityDBError()
	}
	if accountID != claim.AccountID || siteID != claim.SiteID {
		return platform.NewError(platform.CodeIntegrityError, false, "活动超管与不可逆控制身份不一致")
	}
	if claim.State == "committed" {
		return nil
	}
	var digest, keyID, resultHash, state string
	var raw sql.NullString
	if err := store.db.QueryRow(`SELECT input_digest,digest_key_id,result_hash,state,result_json FROM operation_receipts WHERE site_id=? AND actor_id=? AND operation_id=?`,
		claim.ActorSiteID, claim.ActorID, claim.OperationID).Scan(&digest, &keyID, &resultHash, &state, &raw); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "注册活动提交缺少安全回执")
	}
	var receipt sensitiveOperationReceipt
	if digest != claim.RequestDigest || keyID != claim.DigestKeyID || state != "committed" || !raw.Valid ||
		platform.HashBytes([]byte(raw.String)) != resultHash || json.Unmarshal([]byte(raw.String), &receipt) != nil || receipt.AccountID != claim.AccountID {
		return platform.NewError(platform.CodeIntegrityError, false, "注册安全回执与控制决定不一致")
	}
	return markBootstrapCommitted(ctx, store, claim)
}

// RegistrationClient 读取site-config的安全初始化阶段；未知状态不转成未初始化。
type RegistrationClient struct{ Client *gen.SiteConfigClient }

func (runtime RegistrationClient) ReadInitializationStatus(ctx context.Context, op OperationContext) (InitializationStatus, error) {
	if runtime.Client == nil {
		return InitializationStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "初始化客户端尚未装配")
	}
	raw, err := runtime.Client.ReadInitializationStatus(platform.WithOperationContext(ctx, op))
	if err != nil {
		return InitializationStatus{}, err
	}
	var response struct {
		Initialized  *bool           `json:"initialized"`
		State        json.RawMessage `json:"state"`
		Coordination *struct {
			SiteID *string `json:"siteId"`
		} `json:"coordination"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Initialized == nil || len(response.State) == 0 || response.Coordination == nil {
		return InitializationStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "初始化权威响应不完整")
	}
	var phase struct {
		Phase string `json:"phase"`
	}
	if json.Unmarshal(response.State, &phase) != nil {
		return InitializationStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "初始化阶段响应不可用")
	}
	switch phase.Phase {
	case "uninitialized", "preparing", "config_committed", "completed", "aborting", "aborted", "blocked":
	default:
		return InitializationStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "初始化阶段未知")
	}
	if response.Coordination.SiteID != nil && !platform.ValidIdentifier(*response.Coordination.SiteID) {
		return InitializationStatus{}, platform.NewError(platform.CodeIntegrityError, false, "站点身份响应无效")
	}
	if *response.Initialized && response.Coordination.SiteID == nil {
		return InitializationStatus{}, platform.NewError(platform.CodeIntegrityError, false, "已初始化站点缺少身份归属")
	}
	return InitializationStatus{Initialized: *response.Initialized, SiteID: response.Coordination.SiteID}, nil
}

// createPlatformOwner 给站主建用户记录和第一个站点“主站”（站点身份就是首次注册时预留的那个）。
func createPlatformOwner(ctx context.Context, tx *sql.Tx, account Account) error {
	settings, err := loadSettings(ctx, tx)
	if err != nil {
		return err
	}
	userID, now := platform.NewIdentifier(), platform.Now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(user_id,account_id,email,display_name,status,created_at,updated_at) VALUES(?,?,'',?,'active',?,?)`,
		userID, account.AccountID, account.AccountName, now, now); err != nil {
		return identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_limits(user_id,max_sites,storage_bytes,updated_at) VALUES(?,1000,?,?)`, userID, int64(1)<<40, now); err != nil {
		return identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sites(site_id,slug,name,owner_user_id,status,visibility,base_language,revision,created_at,updated_at) VALUES(?,'main','主站',?,'active','public','zh-Hans',1,?,?)`,
		account.SiteID, userID, now, now); err != nil {
		return identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO site_members(site_id,user_id,role,created_at) VALUES(?,?,'owner',?)`, account.SiteID, userID, now); err != nil {
		return identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO site_domains(host,site_id,kind,status,is_primary,verify_token,verified_at,created_at) VALUES(?,?,'subdomain','verified',1,'',?,?)`,
		"main."+sitesDomainOf(settings), account.SiteID, now, now); err != nil {
		return identityDBError()
	}
	return nil
}
