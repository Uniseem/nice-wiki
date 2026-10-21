// 供应方接入参照 opencode（MIT，Copyright (c) 2025 opencode，https://github.com/anomalyco/opencode
// 4d5965762ec5）的做法：供应方和模型都来自 models.dev 目录，接入时只填密钥，模型从目录带出名称、上下文和模态。
// 这里用 Go 重写，未复制其源码；另外先问供应方自己的模型列表，只把账号真正可用的模型排在前面。
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/catalog/bundled"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/security"
)

type ReadProviderCatalogInput struct{}
type ReadProviderCatalogResult = gen.ReadProviderCatalogResult
type DiscoverProviderModelsInput struct {
	ProviderId string `json:"providerId"`
}
type DiscoverProviderModelsResult = gen.DiscoverProviderModelsResult

// 国内常用的排前面，其后是 opencode 的热门顺序。
var popularProviders = []string{"alibaba-cn", "deepseek", "moonshotai-cn", "zhipuai", "siliconflow-cn", "volcengine", "openrouter", "openai", "anthropic", "google"}

// 名字一看就不是文本对话/向量的模型（语音、实时、图像或视频生成），目录里也没有文本输出时不列。
var nonTextModel = regexp.MustCompile(`(?i)(^|[-_./])(tts|asr|realtime|audio|livetranslate|image|wanx|wan2|video|paraformer|cosyvoice|sambert|whisper|dall-e|sora)([-_./]|$)`)

type directoryModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Reasoning bool   `json:"reasoning"`
	Limit     struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Modalities struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"modalities"`
}

type directoryProvider struct {
	Name    string                    `json:"name"`
	API     string                    `json:"api"`
	NPM     string                    `json:"npm"`
	Env     []string                  `json:"env"`
	Models  map[string]directoryModel `json:"models"`
	adapter string
	baseURL string
}

var directory struct {
	once      sync.Once
	providers map[string]*directoryProvider
	err       error
}

// loadDirectory 解析内置 models.dev 快照，进程内只做一次；只留本网关协议能直接调用的供应方。
func loadDirectory() (map[string]*directoryProvider, error) {
	directory.once.Do(func() {
		raw, _, err := bundled.ReadSnapshot()
		if err != nil {
			directory.err = err
			return
		}
		var all map[string]*directoryProvider
		if json.Unmarshal(raw, &all) != nil {
			directory.err = platform.NewError(platform.CodeIntegrityError, false, "内置模型目录不可读")
			return
		}
		directory.providers = make(map[string]*directoryProvider, len(all))
		for id, entry := range all {
			entry.adapter, entry.baseURL = directoryProtocol(entry)
			if entry.adapter != "" && entry.baseURL != "" {
				directory.providers[id] = entry
			}
		}
	})
	return directory.providers, directory.err
}

// directoryProtocol 把目录里的 AI SDK 包名换成本网关协议和接口根地址。
func directoryProtocol(entry *directoryProvider) (string, string) {
	api := strings.TrimRight(entry.API, "/")
	if api != "" && !strings.HasPrefix(api, "https://") {
		return "", ""
	}
	pick := func(fallback string) string {
		if api != "" {
			return api
		}
		return fallback
	}
	switch entry.NPM {
	case "@ai-sdk/anthropic":
		return "anthropic_messages", pick("https://api.anthropic.com/v1")
	case "@ai-sdk/google":
		return "gemini", pick("https://generativelanguage.googleapis.com/v1beta")
	case "@ai-sdk/openai":
		return "openai_chat", pick("https://api.openai.com/v1")
	case "@ai-sdk/openai-compatible", "@openrouter/ai-sdk-provider", "@ai-sdk/groq", "@ai-sdk/cerebras", "@ai-sdk/deepinfra":
		return "openai_chat", api
	}
	return "", ""
}

// ReadProviderCatalog 列出可直接接入的供应方：热门在前，其余按名称。
func ReadProviderCatalog(ctx context.Context, op OperationContext, input ReadProviderCatalogInput) (ReadProviderCatalogResult, error) {
	providers, err := loadDirectory()
	if err != nil {
		return ReadProviderCatalogResult{}, err
	}
	rank := make(map[string]int, len(popularProviders))
	for i, id := range popularProviders {
		rank[id] = i + 1
	}
	result := ReadProviderCatalogResult{Providers: make([]gen.CatalogProvider, 0, len(providers))}
	for id, entry := range providers {
		name := entry.Name
		if name == "" {
			name = id
		}
		keys := make([]string, 0, len(entry.Env))
		for _, key := range entry.Env {
			if key != "" && len(keys) < 16 {
				keys = append(keys, key)
			}
		}
		result.Providers = append(result.Providers, gen.CatalogProvider{CatalogId: id, Name: name, BaseUrl: entry.baseURL, Adapter: gen.CatalogProviderAdapter(entry.adapter), KeyNames: keys, ModelCount: len(entry.Models), Popular: rank[id]})
	}
	sort.Slice(result.Providers, func(i, j int) bool {
		a, b := result.Providers[i], result.Providers[j]
		if (a.Popular == 0) != (b.Popular == 0) {
			return a.Popular != 0
		}
		if a.Popular != b.Popular {
			return a.Popular < b.Popular
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return result, nil
}

type listedModel struct {
	id, name        string
	context, output int
	methods         []string
}

// DiscoverProviderModels 先问供应方自己的模型列表，再用目录补全；列表拿不到时退回目录里的模型。
func DiscoverProviderModels(ctx context.Context, op OperationContext, input DiscoverProviderModelsInput) (DiscoverProviderModelsResult, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return DiscoverProviderModelsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return DiscoverProviderModelsResult{}, availabilityReadError(err)
	}
	var adapter, baseURL string
	err = tx.QueryRowContext(ctx, `SELECT adapter, base_url FROM providers WHERE provider_id = ? AND data_epoch = ?`, input.ProviderId, op.DataEpoch).Scan(&adapter, &baseURL)
	_ = tx.Rollback()
	if errors.Is(err, sql.ErrNoRows) {
		return DiscoverProviderModelsResult{}, platform.NewError(platform.CodeNotFound, false, "供应方不存在")
	}
	if err != nil {
		return DiscoverProviderModelsResult{}, availabilityReadError(err)
	}
	providers, err := loadDirectory()
	if err != nil {
		return DiscoverProviderModelsResult{}, err
	}
	result := DiscoverProviderModelsResult{Models: []gen.AvailableModel{}}
	var entry *directoryProvider
	base := strings.TrimRight(baseURL, "/")
	for id, candidate := range providers {
		if strings.EqualFold(candidate.baseURL, base) && (entry == nil || id < *result.CatalogId) {
			entry, result.CatalogId = candidate, &id
		}
	}
	listed, problemText := listProviderModels(ctx, op, input.ProviderId, adapter, base)
	result.Listed = problemText == ""
	if !result.Listed {
		result.ListProblem = &problemText
	}
	seen := map[string]bool{}
	add := func(id string, fromProvider bool, extra listedModel) {
		if seen[id] || len(result.Models) >= 5000 || id == "" || len(id) > 512 {
			return
		}
		var known *directoryModel
		if entry != nil {
			if model, ok := entry.Models[id]; ok {
				known = &model
			}
		}
		capabilities, text := inferCapabilities(id, known, extra.methods)
		if !text {
			return
		}
		seen[id] = true
		model := gen.AvailableModel{ProviderModelId: id, Name: id, Capabilities: capabilities, InCatalog: known != nil, ListedByProvider: fromProvider}
		if extra.name != "" {
			model.Name = extra.name
		}
		window, output := extra.context, extra.output
		if known != nil {
			if known.Name != "" {
				model.Name = known.Name
			}
			if window == 0 {
				window = known.Limit.Context
			}
			if output == 0 {
				output = known.Limit.Output
			}
		}
		if len(model.Name) > 512 {
			model.Name = id
		}
		if window > 0 {
			model.ContextWindow = &window
		}
		if output > 0 {
			model.MaxOutputTokens = &output
		}
		result.Models = append(result.Models, model)
	}
	for _, model := range listed {
		add(model.id, true, model)
	}
	if entry != nil {
		ids := make([]string, 0, len(entry.Models))
		for id := range entry.Models {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			add(id, false, listedModel{})
		}
	}
	sort.SliceStable(result.Models, func(i, j int) bool {
		a, b := result.Models[i], result.Models[j]
		if a.ListedByProvider != b.ListedByProvider {
			return a.ListedByProvider
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return result, nil
}

// inferCapabilities 按目录模态和模型名推断能力；第二个返回值为 false 表示不是文本或向量模型。
func inferCapabilities(id string, known *directoryModel, methods []string) ([]gen.AvailableModelCapabilities, bool) {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "rerank"):
		return []gen.AvailableModelCapabilities{gen.AvailableModelCapabilitiesRerank}, true
	case strings.Contains(lower, "embed") || (slices.Contains(methods, "embedContent") && !slices.Contains(methods, "generateContent")):
		return []gen.AvailableModelCapabilities{gen.AvailableModelCapabilitiesEmbedding}, true
	}
	if known != nil && len(known.Modalities.Output) > 0 && !slices.Contains(known.Modalities.Output, "text") {
		return nil, false
	}
	if known == nil && nonTextModel.MatchString(lower) {
		return nil, false
	}
	capabilities := []gen.AvailableModelCapabilities{gen.AvailableModelCapabilitiesChat}
	vision := known != nil && slices.Contains(known.Modalities.Input, "image")
	if known == nil {
		vision = strings.Contains(lower, "-vl") || strings.Contains(lower, "vision")
	}
	if vision {
		capabilities = append(capabilities, gen.AvailableModelCapabilitiesVision)
	}
	if known != nil && known.Reasoning {
		capabilities = append(capabilities, gen.AvailableModelCapabilitiesReasoning)
	}
	return capabilities, true
}

// listProviderModels 用当前密钥请求供应方的模型列表；失败只返回原因（不含响应正文和密钥）。
func listProviderModels(ctx context.Context, op OperationContext, providerID, adapter, base string) ([]listedModel, string) {
	key, err := security.OpenProviderCredential(ctx, op, providerID)
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			return nil, "还没有设置密钥"
		}
		return nil, "密钥读取失败"
	}
	defer clear(key)
	if strings.ContainsAny(string(key), "\r\n\x00") {
		return nil, "密钥格式不对"
	}
	target := base + "/models"
	headers := http.Header{"Accept": {"application/json"}}
	switch adapter {
	case "anthropic_messages":
		target += "?limit=1000"
		headers.Set("x-api-key", string(key))
		headers.Set("anthropic-version", "2023-06-01")
	case "gemini", "gemini_embeddings":
		target += "?pageSize=1000"
		headers.Set("x-goog-api-key", string(key))
	default:
		headers.Set("Authorization", "Bearer "+string(key))
	}
	work, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(work, http.MethodGet, target, nil)
	if err != nil {
		return nil, "接口地址不合法"
	}
	request.Header = headers
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Timeout() {
			return nil, "请求超时"
		}
		return nil, "连不上供应方"
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("供应方返回 HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, "模型列表太大或读取中断"
	}
	var body struct {
		Data []struct {
			ID            string `json:"id"`
			DisplayName   string `json:"display_name"`
			ContextLength int    `json:"context_length"`
			TopProvider   struct {
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
		Models []struct {
			Name        string   `json:"name"`
			DisplayName string   `json:"displayName"`
			Input       int      `json:"inputTokenLimit"`
			Output      int      `json:"outputTokenLimit"`
			Methods     []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if json.Unmarshal(raw, &body) != nil || (len(body.Data) == 0 && len(body.Models) == 0) {
		return nil, "供应方没有返回模型列表"
	}
	models := make([]listedModel, 0, len(body.Data)+len(body.Models))
	for _, item := range body.Data {
		// OpenRouter、SiliconFlow 等会在列表里直接给上下文长度，优先于目录。
		models = append(models, listedModel{id: item.ID, name: item.DisplayName, context: max(item.ContextLength, 0), output: max(item.TopProvider.MaxCompletionTokens, 0)})
	}
	for _, item := range body.Models {
		models = append(models, listedModel{id: strings.TrimPrefix(item.Name, "models/"), name: item.DisplayName, context: item.Input, output: item.Output, methods: item.Methods})
	}
	return models, ""
}

// NewDiscoveryHandlers 绑定供应方目录与模型拉取两个只读后台接口。
func NewDiscoveryHandlers() map[string]http.Handler {
	handlers := make(map[string]http.Handler, 2)
	for _, operationID := range []string{"readProviderCatalog", "discoverProviderModels"} {
		handlers[operationID] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var result any
			var err error
			if operationID == "readProviderCatalog" {
				result, err = ReadProviderCatalog(r.Context(), op, ReadProviderCatalogInput{})
			} else {
				result, err = DiscoverProviderModels(r.Context(), op, DiscoverProviderModelsInput{ProviderId: r.PathValue("providerId")})
			}
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		})
	}
	return handlers
}
