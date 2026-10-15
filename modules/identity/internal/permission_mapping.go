package identity

import (
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal/gen"
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
	IsSuperadmin bool                `json:"isSuperadmin"`
}

// MapFunctionGrantsResult 是排序去重后的技术权限集合。
type MapFunctionGrantsResult struct {
	Grants []string `json:"grants"`
}

// 以下映射全部来自 make gen 由根 contracts/permission-groups.json 生成的
// internal/gen/permission_groups.gen.go；本文件不维护第二份手写副本。
var (
	// allFunctionPermissions 是默认全选的功能开关（合同 defaultSelected 顺序）。
	allFunctionPermissions = toFunctionPermissions(gen.DefaultSelectedPermissions)
	// registeredFunctionPermissions 是合同登记的全部功能开关，唯一超管全部获得。
	registeredFunctionPermissions = sortedFunctionPermissions(gen.PermissionGroups)
	functionGrantGroups           = toGrantGroups(gen.PermissionGroups)
	authenticatedBaseGrants       = gen.AuthenticatedBaseGrants
	superadminOnlyGrants          = gen.SuperadminOnlyGrants
)

func toFunctionPermissions(values []string) []FunctionPermission {
	result := make([]FunctionPermission, 0, len(values))
	for _, value := range values {
		result = append(result, FunctionPermission(value))
	}
	return result
}

func sortedFunctionPermissions(groups map[string][]string) []FunctionPermission {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return toFunctionPermissions(names)
}

func toGrantGroups(groups map[string][]string) map[FunctionPermission][]string {
	result := make(map[FunctionPermission][]string, len(groups))
	for name, grants := range groups {
		result[FunctionPermission(name)] = grants
	}
	return result
}

// MapFunctionGrants 将固定的产品功能映射为技术权限。
// 映射不读取配置、不访问网络或数据库，也不接受调用方直接传入技术权限。
func MapFunctionGrants(input MapFunctionGrantsInput) (MapFunctionGrantsResult, error) {
	selected := input.Permissions
	if input.IsSuperadmin {
		selected = registeredFunctionPermissions
	} else if selected == nil {
		selected = allFunctionPermissions
	}

	known := make(map[FunctionPermission]struct{}, len(functionGrantGroups))
	for permission := range functionGrantGroups {
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
