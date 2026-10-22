package publicsite

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 页面自带的界面文字以简体中文为原文；其他语言先用 builtinUI 的内置译文，没有的退回英文。
// 只替换固定文案，正文和用户填写的内容不经过这里。整句在前、片段在后（替换按参数顺序匹配）。
var englishUI = strings.NewReplacer(
	"要切换到这种语言阅读吗？", "Switch to this language?", "保持当前语言", "Keep current language", "切换", "Switch",
	"当前语言的译文尚未完成，部分内容显示旧译文或原文。", "This translation is still in progress; some content shows an older translation or the original.",
	"回答由 AI 生成，可能有误，请以文档原文为准。", "Answers are AI-generated and may be wrong; check the docs for details.",
	"向这个站点的文档提问，回答会附上来源。", "Ask questions about these docs; answers cite their sources.",
	"需要启用 JavaScript 才能使用 AI 问答。", "JavaScript is required for Ask AI.",
	"地址可能有误，或这篇文档还没有发布。", "The address may be wrong, or this page is not published yet.",
	"复制失败，请打开 Markdown 版本", "Copy failed; open the Markdown version",
	"公开问答暂未开放。", "Ask AI is not available yet.", "这篇文档已经被删除。", "This page has been deleted.",
	"页面暂时无法访问", "Page temporarily unavailable", "请稍后刷新重试。", "Please refresh and try again later.",
	"页面不存在", "Page not found", "页面已删除", "Page deleted", "回到首页", "Back to home",
	"打开本页目录", "Open table of contents", "打开文档树", "Open navigation",
	"本页目录", "On this page", "文档树", "Navigation", "相邻文档", "Adjacent pages", "面包屑", "Breadcrumb",
	"AI 未启用", "AI unavailable", "AI 问答", "Ask AI", "亮 / 暗", "Light / Dark", "跟随系统", "System", "浅色", "Light", "深色", "Dark",
	"复制为 Markdown", "Copy as Markdown", "Markdown 版本", "Markdown version", "复制代码", "Copy code", "已复制", "Copied",
	"跳到正文", "Skip to content", "开始阅读", "Start reading", "思考中…", "Thinking…", "感谢反馈", "Thanks for the feedback",
	"有帮助", "Helpful", "没帮助", "Not helpful", "停止", "Stop",
	"最后更新", "Last updated", "上一篇", "Previous", "下一篇", "Next", "首页", "Home", "源代码", "Source code", "视频播放器", "Video player",
	"文档", "Docs", "语言", "Language",
	"新对话", "New chat", "发送", "Send", "关闭", "Close", "问问这个站点的文档…", "Ask about these docs…",
	"来源：", "Source: ", "这次没有回答成功（", "Could not answer (", "），请稍后再试。", "). Please try again later.",
	"回答连接失败（", "Connection failed (", "请求失败（", "Request failed (", "人机验证没有完成", "Verification failed",
	"人机验证无法在此浏览器运行", "Verification is not supported in this browser", "错误", "error", "：", ": ", "）", ")", "（", " (",
)

// simplifiedChinese 判断页面语言是不是界面原文所用的简体中文。
func simplifiedChinese(locale string) bool {
	return locale == "zh" || locale == "zh-Hans" || locale == "zh-CN" || locale == "zh-SG"
}

// uiText：简体中文页面原样返回；其他语言依次查后台翻译过的界面字典、内置译文，都没有就退回英文。
func uiText(locale string, catalog map[string]string, key string) string {
	if simplifiedChinese(locale) {
		return key
	}
	if value, ok := catalog[key]; ok && value != "" {
		return value
	}
	if value, ok := builtinUI[locale][key]; ok {
		return value
	}
	return englishUI.Replace(key)
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
	if simplifiedChinese(locale) {
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
