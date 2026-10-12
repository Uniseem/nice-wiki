package auth

type Permission string

const (
	EditDocs           Permission = "editDocs"
	PublishDocs        Permission = "publishDocs"
	ManageTranslations Permission = "manageTranslations"
	ManageLanguages    Permission = "manageLanguages"
	ManageAssets       Permission = "manageAssets"
	ManageQA           Permission = "manageQA"
	AISettings         Permission = "aiSettings"
	AIMemory           Permission = "aiMemory"
	SiteSettings       Permission = "siteSettings"
	ViewStats          Permission = "viewStats"
)

var AllPermissions = []Permission{EditDocs, PublishDocs, ManageTranslations, ManageLanguages, ManageAssets, ManageQA, AISettings, AIMemory, SiteSettings, ViewStats}

func ValidPermissions(values []Permission) error {
	seen := map[Permission]bool{}
	for _, value := range values {
		found := false
		for _, known := range AllPermissions {
			if value == known {
				found = true
			}
		}
		if !found || seen[value] {
			return invalid("权限列表包含未知或重复权限")
		}
		seen[value] = true
	}
	return nil
}
func (a Account) Has(permission Permission) bool {
	if a.Disabled || a.MustChangePassword {
		return false
	}
	if a.Superadmin {
		return true
	}
	for _, value := range a.Permissions {
		if value == permission {
			return true
		}
	}
	return false
}
