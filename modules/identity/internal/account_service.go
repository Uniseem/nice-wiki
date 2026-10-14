package identity

import (
	"context"

	"nice-wiki/libs/platform"
)

// CreateAccountInput 是超管创建子账号的输入。
type CreateAccountInput struct {
	AccountName    AccountName        `json:"accountName"`
	InitialPassword Password           `json:"initialPassword"`
	Permissions    PermissionSelection `json:"permissions"`
}

// CreateAccountResult 是创建子账号的结果。
type CreateAccountResult struct {
	Account Account `json:"account"`
}

// CreateAccount 执行创建子账号所需的安全前置校验。
// 写入暂缓至 operation receipt、account-index 和撤销 outbox 的同事务装配完成，
// 避免留下没有幂等回执或不可撤销权限的半成品账户。
func CreateAccount(ctx context.Context, op OperationContext, input CreateAccountInput) (CreateAccountResult, error) {
	if err := validateIdentityOperation(ctx, op); err != nil {
		return CreateAccountResult{}, err
	}
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "superadmin") {
		return CreateAccountResult{}, platform.NewError(platform.CodeForbidden, false, "仅超管可以创建账户")
	}
	if op.Actor.SiteID == "" || !platform.ValidIdentifier(op.Actor.SiteID) {
		return CreateAccountResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账户主体无效")
	}
	normalized, err := NormalizeAccountName(NormalizeAccountNameInput{AccountName: input.AccountName})
	if err != nil {
		return CreateAccountResult{}, err
	}
	if !validPassword(input.InitialPassword) {
		return CreateAccountResult{}, platform.NewError(platform.CodeValidation, false, "口令不合法")
	}
	if _, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: input.Permissions}); err != nil {
		return CreateAccountResult{}, err
	}
	if identityStoreFromContext(ctx) == nil {
		return CreateAccountResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	_ = normalized
	return CreateAccountResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户回执、索引与撤销事件尚未装配")
}

func hasGrant(grants []string, wanted string) bool {
	for _, grant := range grants {
		if grant == wanted {
			return true
		}
	}
	return false
}
