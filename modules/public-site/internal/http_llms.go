package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
	// 导航只建一次；缓存里还有效的篇直接用，其余并发渲染（每篇要读正文、资源清单和译文），再按导航顺序拼接。
	pages := make([]string, len(navigation.Tree))
	var size atomic.Int64
	missing := make([]int, 0)
	for i, node := range navigation.Tree {
		if text, ok := llmsFullCache.get(siteKey(ctx)+"|"+in.Locale, node, navigation.Revision); ok {
			pages[i] = text
			size.Add(int64(len(text)))
		} else {
			missing = append(missing, i)
		}
	}
	var failed atomic.Pointer[error]
	next := make(chan int)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range next {
				page, err := renderNodeDetail(ctx, op, in.Locale, true, navigation, navigation.Tree[i])
				if err != nil {
					failed.CompareAndSwap(nil, &err)
					continue
				}
				pages[i] = strings.TrimSpace(page.text)
				size.Add(int64(len(pages[i])))
				llmsFullCache.put(siteKey(ctx)+"|"+in.Locale, navigation.Tree[i], navigation.Revision, page.linked, page.translated, pages[i])
			}
		}()
	}
	for _, i := range missing {
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
	llmsFullCache.keepOnly(siteKey(ctx)+"|"+in.Locale, navigation.Tree)
	return HandleLLMsFullResult{Status: 200, Text: strings.Join(pages, "\n\n---\n\n") + "\n"}, nil
}

// llmsFullCache 记着各语言每篇已渲染的 Markdown，llms-full 只重渲变化的篇。
// 发布修订变了、文档撤回（不在导航里）立即失效；含站内链接的篇随导航版本失效（链接目标可能改名或撤回）；
// 用了译文的篇 10 分钟后重渲（译文会在发布之后陆续更新）。
var llmsFullCache = &renderedPageCache{pages: map[string]cachedPage{}}

const translatedPageTTL = 10 * time.Minute

type cachedPage struct {
	publishedRevisionID string
	navigationRevision  int
	linked              bool
	expires             time.Time
	text                string
}

type renderedPageCache struct {
	mu    sync.Mutex
	pages map[string]cachedPage
}

func (c *renderedPageCache) get(locale string, node NavigationNode, navigationRevision int) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	page, ok := c.pages[locale+"\x00"+node.DocumentID]
	if !ok || page.publishedRevisionID != node.Publication.PublishedRevisionId || (page.linked && page.navigationRevision != navigationRevision) || (!page.expires.IsZero() && time.Now().After(page.expires)) {
		return "", false
	}
	return page.text, true
}

func (c *renderedPageCache) put(locale string, node NavigationNode, navigationRevision int, linked, translated bool, text string) {
	page := cachedPage{publishedRevisionID: node.Publication.PublishedRevisionId, navigationRevision: navigationRevision, linked: linked, text: text}
	if translated {
		page.expires = time.Now().Add(translatedPageTTL)
	}
	c.mu.Lock()
	c.pages[locale+"\x00"+node.DocumentID] = page
	c.mu.Unlock()
}

// keepOnly 丢掉本语言已不在公开导航里的篇，缓存大小随公开文档数有界。
func (c *renderedPageCache) keepOnly(locale string, tree []NavigationNode) {
	visible := make(map[string]bool, len(tree))
	for _, node := range tree {
		visible[locale+"\x00"+node.DocumentID] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.pages {
		if strings.HasPrefix(key, locale+"\x00") && !visible[key] {
			delete(c.pages, key)
		}
	}
}
