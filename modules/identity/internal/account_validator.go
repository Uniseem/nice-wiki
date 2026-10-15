// Package identity 内聚本域身份规则；本文件只处理账号名与超管不变式。
package identity

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"nice-wiki/libs/platform"
)

// AccountName 与 types.json 的账号名标量对应。
type AccountName = string

// AccountStatus 与 types.json 的账户状态枚举对应。
type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusDisabled AccountStatus = "disabled"
	AccountStatusDeleted  AccountStatus = "deleted"
)

// Account 是 AssertSuperadminImmutable 所需的账户元数据。
// 字段与 types.json 的 Account schema 保持一致；凭据和会话不属于此规则输入。
type Account struct {
	AccountID          platform.Identifier `json:"accountId"`
	SiteID             platform.Identifier `json:"siteId"`
	AccountName        AccountName         `json:"accountName"`
	IsSuperadmin       bool                `json:"isSuperadmin"`
	Permissions        []string            `json:"permissions"`
	MustChangePassword bool                `json:"mustChangePassword"`
	Status             AccountStatus       `json:"status"`
	Revision           platform.Revision   `json:"revision"`
	PermissionRevision platform.Revision   `json:"permissionRevision"`
	CreatedAt          platform.Timestamp  `json:"createdAt"`
	UpdatedAt          platform.Timestamp  `json:"updatedAt"`
}

// NormalizeAccountNameInput 是账号名规范化输入。
type NormalizeAccountNameInput struct {
	AccountName AccountName `json:"accountName"`
}

// NormalizeAccountNameResult 是账号名规范化结果。
type NormalizeAccountNameResult struct {
	Normalized AccountName `json:"normalized"`
}

const accountNameMaxRunes = 64

// NormalizeAccountName 将账号名规范为 NFC，并拒绝无效 UTF-8、控制字符、
// 首尾空白以及超出合同上限的输入。账号名不做trim或大小写折叠。
func NormalizeAccountName(input NormalizeAccountNameInput) (NormalizeAccountNameResult, error) {
	raw := string(input.AccountName)
	if !utf8.ValidString(raw) {
		return NormalizeAccountNameResult{}, platform.NewError(platform.CodeValidation, false, "账号名不合法")
	}

	normalized := norm.NFC.String(raw)
	runes := []rune(normalized)
	if len(runes) == 0 || len(runes) > accountNameMaxRunes {
		return NormalizeAccountNameResult{}, platform.NewError(platform.CodeValidation, false, "账号名不合法")
	}
	if unicode.IsSpace(runes[0]) || unicode.IsSpace(runes[len(runes)-1]) {
		return NormalizeAccountNameResult{}, platform.NewError(platform.CodeValidation, false, "账号名不合法")
	}
	for _, r := range runes {
		if unicode.IsControl(r) {
			return NormalizeAccountNameResult{}, platform.NewError(platform.CodeValidation, false, "账号名不合法")
		}
	}

	return NormalizeAccountNameResult{Normalized: normalized}, nil
}

// AssertSuperadminImmutableInput 是保护唯一超管永久有效的输入。
type AssertSuperadminImmutableInput struct {
	Account         Account       `json:"account"`
	RequestedStatus AccountStatus `json:"requestedStatus"`
}

// AssertSuperadminImmutableResult 是保护唯一超管永久有效的结果。
type AssertSuperadminImmutableResult struct {
	Allowed bool `json:"allowed"`
}

// AssertSuperadminImmutable 校验账户状态变更是否破坏唯一超管不变式。
// 超管只能保持active；子账号遵循active↔disabled→deleted状态机，deleted为墓碑。
// 权限列表只表示功能授权，不能改变isSuperadmin身份。
func AssertSuperadminImmutable(input AssertSuperadminImmutableInput) (AssertSuperadminImmutableResult, error) {
	if !validAccountStatus(input.Account.Status) || !validAccountStatus(input.RequestedStatus) {
		return AssertSuperadminImmutableResult{}, platform.NewError(platform.CodeValidation, false, "账户状态不合法")
	}

	if input.Account.IsSuperadmin {
		if input.Account.Status != AccountStatusActive || input.RequestedStatus != AccountStatusActive {
			return AssertSuperadminImmutableResult{}, platform.NewError(platform.CodeForbidden, false, "超管账户必须保持启用")
		}
		return AssertSuperadminImmutableResult{Allowed: true}, nil
	}

	if !validAccountStatusTransition(input.Account.Status, input.RequestedStatus) {
		return AssertSuperadminImmutableResult{}, platform.NewError(platform.CodeForbidden, false, "账户状态迁移不允许")
	}
	return AssertSuperadminImmutableResult{Allowed: true}, nil
}

func validAccountStatus(status AccountStatus) bool {
	switch status {
	case AccountStatusActive, AccountStatusDisabled, AccountStatusDeleted:
		return true
	default:
		return false
	}
}

func validAccountStatusTransition(current, requested AccountStatus) bool {
	switch current {
	case AccountStatusActive:
		return requested == AccountStatusActive || requested == AccountStatusDisabled
	case AccountStatusDisabled:
		return requested == AccountStatusActive || requested == AccountStatusDisabled || requested == AccountStatusDeleted
	case AccountStatusDeleted:
		return requested == AccountStatusDeleted
	default:
		return false
	}
}
