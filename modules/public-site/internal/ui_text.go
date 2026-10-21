package publicsite

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// 页面自带的界面文字默认是中文；基准语言以外的语言（以及所有非中文页面）改用英文。
// 只替换固定文案，正文和用户填写的内容不经过这里。
var englishUI = strings.NewReplacer(
	"当前语言的译文尚未完成，部分内容显示旧译文或原文。", "This translation is still in progress; some content shows an older translation or the original.",
	"复制失败，请打开 Markdown 版本", "Copy failed; open the Markdown version",
	"打开本页目录", "Open table of contents", "打开文档树", "Open navigation",
	"本页目录", "On this page", "文档树", "Navigation", "相邻文档", "Adjacent pages", "面包屑", "Breadcrumb",
	"AI 未启用", "AI unavailable", "AI 问答", "Ask AI", "亮 / 暗", "Light / Dark",
	"复制为 Markdown", "Copy as Markdown", "Markdown 版本", "Markdown version", "已复制", "Copied",
	"上一篇", "Previous", "下一篇", "Next", "源代码", "Source code", "视频播放器", "Video player",
	"\">文档</a>", "\">Docs</a>", "aria-label=\"语言\"", "aria-label=\"Language\"",
	"新对话", "New chat", "发送", "Send", "关闭", "Close", "问问这个站点的文档…", "Ask about these docs…",
	"来源：", "Source: ", "这次没有回答成功（", "Could not answer (", "），请稍后再试。", "). Please try again later.",
	"回答连接失败（", "Connection failed (", "请求失败（", "Request failed (", "人机验证没有完成", "Verification failed",
	"人机验证无法在此浏览器运行", "Verification is not supported in this browser", "错误", "error", "）", ")", "（", " (",
)

// localizeUI：中文页面原样输出；其他语言先套用界面字典里已翻译的条目，没翻译的退回英文。
func localizeUI(locale, text string, catalog map[string]string) string {
	if locale == "zh" || strings.HasPrefix(locale, "zh-") {
		return text
	}
	if len(catalog) > 0 {
		keys := make([]string, 0, len(catalog))
		for k := range catalog {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
		pairs := make([]string, 0, 2*len(keys))
		for _, k := range keys {
			switch k {
			case "文档": // 页头链接，避免误换其他文字里的“文档”
				pairs = append(pairs, "\">文档</a>", "\">"+catalog[k]+"</a>")
			case "语言":
				pairs = append(pairs, "aria-label=\"语言\"", "aria-label=\""+catalog[k]+"\"")
			default:
				pairs = append(pairs, k, catalog[k])
			}
		}
		text = strings.NewReplacer(pairs...).Replace(text)
	}
	return englishUI.Replace(text)
}

var uiCache = struct {
	sync.Mutex
	entries map[string]uiCacheEntry
}{entries: map[string]uiCacheEntry{}}

type uiCacheEntry struct {
	strings map[string]string
	expires time.Time
}

// loadUIStrings 读取某语言已翻译的界面字典（key=中文原文，value=译文，只含确实翻译过的条目），缓存一分钟；读不到就当没有。
func loadUIStrings(ctx context.Context, locale string) map[string]string {
	if locale == "zh" || strings.HasPrefix(locale, "zh-") {
		return nil
	}
	uiCache.Lock()
	if entry, ok := uiCache.entries[locale]; ok && time.Now().Before(entry.expires) {
		uiCache.Unlock()
		return entry.strings
	}
	uiCache.Unlock()
	result := map[string]string{}
	if directory, err := directoryFrom(ctx); err == nil && directory.Localization != nil {
		if raw, err := directory.Localization.ReadInterfaceCatalog(ctx, locale); err == nil {
			var reply struct {
				Strings []struct {
					Key    string `json:"key"`
					Value  string `json:"value"`
					Origin string `json:"origin"`
				} `json:"strings"`
			}
			if json.Unmarshal(raw, &reply) == nil {
				for _, item := range reply.Strings {
					if item.Origin != "builtin" && item.Value != "" && item.Value != item.Key {
						result[item.Key] = item.Value
					}
				}
			}
		}
	}
	uiCache.Lock()
	uiCache.entries[locale] = uiCacheEntry{result, time.Now().Add(time.Minute)}
	uiCache.Unlock()
	return result
}

type translatedTitle struct{ revision, title string }

var titleCache = struct {
	sync.Mutex
	entries map[string]titleCacheEntry
}{entries: map[string]titleCacheEntry{}}

type titleCacheEntry struct {
	titles  map[string]translatedTitle
	expires time.Time
}

// loadTranslatedTitles 读取某语言各文档的标题译文（文档 ID → 对应发布修订与译文标题），缓存一分钟；读不到就当没有，页面沿用原文标题。
func loadTranslatedTitles(ctx context.Context, locale string) map[string]translatedTitle {
	titleCache.Lock()
	if entry, ok := titleCache.entries[locale]; ok && time.Now().Before(entry.expires) {
		titleCache.Unlock()
		return entry.titles
	}
	titleCache.Unlock()
	result := map[string]translatedTitle{}
	if directory, err := directoryFrom(ctx); err == nil && directory.Localization != nil {
		if raw, err := directory.Localization.ListTranslationTitles(ctx, url.Values{"targetLanguage": {locale}}); err == nil {
			var reply struct {
				Titles []struct {
					DocumentID          string `json:"documentId"`
					PublishedRevisionID string `json:"publishedRevisionId"`
					Title               string `json:"title"`
				} `json:"titles"`
			}
			if json.Unmarshal(raw, &reply) == nil {
				for _, item := range reply.Titles {
					result[item.DocumentID] = translatedTitle{item.PublishedRevisionID, item.Title}
				}
			}
		}
	}
	titleCache.Lock()
	titleCache.entries[locale] = titleCacheEntry{result, time.Now().Add(time.Minute)}
	titleCache.Unlock()
	return result
}
