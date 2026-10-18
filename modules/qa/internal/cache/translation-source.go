package cache

import (
	"context"
	"nice-wiki/libs/platform"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type ReadCacheTranslationSourceInput struct {
	CacheId string `json:"cacheId"`
}
type ReadCacheTranslationSourceResult struct {
	CacheId        string `json:"cacheId"`
	AnswerRevision int    `json:"answerRevision"`
	SourceHash     string `json:"sourceHash"`
	State          string `json:"state"`
	Eligible       bool   `json:"eligible"`
	Question       string `json:"question"`
	Answer         string `json:"answer"`
	SourceLanguage string `json:"sourceLanguage"`
}

func ReadCacheTranslationSource(ctx context.Context, op OperationContext, input ReadCacheTranslationSourceInput) (ReadCacheTranslationSourceResult, error) {
	if op.CallerService != "localization" || op.Actor.Kind != "service" || op.Actor.SubjectID != "localization" {
		return ReadCacheTranslationSourceResult{}, platform.NewError(platform.CodeForbidden, false, "缓存翻译来源只允许本地化纯服务读取")
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ReadCacheTranslationSourceResult{}, err
	}
	record, err := ReadCache(tx, op, input.CacheId)
	tx.Close()
	if err != nil {
		return ReadCacheTranslationSourceResult{}, err
	}
	entry := record.Entry
	eligible := false
	if entry.State == "active" {
		work := qaruntime.ServiceReadContext(ctx, op)
		serviceOp, _ := platform.OperationContextFrom(work)
		validated, err := ValidateCachedAnswer(work, serviceOp, ValidateCachedAnswerInput{Entry: entry, ContextHash: entry.MemoryVersionHash})
		if err != nil {
			return ReadCacheTranslationSourceResult{}, err
		}
		if validated.Valid && validated.Entry != nil {
			entry = *validated.Entry
			eligible = true
		}
	}
	source := struct {
		SchemaVersion  int    `json:"schemaVersion"`
		CacheID        string `json:"cacheId"`
		AnswerRevision int    `json:"answerRevision"`
		SourceLanguage string `json:"sourceLanguage"`
		Question       string `json:"question"`
		Answer         string `json:"answer"`
	}{1, entry.CacheId, entry.AnswerRevision, entry.BaseLanguage, entry.Question, entry.Answer}
	raw, err := platform.CanonicalJSON(source)
	if err != nil {
		return ReadCacheTranslationSourceResult{}, err
	}
	return ReadCacheTranslationSourceResult{CacheId: entry.CacheId, AnswerRevision: entry.AnswerRevision, SourceHash: platform.HashBytes(raw), State: string(entry.State), Eligible: eligible, Question: entry.Question, Answer: entry.Answer, SourceLanguage: entry.BaseLanguage}, nil
}
