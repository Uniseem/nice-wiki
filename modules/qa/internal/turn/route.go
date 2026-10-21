package turn

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/answer"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

// 路由和材料筛选的取用规则（见 docs/decision-model.md）。
const (
	noneConfidence     = 0.5 // “都不相关”的概率到这个值就不按文档路由
	routeCumulative    = 0.9 // 按概率从高到低取文档，累计到这里为止
	routeMinimum       = 0.1 // 单个选项概率低于它就不取
	routeMaxOptions    = 3   // 最多取几个选项（主要 + 补充）
	routeMaxDocuments  = 20  // 交给检索的文档数上限
	evidenceKeep       = 0.5 // 材料“有用”的概率到这个值才保留
	evidenceBestEffort = 0.2 // 没有一段达标时，最好的一段到这个值仍保留
)

// documentQuestion 把路由视图变成一道单选题：选项是文档或折叠分区的概括，另加“都不相关”。
func documentQuestion(entries []qaruntime.DocumentIndexEntry) (platform.DecisionQuestion, map[string]qaruntime.DocumentIndexEntry) {
	criteria := map[string]string{"none": "以上文档都和这个问题无关"}
	byKey := map[string]qaruntime.DocumentIndexEntry{}
	for _, entry := range entries {
		key := entry.DocumentID
		if entry.Kind == "section" {
			key = "section:" + entry.DocumentID
		}
		criteria[key] = entry.Text
		byKey[key] = entry
	}
	return platform.DecisionQuestion{Type: "choice", Instructions: "要回答 question，最该读哪篇文档？选项是本站文档（或一组文档）的概括。", Criteria: criteria}, byKey
}

// pickDocuments 按概率取文档：最高的作主要文档，累计到 0.9 为止的作补充；最高的是折叠分区时返回该分区，由调用方展开再问。
func pickDocuments(answer platform.DecisionAnswer, byKey map[string]qaruntime.DocumentIndexEntry) (documents []string, expand string) {
	type option struct {
		key         string
		probability float64
	}
	options := make([]option, 0, len(answer.Probabilities))
	for key, probability := range answer.Probabilities {
		options = append(options, option{key, probability})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].probability > options[j].probability })
	if len(options) == 0 || (options[0].key == "none" && options[0].probability >= noneConfidence) {
		return nil, ""
	}
	seen := map[string]bool{}
	cumulative := 0.0
	picked := 0
	for _, choice := range options {
		entry, ok := byKey[choice.key]
		if !ok || choice.probability < routeMinimum || picked >= routeMaxOptions || cumulative >= routeCumulative {
			continue
		}
		if picked == 0 && entry.Kind == "section" {
			return nil, entry.DocumentID
		}
		for _, id := range entry.DocumentIDs {
			if !seen[id] && len(documents) < routeMaxDocuments {
				seen[id] = true
				documents = append(documents, id)
			}
		}
		cumulative += choice.probability
		picked++
	}
	return documents, ""
}

// routeDocuments 单独问一次文档路由（追问改写之后，或展开选中的分区）；最多展开两层，失败时返回 nil（只做全站检索）。
func routeDocuments(ctx context.Context, op OperationContext, question, root string, depth int) []string {
	documents, err := askRoute(ctx, op, question, root, depth)
	if err != nil && os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "文档路由不可用 err=%v\n", err)
	}
	return documents
}

func askRoute(ctx context.Context, op OperationContext, question, root string, depth int) ([]string, error) {
	entries, err := qaruntime.ReadDocumentIndex(ctx, op, root)
	if err != nil || len(entries) == 0 {
		return nil, err
	}
	profile, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.decide")
	if err != nil {
		return nil, err
	}
	routing, byKey := documentQuestion(entries)
	answers, err := decide(ctx, op, profile, "route."+root, map[string]any{"question": question}, map[string]platform.DecisionQuestion{"document": routing})
	if err != nil {
		return nil, err
	}
	documents, expand := pickDocuments(answers["document"], byKey)
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "文档路由 层级=%d 选项=%d 文档=%d 展开分区=%q\n", depth, len(entries), len(documents), expand)
	}
	if expand != "" && depth < 2 {
		return askRoute(ctx, op, question, expand, depth+1)
	}
	if expand != "" {
		return byKey["section:"+expand].DocumentIDs[:min(len(byKey["section:"+expand].DocumentIDs), routeMaxDocuments)], nil
	}
	return documents, nil
}

// decide 发一次决策调用（4 秒内），返回各题答案。
func decide(ctx context.Context, op OperationContext, profile, key string, state any, questions map[string]platform.DecisionQuestion) (map[string]platform.DecisionAnswer, error) {
	work, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	outcome, err := qaruntime.InvokeModel(work, op, qaruntime.InvokeModelInput{Request: platform.CreateInvocationInput{
		TaskID: "qa.decide", ProfileID: platform.Identifier(profile), Capability: platform.CapabilityDecision, IDempotencyKey: key,
		Messages: []platform.GatewayMessage{}, Texts: []string{}, CandidateIDs: []platform.Identifier{},
		Decision: &platform.DecisionRequest{State: state, Questions: questions},
	}})
	if err != nil {
		return nil, err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "决策结果未完成")
	}
	return outcome.CurrentSnapshot.Output.Decision, nil
}

// evidenceSelector 返回材料筛选函数：每段候选材料一道是非题“有没有回答需要的信息”，只留有用的。
// 决策模型不可用时返回 nil，检索结果原样使用。
func evidenceSelector(ctx context.Context, op OperationContext, question string) func([]answer.EvidenceCandidate) []string {
	profile, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.decide")
	if err != nil {
		return nil
	}
	return func(candidates []answer.EvidenceCandidate) []string {
		questions := make(map[string]platform.DecisionQuestion, len(candidates))
		for i, candidate := range candidates {
			questions[fmt.Sprintf("c%d", i)] = platform.DecisionQuestion{Type: "noul", Instructions: "这段材料里有回答 question 需要的信息吗？\n材料：" + clip(candidate.Text, 1500)}
		}
		answers, err := decide(ctx, op, profile, "evidence", map[string]any{"question": question}, questions)
		if err != nil {
			if os.Getenv("NW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "材料筛选不可用，原样使用 err=%v\n", err)
			}
			ids := make([]string, len(candidates))
			for i, candidate := range candidates {
				ids[i] = candidate.ChunkID
			}
			return ids
		}
		kept := []string{}
		best, bestID := 0.0, ""
		for i, candidate := range candidates {
			value := answers[fmt.Sprintf("c%d", i)]
			if value.Noul == nil {
				continue
			}
			if *value.Noul >= evidenceKeep {
				kept = append(kept, candidate.ChunkID)
			}
			if *value.Noul > best {
				best, bestID = *value.Noul, candidate.ChunkID
			}
		}
		if len(kept) == 0 && best >= evidenceBestEffort {
			kept = append(kept, bestID)
		}
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "材料筛选 候选=%d 保留=%d\n", len(candidates), len(kept))
		}
		return kept
	}
}
