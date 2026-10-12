package content

import "context"

// 后续模型功能以接口注入，基础文档管理不依赖模型实现。
type SlugGenerator interface {
	GenerateSlug(context.Context, string) (string, error)
}
type SEOGenerator interface {
	GenerateDescription(context.Context, File) (string, error)
}
type History interface {
	Commit(context.Context, string, string, []string) (string, error)
}
type PublishedHook func(context.Context, string) error
