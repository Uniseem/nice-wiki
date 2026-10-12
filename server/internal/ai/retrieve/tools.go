package retrieve

import (
	"context"
	"errors"
)

type Tools interface {
	Search(context.Context, string, int) ([]Unit, error)
	ReadSection(context.Context, string, string) ([]Unit, error)
	Tree(context.Context) (string, error)
}
type BoundedTools struct {
	Inner     Tools
	remaining int
}

func NewBoundedTools(inner Tools, maximum int) *BoundedTools {
	return &BoundedTools{Inner: inner, remaining: maximum}
}
func (b *BoundedTools) take() error {
	if b.remaining <= 0 {
		return errors.New("检索工具调用次数已用尽")
	}
	b.remaining--
	return nil
}
func (b *BoundedTools) Search(ctx context.Context, q string, n int) ([]Unit, error) {
	if err := b.take(); err != nil {
		return nil, err
	}
	return b.Inner.Search(ctx, q, n)
}
func (b *BoundedTools) ReadSection(ctx context.Context, d, h string) ([]Unit, error) {
	if err := b.take(); err != nil {
		return nil, err
	}
	return b.Inner.ReadSection(ctx, d, h)
}
func (b *BoundedTools) Tree(ctx context.Context) (string, error) {
	if err := b.take(); err != nil {
		return "", err
	}
	return b.Inner.Tree(ctx)
}
