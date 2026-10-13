package meta

import (
	"context"
	"errors"
	"nice-wiki/internal/content"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/prompts"
	"strings"
)

type Slugger struct{ Client *llm.Client }

func (s Slugger) GenerateSlug(ctx context.Context, title string) (string, error) {
	prompt, err := prompts.Get("slug")
	if err != nil {
		return "", err
	}
	response, err := s.Client.Complete(ctx, "slug", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: title}})
	if err != nil {
		return "", err
	}
	value := strings.Trim(strings.TrimSpace(response.Text), "`\"'")
	if err := content.ValidateSlug(value); err != nil {
		return "", errors.New("模型生成的路径名无效")
	}
	return value, nil
}
