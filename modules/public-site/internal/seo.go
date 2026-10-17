// Package publicsite 实现公开链接的规范地址组合。
package publicsite

import (
	"context"
	"net/url"
	"strings"

	"nice-wiki/libs/platform"
)

// BuildCanonicalInput 是规范地址的已声明输入。baseURL 为已读取的 publicBaseURL。
type BuildCanonicalInput struct {
	BaseURL       string
	CanonicalPath string
}

// BuildCanonicalResult 是不含 query 与 fragment 的原始绝对规范 URL。
type BuildCanonicalResult struct {
	CanonicalURL string
}

// BuildCanonical 把 publicBaseURL 的 basePath 与调用方语言规范路径恰好拼接一次。
// 输入没有配置对象 ID，因此不另调 readConfigObject，只信任传入的基址。
func BuildCanonical(ctx context.Context, op platform.OperationContext, in BuildCanonicalInput) (BuildCanonicalResult, error) {
	_ = ctx
	_ = op
	base, err := parsePublicBase(in.BaseURL)
	if err != nil {
		return BuildCanonicalResult{}, err
	}
	if !validEncodedPath(in.CanonicalPath) {
		return BuildCanonicalResult{}, platform.NewError(platform.CodeValidation, false, "规范路径不合法")
	}
	joined, err := joinBaseOnce(base.EscapedPath(), in.CanonicalPath)
	if err != nil {
		return BuildCanonicalResult{}, err
	}
	absolute := *base
	absolute.RawQuery = ""
	absolute.Fragment = ""
	absolute.ForceQuery = false
	if err = setEncodedPath(&absolute, joined); err != nil {
		return BuildCanonicalResult{}, platform.NewError(platform.CodeValidation, false, "规范路径不合法")
	}
	if absolute.RawQuery != "" || absolute.Fragment != "" || absolute.User != nil {
		return BuildCanonicalResult{}, platform.NewError(platform.CodeValidation, false, "规范地址含查询或来源信息")
	}
	return BuildCanonicalResult{CanonicalURL: absolute.String()}, nil
}

func parsePublicBase(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, platform.NewError(platform.CodeValidation, false, "公开基址不合法")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, platform.NewError(platform.CodeValidation, false, "公开基址不合法")
	}
	basePath := strings.TrimRight(parsed.EscapedPath(), "/")
	if basePath == "" {
		basePath = "/"
	}
	if basePath != "/" && !validEncodedPath(basePath) {
		return nil, platform.NewError(platform.CodeValidation, false, "公开基址不合法")
	}
	if err = setEncodedPath(parsed, basePath); err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "公开基址不合法")
	}
	return parsed, nil
}

func pathOrRoot(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func joinBaseOnce(basePath, canonicalPath string) (string, error) {
	basePath = pathOrRoot(basePath)
	if basePath == "/" {
		return canonicalPath, nil
	}
	basePath = strings.TrimRight(basePath, "/")
	if canonicalPath == basePath || strings.HasPrefix(canonicalPath, basePath + "/") {
		return "", platform.NewError(platform.CodeValidation, false, "规范路径已含基址")
	}
	return basePath + canonicalPath, nil
}

func setEncodedPath(target *url.URL, encoded string) error {
	decoded, err := url.PathUnescape(encoded)
	if err != nil || decoded == "" {
		if err == nil {
			return errPathMismatch
		}
		return err
	}
	target.Path = decoded
	target.RawPath = encoded
	if target.EscapedPath() != encoded {
		return errPathMismatch
	}
	return nil
}

var errPathMismatch = platform.NewError(platform.CodeValidation, false, "规范路径编码不稳定")

func validEncodedPath(encoded string) bool {
	if !strings.HasPrefix(encoded, "/") || strings.ContainsAny(encoded, "?#\\\x00") {
		return false
	}
	for _, segment := range strings.Split(encoded, "/")[1:] {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "" || decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "/\\?#") {
			return false
		}
		for _, r := range decoded {
			if r <= 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}
