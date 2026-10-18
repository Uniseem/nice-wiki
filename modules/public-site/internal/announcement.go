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
		text = "<a href=\"" + html.EscapeString(in.Link) + "\">" + text + "</a>"
	}
	markup := "<aside data-announcement=\"" + html.EscapeString(in.AnnouncementID) + "\">" + text + "<button type=\"button\" aria-label=\"关闭公告\">×</button></aside><script>(function(s){var a=s.previousElementSibling,k='announcement:'+a.dataset.announcement;try{if(localStorage.getItem(k)==='closed')a.hidden=true}catch(e){}a.querySelector('button').addEventListener('click',function(){a.hidden=true;try{localStorage.setItem(k,'closed')}catch(e){}})})(document.currentScript)</script>"
	return RenderAnnouncementResult{markup, true}, nil
}
