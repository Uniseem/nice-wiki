package doc

// LinkResolver 由 site 注入，基础渲染层不依赖文档仓库。
type LinkResolver interface {
	ResolveDocument(documentID, blockID, language string) (string, bool)
}
type RenderOptions struct {
	Language string
	Links    LinkResolver
	AssetURL func(string, string, string) string
}
