// Package identity 内聚本域身份规则；本文件只处理不可变账号名的规范化。
package identity

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"nice-wiki/libs/platform"
)

// AccountName 与 types.json 的账号名标量对应。
type AccountName = string

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
