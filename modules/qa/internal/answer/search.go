package answer

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/admission"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type SearchDocumentsInput = gen.SearchDocumentsInput
type SearchDocumentsResult = gen.SearchDocumentsResult

// searchHit 是一条命中段落，与生成的 SearchDocumentsResult.Results 元素同型。
type searchHit = struct {
	Excerpt string `json:"excerpt"`
	Href    string `json:"href"`
	Title   string `json:"title"`
}

// SearchDocuments 是访客全文搜索（⌘K 里按回车）：消费一次 qa.ask 准入，复用问答的查询向量缓存，
// 在知识索引上做关键词 + 向量混合检索，不调用重排模型；返回命中段落的标题、带段落锚点的访客语言链接和节选。
// 节选是基准语言原文，访客语言不是基准语言时不返回节选，免得页面里混进另一种语言。结束时释放准入的并发占位。
// 搜索是只读的，不做幂等回执：客户端重试时换新的幂等键，入口会领新的准入。
func SearchDocuments(ctx context.Context, op OperationContext, input SearchDocumentsInput) (SearchDocumentsResult, error) {
	result := SearchDocumentsResult{Results: []searchHit{}}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return result, err
	}
	if runtime.Guard == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, false, "访客准入权威未装配")
	}
	release := func() {
		_, _ = runtime.Guard.ReleaseAdmission(qaruntime.ServiceReadContext(ctx, op), input.Admission.AdmissionId, struct{}{}, "qa.search.release."+platform.HashBytes([]byte(op.RootOperationID+":"+input.Admission.AdmissionId)))
	}
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return result, err
	}
	policy, err := qaruntime.ReadPolicy(read)
	read.Close()
	query := strings.TrimSpace(input.Query)
	switch {
	case err != nil:
	case !policy.Enabled:
		err = platform.NewError(platform.CodeForbidden, false, "公开问答尚未开启")
	case query == "" || utf8.RuneCountInString(query) > 200:
		err = platform.NewError(platform.CodeValidation, false, "搜索词为空或过长")
	}
	if err != nil {
		release()
		return result, err
	}
	permit := platform.AdmissionPermit{AdmissionID: input.Admission.AdmissionId, RequestID: input.Admission.RequestId, SubjectID: input.Admission.SubjectId, Action: input.Admission.Action, Token: input.Admission.Token, ExpiresAt: input.Admission.ExpiresAt.UTC().Format(time.RFC3339Nano), SessionBudgetScope: input.Admission.SessionBudgetScope, NetworkBudgetScope: input.Admission.NetworkBudgetScope, DataEpoch: input.Admission.DataEpoch, Audience: string(input.Admission.Audience)}
	validated, err := admission.ValidateAdmission(ctx, op, admission.ValidateAdmissionInput{Admission: permit, Action: "qa.ask"})
	if err != nil {
		return result, err
	}
	defer release()
	// 和问答轮次一样，发模型前固定可信费用类别为 qa：查询向量按问答档位计费。
	op.CostClass = platform.CostClassQA
	runtime.SetPermit(validated.Permit)
	ctx = qaruntime.WithRuntime(ctx, runtime)
	canonical, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: query, Language: runtime.BaseLanguage})
	if err != nil {
		return result, err
	}
	found, err := RetrieveEvidence(ctx, op, RetrieveEvidenceInput{TurnID: "search." + op.RootOperationID, Normalized: canonical.Normalized, Policy: policy, Language: input.Language, SkipRerank: true})
	if err != nil {
		return result, err
	}
	var materials []struct {
		CitationID string `json:"citationId"`
		Text       string `json:"text"`
	}
	_ = json.Unmarshal([]byte(found.EvidenceText), &materials)
	texts := make(map[string]string, len(materials))
	for _, material := range materials {
		texts[material.CitationID] = material.Text
	}
	for _, citation := range found.Citations {
		excerpt := ""
		if input.Language == runtime.BaseLanguage {
			excerpt = clipExcerpt(texts[citation.CitationId], 240)
		}
		result.Results = append(result.Results, searchHit{excerpt, citation.Href, citation.Title})
	}
	return result, nil
}

// clipExcerpt 把段落压成一行，超过 limit 个字符时截断并加省略号。
func clipExcerpt(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "…"
}
