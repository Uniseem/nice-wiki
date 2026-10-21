package publicsite

import (
	"bytes"
	"context"
	"nice-wiki/libs/platform"
)

type HandlePreviewInput struct {
	PreviewID string
	Token     string
}
type HandlePreviewResult struct {
	Status  int
	Headers []Header
	HTML    string
}

// HandlePreview 每次刷新重新验证本人草稿权限，绝不把预览写入公开cache。
// 只放行同源脚本（公开页的交互岛与 KaTeX），预览看到的公式、代码复制等与访客一致。
func HandlePreview(ctx context.Context, op platform.OperationContext, in HandlePreviewInput) (HandlePreviewResult, error) {
	snapshot, err := ValidatePreviewToken(ctx, op, ValidatePreviewTokenInput{in.PreviewID, in.Token})
	if err != nil {
		return HandlePreviewResult{}, err
	}
	rendered, err := RenderPreview(ctx, op, RenderPreviewInput{in.PreviewID, in.Token, snapshot})
	if err != nil {
		return HandlePreviewResult{}, err
	}
	current, err := ValidatePreviewToken(ctx, op, ValidatePreviewTokenInput{in.PreviewID, in.Token})
	if err != nil {
		return HandlePreviewResult{}, err
	}
	if current.Metadata != snapshot.Metadata || !bytes.Equal(current.Body, snapshot.Body) {
		return HandlePreviewResult{}, platform.NewError(platform.CodeStaleResult, false, "预览草稿在渲染期间发生变化")
	}
	return HandlePreviewResult{200, []Header{{"Cache-Control", "no-store, private"}, {"X-Robots-Tag", "noindex, nofollow"}, {"Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https:; media-src 'self'; frame-src https:; frame-ancestors 'self'; base-uri 'none'"}, {"Referrer-Policy", "no-referrer"}}, rendered.HTML}, nil
}
