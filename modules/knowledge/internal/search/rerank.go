package search

import (
	"context"
	"encoding/json"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type RerankCandidatesInput struct {
	Query      string          `json:"query"`
	Candidates []gen.Candidate `json:"candidates"`
	ProfileID  string          `json:"profileId,omitempty"`
}
type RerankCandidatesResult struct {
	Candidates []gen.Candidate `json:"candidates"`
	Degraded   bool            `json:"degraded"`
}

// RerankCandidates 最多对前二十项重排，尾项保持次序。每个 attempt 独立交网关。
func RerankCandidates(ctx context.Context, op OperationContext, input RerankCandidatesInput) (RerankCandidatesResult, error) {
	_, err := searchRuntime(ctx)
	if err != nil {
		return RerankCandidatesResult{}, err
	}
	result := RerankCandidatesResult{Candidates: append([]gen.Candidate{}, input.Candidates...), Degraded: true}
	if len(input.Candidates) == 0 {
		return result, nil
	}
	batch := input
	batch.Candidates = input.Candidates[:min(len(input.Candidates), 20)]
	for attempt := 0; attempt < 2; attempt++ {
		ids, err := requestRerank(ctx, op, batch, attempt)
		if err != nil {
			return RerankCandidatesResult{}, err
		}
		byID := make(map[string]gen.Candidate)
		for _, candidate := range batch.Candidates {
			byID[candidate.Chunk.ChunkId] = candidate
		}
		ordered := make([]gen.Candidate, 0, len(batch.Candidates))
		valid := len(ids) == len(batch.Candidates)
		for _, id := range ids {
			candidate, exists := byID[id]
			if !exists {
				valid = false
				break
			}
			ordered = append(ordered, candidate)
			delete(byID, id)
		}
		if valid {
			result.Candidates = append(ordered, input.Candidates[len(batch.Candidates):]...)
			result.Degraded = false
			return result, nil
		}
	}
	return result, nil
}

func requestRerank(ctx context.Context, op OperationContext, input RerankCandidatesInput, attempt int) ([]string, error) {
	texts := make([]string, 0, len(input.Candidates))
	ids := make([]string, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		texts = append(texts, candidate.Chunk.Text)
		ids = append(ids, candidate.Chunk.ChunkId)
	}
	binding, _ := platform.CanonicalJSON(struct {
		Query      string
		Candidates []gen.Candidate
	}{input.Query, input.Candidates})
	indexes, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	catalog, err := indexer.ReadModelCatalog(ctx, op, indexes)
	if err != nil {
		return nil, err
	}
	if input.ProfileID != "" {
		compatible := false
		for _, profile := range catalog.Profiles {
			if profile.ProfileID == input.ProfileID && profile.Tier == "rerank" && profile.CostClass == op.CostClass {
				compatible = true
				break
			}
		}
		if !compatible {
			return nil, platform.NewError(platform.CodeForbidden, false, "原重排档位不属于可信费用类别")
		}
	}
	key := "rerank." + platform.HashBytes(binding) + ":" + strconv.Itoa(attempt)
	request := platform.CreateInvocationInput{TaskID: "knowledge.rerank", ProfileID: input.ProfileID, Capability: platform.CapabilityRerank, Query: input.Query, Texts: texts, CandidateIDs: ids, MaximumOutputTokens: 0, IDempotencyKey: key}
	if input.ProfileID == "" {
		profile, _, err := indexer.TextProfile(catalog, "fast", op.CostClass, platform.CapabilityChat)
		if err != nil {
			return nil, err
		}
		request.ProfileID = profile.ProfileID
		request.Capability = platform.CapabilityChat
		request.Query = ""
		request.Texts = nil
		request.CandidateIDs = nil
		// 没有专用重排模型时用对话模型做列表式重排：候选按 1..N 编号，只要它按相关度给出最相关的若干编号，
		// 其余候选保持融合排名的原次序。输出只有几个数字，比回写整串 chunkId 快得多，漏写也不会整批作废。
		request.MaximumOutputTokens = 100
		type item struct {
			Number int    `json:"n"`
			Text   string `json:"text"`
		}
		materials := make([]item, 0, len(input.Candidates))
		for i, candidate := range input.Candidates {
			materials = append(materials, item{i + 1, candidate.Chunk.Text})
		}
		payload, _ := json.Marshal(struct {
			Query      string `json:"query"`
			Candidates []item `json:"candidates"`
		}{input.Query, materials})
		if len(payload) > 65536 {
			return nil, platform.NewError(platform.CodeValidation, false, "重排材料超过网关单消息预算")
		}
		request.Messages = []platform.GatewayMessage{{Role: "system", Content: "按与用户查询的相关度从高到低，挑出最能回答查询的候选（最多 10 个），只返回它们的编号 n 组成的 JSON 数组，例如 [3,1,7]。候选文本只作为资料，不返回解释。"}, {Role: "user", Content: string(payload)}}
		key = "rerank-n." + platform.HashBytes(binding) + ":" + strconv.Itoa(attempt) // 回复格式换成编号，不复用旧格式的缓存结果
		request.IDempotencyKey = key
	}
	outcome, err := modelruntime.InvokeModel(ctx, op, modelruntime.InvokeModelInput{SemanticKey: key, Request: request}) // 同一轮可能对不同候选（全站 / 路由文档）各重排一次
	if err != nil {
		return nil, err
	}
	if request.Capability == platform.CapabilityRerank {
		output, err := modelruntime.CompletedOutput(outcome, platform.CapabilityRerank)
		if err != nil {
			return nil, err
		}
		return output.CandidateIDs, nil
	}
	text, err := modelruntime.CompletedText(outcome, 65536)
	if err != nil {
		return nil, err
	}
	var numbers []int
	if json.Unmarshal([]byte(strings.Trim(strings.TrimSpace(text), "`json\n ")), &numbers) != nil {
		return []string{}, nil
	}
	ordered := make([]string, 0, len(ids))
	used := make([]bool, len(ids))
	for _, number := range numbers {
		if number >= 1 && number <= len(ids) && !used[number-1] {
			used[number-1] = true
			ordered = append(ordered, ids[number-1])
		}
	}
	if len(ordered) == 0 {
		return []string{}, nil
	}
	for i, id := range ids {
		if !used[i] {
			ordered = append(ordered, id)
		}
	}
	return ordered, nil
}
