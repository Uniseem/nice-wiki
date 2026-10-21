package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
	"strings"
)

type HandleLLMsIndexInput struct {
	Locale    string
	DataEpoch string
}
type HandleLLMsIndexResult struct {
	Status int
	Text   string
}

// HandleLLMsIndex 禁索引时 404；公开摘要只能取同次权威快照。
func HandleLLMsIndex(ctx context.Context, op platform.OperationContext, in HandleLLMsIndexInput) (HandleLLMsIndexResult, error) {
	enabled, err := readIndexingPolicy(ctx)
	if err != nil {
		return HandleLLMsIndexResult{}, err
	}
	if !enabled {
		return HandleLLMsIndexResult{Status: 404, Text: ""}, nil
	}
	index, err := BuildLLMsIndex(ctx, op, BuildLLMsIndexInput{in.Locale, string(in.DataEpoch), publishedBodyLimit})
	if err != nil {
		return HandleLLMsIndexResult{}, err
	}
	if enabled, err = readIndexingPolicy(ctx); err != nil {
		return HandleLLMsIndexResult{}, err
	} else if !enabled {
		return HandleLLMsIndexResult{Status: 404, Text: ""}, nil
	}
	return HandleLLMsIndexResult{Status: 200, Text: index.Text}, nil
}

type HandleLLMsFullInput struct {
	Locale    string
	DataEpoch string
}
type HandleLLMsFullResult struct {
	Status int
	Text   string
}

const llmsFullLimit = 4 << 20

// HandleLLMsFull 把本语言全部公开文档的 Markdown 正文按导航顺序拼成一份文本；禁索引时 404，总量超限时报错而不是截断。
func HandleLLMsFull(ctx context.Context, op platform.OperationContext, in HandleLLMsFullInput) (HandleLLMsFullResult, error) {
	enabled, err := readIndexingPolicy(ctx)
	if err != nil {
		return HandleLLMsFullResult{}, err
	}
	if !enabled {
		return HandleLLMsFullResult{Status: 404}, nil
	}
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return HandleLLMsFullResult{}, err
	}
	var output strings.Builder
	prefix := "/" + in.Locale + "/"
	for _, node := range navigation.Tree {
		slug := strings.TrimPrefix(node.CanonicalPath, prefix)
		page, err := HandlePage(ctx, op, HandlePageInput{Locale: in.Locale, Slug: slug + ".md", DataEpoch: in.DataEpoch})
		if err != nil {
			return HandleLLMsFullResult{}, err
		}
		if output.Len() > 0 {
			output.WriteString("\n\n---\n\n")
		}
		output.WriteString(strings.TrimSpace(page.HTML))
		if output.Len() > llmsFullLimit {
			return HandleLLMsFullResult{}, platform.NewError(platform.CodeValidation, false, "LLMS_FULL_TOO_LARGE")
		}
	}
	return HandleLLMsFullResult{Status: 200, Text: output.String() + "\n"}, nil
}
