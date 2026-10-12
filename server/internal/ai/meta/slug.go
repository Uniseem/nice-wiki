package meta

import (
	"context"
	"errors"
	"nice-wiki/internal/content"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"strings"
)

type Slugger struct{ Client *llm.Client }

func (s Slugger) GenerateSlug(ctx context.Context, title string) (string, error) {
	response, err := s.Client.Complete(ctx, "slug", []protocol.Message{{Role: "user", Content: "Generate one lowercase English URL slug for: " + title}})
	if err != nil {
		return "", err
	}
	value := strings.Trim(strings.TrimSpace(response.Text), "`\"'")
	if err := content.ValidateSlug(value); err != nil {
		return "", errors.New("模型生成的路径名无效")
	}
	return value, nil
}
