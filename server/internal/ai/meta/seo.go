package meta

import (
	"context"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/prompts"
	"strings"
)

type SEO struct{ Client *llm.Client }

func (s SEO) GenerateDescription(ctx context.Context, file content.File) (string, error) {
	text, err := doc.RenderText(file.Document)
	if err != nil {
		return "", err
	}
	prompt, err := prompts.Get("seo-description")
	if err != nil {
		return "", err
	}
	response, err := s.Client.Complete(ctx, "seo-description", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: text}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Text), nil
}
