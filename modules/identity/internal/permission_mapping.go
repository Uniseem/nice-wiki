package identity

import (
	"sort"

	"nice-wiki/libs/platform"
)

// FunctionPermission 是产品确认的十项全站功能开关。
type FunctionPermission string

const (
	PermissionEditDocuments      FunctionPermission = "edit_documents"
	PermissionPublishDocuments   FunctionPermission = "publish_documents"
	PermissionManageTranslations FunctionPermission = "manage_translations"
	PermissionManageLanguages    FunctionPermission = "manage_languages"
	PermissionManageResources    FunctionPermission = "manage_resources"
	PermissionManageQA           FunctionPermission = "manage_qa"
	PermissionManageAI           FunctionPermission = "manage_ai"
	PermissionManageMemory       FunctionPermission = "manage_memory"
	PermissionManageSite         FunctionPermission = "manage_site"
	PermissionViewStats          FunctionPermission = "view_stats"
)

// PermissionSelection 是无重复的产品功能选择。nil 表示请求省略权限，
// 此时按合同默认十项全选；非 nil 的空切片表示明确不选任何功能。
type PermissionSelection []FunctionPermission

// MapFunctionGrantsInput 是功能选择到技术权限的纯规则输入。
type MapFunctionGrantsInput struct {
	Permissions  PermissionSelection `json:"permissions"`
	IsSuperadmin bool                 `json:"isSuperadmin"`
}

// MapFunctionGrantsResult 是排序去重后的技术权限集合。
type MapFunctionGrantsResult struct {
	Grants []string `json:"grants"`
}

var allFunctionPermissions = []FunctionPermission{
	PermissionEditDocuments,
	PermissionPublishDocuments,
	PermissionManageTranslations,
	PermissionManageLanguages,
	PermissionManageResources,
	PermissionManageQA,
	PermissionManageAI,
	PermissionManageMemory,
	PermissionManageSite,
	PermissionViewStats,
}

var functionGrantGroups = map[FunctionPermission][]string{
	PermissionEditDocuments:      {"content.read", "content.edit", "content.preview", "media.upload", "media.read"},
	PermissionPublishDocuments:   {"content.publish", "content.history", "content.trash"},
	PermissionManageTranslations: {"localization.manage"},
	PermissionManageLanguages:    {"localization.languages"},
	PermissionManageResources:    {"media.manage", "media.read"},
	PermissionManageQA:           {"qa.manage"},
	PermissionManageAI:           {"guard.manage", "knowledge.manage", "qa.debug"},
	PermissionManageMemory:       {"memory.manage"},
	PermissionManageSite:         {"site.manage"},
	PermissionViewStats:          {"stats.read"},
}

var authenticatedBaseGrants = []string{"portal.use", "account.self", "site.read"}

var superadminOnlyGrants = []string{
	"account.manage",
	"model.manage",
	"audit.read",
	"backup.manage",
	"system.manage",
	"superadmin",
}

// MapFunctionGrants 将固定的产品功能映射为技术权限。
// 映射不读取配置、不访问网络或数据库，也不接受调用方直接传入技术权限。
func MapFunctionGrants(input MapFunctionGrantsInput) (MapFunctionGrantsResult, error) {
	selected := input.Permissions
	if selected == nil || input.IsSuperadmin {
		selected = allFunctionPermissions
	}

	known := make(map[FunctionPermission]struct{}, len(allFunctionPermissions))
	for _, permission := range allFunctionPermissions {
		known[permission] = struct{}{}
	}
	seen := make(map[FunctionPermission]struct{}, len(selected))
	grants := make(map[string]struct{}, 3+len(selected)*2)
	for _, grant := range authenticatedBaseGrants {
		grants[grant] = struct{}{}
	}
	for _, permission := range selected {
		if _, ok := known[permission]; !ok {
			return MapFunctionGrantsResult{}, platform.NewError(platform.CodeValidation, false, "功能权限不合法")
		}
		if _, duplicate := seen[permission]; duplicate {
			return MapFunctionGrantsResult{}, platform.NewError(platform.CodeValidation, false, "功能权限不可重复")
		}
		seen[permission] = struct{}{}
		for _, grant := range functionGrantGroups[permission] {
			grants[grant] = struct{}{}
		}
	}
	if input.IsSuperadmin {
		for _, grant := range superadminOnlyGrants {
			grants[grant] = struct{}{}
		}
	}

	result := make([]string, 0, len(grants))
	for grant := range grants {
		result = append(result, grant)
	}
	sort.Strings(result)
	return MapFunctionGrantsResult{Grants: result}, nil
}
