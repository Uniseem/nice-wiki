package identity

import (
	"context"
	"crypto/subtle"
	"time"

	"nice-wiki/libs/platform"
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
	Account Account       `json:"account"`
	Tokens  SessionTokens `json:"tokens"`
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

// RegisterFirstSuperadmin 校验首个注册输入并保留真实依赖边界。
// 完整写入必须由同一事务同时完成 operation receipt、bootstrapClaim、
// 账号/凭据、撤销 outbox 和已签名 session；这些依赖尚未装配时坚决不写入。
func RegisterFirstSuperadmin(ctx context.Context, op OperationContext, input RegisterFirstSuperadminInput) (RegisterFirstSuperadminResult, error) {
	if err := validateIdentityOperation(ctx, op); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	normalized, err := NormalizeAccountName(NormalizeAccountNameInput{AccountName: input.AccountName})
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if !validPassword(input.Password) || !validPassword(input.PasswordConfirmation) {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeValidation, false, "口令不合法")
	}
	if subtle.ConstantTimeCompare([]byte(input.Password), []byte(input.PasswordConfirmation)) != 1 {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeValidation, false, "两次口令不一致")
	}
	if store, ok := identityStoreFromContext(ctx); !ok || store == nil || registrationRuntimeFromContext(ctx) == nil {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "注册事务依赖尚未装配")
	}
	_ = normalized
	return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话签发与注册回执尚未装配")
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
