package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
	"reflect"
	"strings"
)

type BuildLLMsIndexInput struct {
	Locale    string
	DataEpoch string
	MaxBytes  int64
}
type BuildLLMsIndexResult struct {
	Text           string
	SourceRevision int
}

// BuildLLMsIndex 从精确公开版本读取摘要，输出前再次核对所有公开引用。
func BuildLLMsIndex(ctx context.Context, op platform.OperationContext, in BuildLLMsIndexInput) (BuildLLMsIndexResult, error) {
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return BuildLLMsIndexResult{}, err
	}
	// 摘要取公开树同一快照里的搜索摘要，不逐篇读正文。
	var output strings.Builder
	output.WriteString("# 文档索引\n\n")
	for _, node := range navigation.Tree {
		summary := node.SEODescription
		output.WriteString("- [" + markdownLabel(node.Title) + "](" + node.CanonicalPath + ".md): " + strings.Join(strings.Fields(summary), " ") + "\n")
		if int64(output.Len()) > in.MaxBytes {
			return BuildLLMsIndexResult{}, platform.NewError(platform.CodeValidation, false, "INDEX_TOO_LARGE")
		}
	}
	again, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return BuildLLMsIndexResult{}, err
	}
	if again.Revision != navigation.Revision || !reflect.DeepEqual(again.Tree, navigation.Tree) {
		return BuildLLMsIndexResult{}, stale()
	}
	return BuildLLMsIndexResult{output.String(), navigation.Revision}, nil
}

func markdownLabel(value string) string {
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "\n", " ", "\r", " ").Replace(value)
}
