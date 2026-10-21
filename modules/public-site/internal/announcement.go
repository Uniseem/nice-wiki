package publicsite

import (
	"context"
	"html"
	"nice-wiki/libs/platform"
	"time"
)

type RenderAnnouncementInput struct {
	AnnouncementID string
	Text           string
	Link           string
	StartAt        string
	EndAt          string
	Now            time.Time
}
type RenderAnnouncementResult struct {
	HTML    string
	Visible bool
}

// RenderAnnouncement 使用左闭右开的时间窗，关闭状态只存浏览器本机。
func RenderAnnouncement(ctx context.Context, op platform.OperationContext, in RenderAnnouncementInput) (RenderAnnouncementResult, error) {
	start, err := time.Parse(time.RFC3339Nano, in.StartAt)
	if err != nil {
		return RenderAnnouncementResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_WINDOW")
	}
	end, err := time.Parse(time.RFC3339Nano, in.EndAt)
	if err != nil || !end.After(start) {
		return RenderAnnouncementResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_WINDOW")
	}
	if in.Now.Before(start) || !in.Now.Before(end) {
		return RenderAnnouncementResult{}, nil
	}
	text := html.EscapeString(in.Text)
	if in.Link != "" {
		if !safeLink(in.Link) {
			return RenderAnnouncementResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_LINK")
		}
		text = `<a class="font-medium underline decoration-accent-foreground/50 underline-offset-4 hover:decoration-accent-foreground" href="` + html.EscapeString(in.Link) + `">` + text + ` →</a>`
	}
	// 关闭状态只存浏览器本机；紧跟一段内联脚本，在页面脚本加载前就把已关闭的横幅藏起来，避免闪一下。
	// 用原样字符串写标记，类名两侧是普通引号，样式构建扫描 Go 源码时能识别出这些工具类。
	markup := `<aside data-announcement="` + html.EscapeString(in.AnnouncementID) + `" class="relative bg-accent px-12 py-2.5 text-center text-sm text-accent-foreground">` + text +
		`<button type="button" data-dismiss-announcement class="absolute end-2 top-1/2 flex size-7 -translate-y-1/2 items-center justify-center rounded-full text-accent-foreground/80 hover:bg-black/10 hover:text-accent-foreground" aria-label="关闭公告">` +
		`<svg class="size-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m4 4 8 8M12 4l-8 8"/></svg></button></aside>` +
		`<script>(function(s){var a=s.previousElementSibling;try{if(localStorage.getItem('announcement:'+a.dataset.announcement)==='closed')a.hidden=true}catch(e){}})(document.currentScript)</script>`
	return RenderAnnouncementResult{markup, true}, nil
}
