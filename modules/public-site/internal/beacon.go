package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
	"strings"
)

type RecordAnonymousViewInput struct {
	DocumentID string
	Locale     string
	UserAgent  string
	Status     int
	ElapsedMs  int64
}
type RecordAnonymousViewResult struct{ Accepted bool }

// RecordAnonymousView 仅提交已知AI爬虫的成功Markdown访问，不传UA原文或IP。
func RecordAnonymousView(ctx context.Context, op platform.OperationContext, in RecordAnonymousViewInput) (RecordAnonymousViewResult, error) {
	if in.Status < 200 || in.Status >= 300 {
		return RecordAnonymousViewResult{}, nil
	}
	family := ""
	for _, known := range []string{"GPTBot", "ChatGPT-User", "ClaudeBot", "PerplexityBot"} {
		if strings.Contains(in.UserAgent, known) {
			family = known
			break
		}
	}
	if family == "" {
		return RecordAnonymousViewResult{}, nil
	}
	directory, err := directoryFrom(ctx)
	if err != nil || directory.Analytics == nil {
		return RecordAnonymousViewResult{}, nil
	}
	_, err = directory.Analytics.RecordCrawlerAccess(ctx, map[string]any{"crawlerFamily": family, "documentId": in.DocumentID, "locale": in.Locale, "status": in.Status}, string(platform.NewIdentifier()))
	return RecordAnonymousViewResult{Accepted: err == nil}, nil
}
