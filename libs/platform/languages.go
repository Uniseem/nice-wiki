package platform

// SiteLanguage 是站点可用的一种语言：初始化时的基准语言和之后添加的语言都只能从 SiteLanguages 里选，
// 后台界面也只提供这些语言。
type SiteLanguage struct {
	Tag       string // BCP 47 规范写法
	English   string // 英文名（给决策模型和日志）
	Native    string // 本族语名称（给访客和管理员看）
	Direction string // ltr / rtl
}

// SiteLanguages 是预设的 15 种语言，顺序即界面上的展示顺序；前端清单在 libs/platform/web/ui/languages.ts，两处要一致。
var SiteLanguages = []SiteLanguage{
	{"zh-Hans", "Simplified Chinese", "简体中文", "ltr"},
	{"zh-Hant", "Traditional Chinese", "繁體中文", "ltr"},
	{"en", "English", "English", "ltr"},
	{"ja", "Japanese", "日本語", "ltr"},
	{"ko", "Korean", "한국어", "ltr"},
	{"fr", "French", "Français", "ltr"},
	{"de", "German", "Deutsch", "ltr"},
	{"es", "Spanish", "Español", "ltr"},
	{"pt", "Portuguese", "Português", "ltr"},
	{"ru", "Russian", "Русский", "ltr"},
	{"it", "Italian", "Italiano", "ltr"},
	{"vi", "Vietnamese", "Tiếng Việt", "ltr"},
	{"th", "Thai", "ไทย", "ltr"},
	{"id", "Indonesian", "Bahasa Indonesia", "ltr"},
	{"ar", "Arabic", "العربية", "rtl"},
}

// FindSiteLanguage 按规范标签找预设语言。
func FindSiteLanguage(tag string) (SiteLanguage, bool) {
	for _, language := range SiteLanguages {
		if language.Tag == tag {
			return language, true
		}
	}
	return SiteLanguage{}, false
}
