package meta

import (
	"context"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"strings"
)

type SEO struct{ Client *llm.Client }

func (s SEO) GenerateDescription(ctx context.Context, file content.File) (string, error) {
	text, err := doc.RenderText(file.Document)
	if err != nil {
		return "", err
	}
	response, err := s.Client.Complete(ctx, "seo-description", []protocol.Message{{Role: "user", Content: "用不超过 160 字概括以下文档：\n" + text}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Text), nil
}
