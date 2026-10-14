package internal

import (
	"strings"

	"nice-wiki/libs/platform"
)

type navigationDefinition struct {
	key        string
	capability string
	route      string
	labels     map[string]string
}

var navigationDefinitions = []navigationDefinition{
	{key: "content", capability: "content.read", route: "/admin/content", labels: map[string]string{"zh-CN": "内容", "en": "Content"}},
	{key: "media", capability: "media.upload", route: "/admin/media", labels: map[string]string{"zh-CN": "媒体", "en": "Media"}},
	{key: "knowledge", capability: "knowledge.manage", route: "/admin/knowledge", labels: map[string]string{"zh-CN": "知识库", "en": "Knowledge"}},
	{key: "qa", capability: "qa.manage", route: "/admin/qa", labels: map[string]string{"zh-CN": "问答", "en": "Q&A"}},
	{key: "localization", capability: "localization.manage", route: "/admin/localization", labels: map[string]string{"zh-CN": "本地化", "en": "Localization"}},
	{key: "memory", capability: "memory.manage", route: "/admin/memory", labels: map[string]string{"zh-CN": "长期记忆", "en": "Memory"}},
	{key: "jobs", capability: "job.manage", route: "/admin/jobs", labels: map[string]string{"zh-CN": "任务", "en": "Jobs"}},
	{key: "analytics", capability: "stats.read", route: "/admin/analytics", labels: map[string]string{"zh-CN": "统计", "en": "Analytics"}},
	{key: "guard", capability: "guard.manage", route: "/admin/security", labels: map[string]string{"zh-CN": "安全", "en": "Security"}},
	{key: "settings", capability: "site.manage", route: "/admin/settings", labels: map[string]string{"zh-CN": "设置", "en": "Settings"}},
}

func BuildCapabilities(input BuildCapabilitiesInput) (BuildCapabilitiesResult, error) {
	if len(input.Grants) > 100 {
		return BuildCapabilitiesResult{}, platform.NewError(platform.CodeValidation, false, "权限数量超限")
	}
	grants := make(map[string]struct{}, len(input.Grants))
	for _, grant := range input.Grants {
		if grant == "" || len(grant) > 64 {
			return BuildCapabilitiesResult{}, platform.NewError(platform.CodeValidation, false, "权限标识不合法")
		}
		grants[grant] = struct{}{}
	}
	capabilities := make([]Capability, 0, len(navigationDefinitions)+3)
	for _, definition := range navigationDefinitions {
		_, enabled := grants[definition.capability]
		if _, superadmin := grants["superadmin"]; superadmin {
			enabled = true
		}
		var reason *string
		if !enabled {
			value := "missing_permission"
			reason = &value
		}
		capabilities = append(capabilities, Capability{Key: definition.key, Enabled: enabled, Reason: reason})
	}
	appendFlagCapability := func(key string, enabled bool) {
		var reason *string
		if !enabled {
			value := "disabled_by_site"
			reason = &value
		}
		capabilities = append(capabilities, Capability{Key: key, Enabled: enabled, Reason: reason})
	}
	appendFlagCapability("qa.enabled", input.Flags.QAEnabled)
	appendFlagCapability("localization.enabled", input.Flags.LocalizationEnabled)
	appendFlagCapability("announcements.enabled", input.Flags.AnnouncementsEnabled)
	return BuildCapabilitiesResult{Capabilities: capabilities}, nil
}

func BuildNavigation(input BuildNavigationInput) (BuildNavigationResult, error) {
	if len(input.Locale) < 2 || len(input.Locale) > 35 || strings.ContainsAny(input.Locale, " \r\n/") {
		return BuildNavigationResult{}, platform.NewError(platform.CodeValidation, false, "界面语言不合法")
	}
	enabled := make(map[string]bool, len(input.Capabilities))
	for _, capability := range input.Capabilities {
		if capability.Key == "" {
			return BuildNavigationResult{}, platform.NewError(platform.CodeValidation, false, "能力标识不合法")
		}
		enabled[capability.Key] = capability.Enabled
	}
	locale := strings.ToLower(input.Locale)
	if index := strings.IndexByte(locale, '-'); index >= 0 {
		locale = locale[:index]
	}
	navigation := make([]NavigationItem, 0, len(navigationDefinitions))
	for _, definition := range navigationDefinitions {
		if !enabled[definition.key] {
			continue
		}
		label := definition.labels[locale]
		if label == "" {
			label = definition.labels["en"]
		}
		navigation = append(navigation, NavigationItem{
			Key: definition.key, Route: definition.route, Label: label, RequiredCapability: definition.key,
		})
	}
	return BuildNavigationResult{Navigation: navigation}, nil
}
