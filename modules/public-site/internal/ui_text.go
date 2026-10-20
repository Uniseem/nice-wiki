package publicsite

import "strings"

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

func localizeUI(locale, text string) string {
	if locale == "zh" || strings.HasPrefix(locale, "zh-") {
		return text
	}
	return englishUI.Replace(text)
}
