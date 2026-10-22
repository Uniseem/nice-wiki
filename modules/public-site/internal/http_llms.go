package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
	"strings"
	"sync"
	"sync/atomic"
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
	// 导航只建一次，各篇 Markdown 并发渲染（每篇要读正文、资源清单和译文），再按导航顺序拼接。
	pages := make([]string, len(navigation.Tree))
	var size atomic.Int64
	var failed atomic.Pointer[error]
	next := make(chan int)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range next {
				page, err := renderNode(ctx, op, in.Locale, true, navigation, navigation.Tree[i])
				if err != nil {
					failed.CompareAndSwap(nil, &err)
					continue
				}
				pages[i] = strings.TrimSpace(page)
				size.Add(int64(len(pages[i])))
			}
		}()
	}
	for i := range navigation.Tree {
		if failed.Load() != nil || size.Load() > llmsFullLimit {
			break
		}
		next <- i
	}
	close(next)
	workers.Wait()
	if err := failed.Load(); err != nil {
		return HandleLLMsFullResult{}, *err
	}
	if size.Load() > llmsFullLimit {
		return HandleLLMsFullResult{}, platform.NewError(platform.CodeValidation, false, "LLMS_FULL_TOO_LARGE")
	}
	return HandleLLMsFullResult{Status: 200, Text: strings.Join(pages, "\n\n---\n\n") + "\n"}, nil
}
