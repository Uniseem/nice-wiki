package internal

import (
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/portal/internal/gen"
)

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
	capabilities := make([]Capability, 0, len(gen.AdminRoutes)+3)
	for _, definition := range gen.AdminRoutes {
		enabled := false
		for _, permission := range definition.Permissions {
			if _, ok := grants[permission]; ok {
				enabled = true
				break
			}
		}
		if _, superadmin := grants["superadmin"]; superadmin {
			enabled = true
		}
		var reason *string
		if !enabled {
			value := "missing_permission"
			reason = &value
		}
		capabilities = append(capabilities, Capability{Key: definition.Key, Enabled: enabled, Reason: reason})
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
	navigation := make([]NavigationItem, 0, len(gen.AdminRoutes))
	for _, definition := range gen.AdminRoutes {
		if !definition.Navigation {
			continue
		}
		if !enabled[definition.Key] {
			continue
		}
		label := definition.LabelEN
		if locale == "zh" {
			label = definition.LabelZH
		}
		navigation = append(navigation, NavigationItem{
			Key: definition.Key, Route: definition.Path, Label: label, RequiredCapability: definition.Key,
		})
	}
	return BuildNavigationResult{Navigation: navigation}, nil
}
