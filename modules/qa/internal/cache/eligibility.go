// Package cache 实现问答缓存的确定性准入。
// 本文件只按已写明的布尔字段拒绝共享；指代残留没有可执行规则，因此不放行。
package cache

import (
	"context"

	"nice-wiki/libs/platform"
)

// OperationContext 是平台可信执行上下文的本域别名。
type OperationContext = platform.OperationContext

const (
	reasonAmbiguous  = "ambiguous"
	reasonPrivate    = "private"
	reasonOutOfScope = "out_of_scope"

	classificationOutOfScope = "out_of_scope"
)

// QuestionPlan 是前置计划里本函数读取的封闭字段。
type QuestionPlan struct {
	OriginalQuestion   string        `json:"originalQuestion"`
	CanonicalQuestion  string        `json:"canonicalQuestion"`
	VisitorLanguage    string        `json:"visitorLanguage"`
	Complete           bool          `json:"complete"`
	CacheEligible      bool          `json:"cacheEligible"`
	Subqueries         []string      `json:"subqueries"`
	Classification     string        `json:"classification"`
	Risk               string        `json:"risk"`
	ContextFingerprint platform.Hash `json:"contextFingerprint"`
}

// EvaluateCacheEligibilityInput 是缓存准入输入。
type EvaluateCacheEligibilityInput struct {
	Plan                    QuestionPlan `json:"plan"`
	FirstTurn               bool         `json:"firstTurn"`
	CurrentPageDependent    bool         `json:"currentPageDependent"`
	PrivateContextDependent bool         `json:"privateContextDependent"`
}

// EvaluateCacheEligibilityResult 表示能否查写共享缓存。
type EvaluateCacheEligibilityResult struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
}

// EvaluateCacheEligibility 只执行已写明的确定性拒绝。
// 已补全看 plan.complete，非个性化看 privateContextDependent，
// 非敏感动态看 plan.cacheEligible，仍依赖页面看 currentPageDependent。
// 分类为 out_of_scope 时不写入可回答缓存。
// 不读取问题原文，不接语义缓存、嵌入、模型或存储。
// firstTurn 没有单独的准入句子，不据此放行或拒绝。
// 指代是否残留这/它或未命名来源没有可执行规则，因此不返回 eligible=true。
func EvaluateCacheEligibility(ctx context.Context, op OperationContext, input EvaluateCacheEligibilityInput) (EvaluateCacheEligibilityResult, error) {
	_ = ctx
	_ = op

	if !input.Plan.Complete {
		return ineligible(reasonAmbiguous), nil
	}
	if input.PrivateContextDependent {
		return ineligible(reasonPrivate), nil
	}
	if input.CurrentPageDependent {
		return ineligible(reasonAmbiguous), nil
	}
	if input.Plan.Classification == classificationOutOfScope {
		return ineligible(reasonOutOfScope), nil
	}
	if !input.Plan.CacheEligible {
		return ineligible(reasonAmbiguous), nil
	}
	// 已补全、非个性化、计划标记为非敏感动态，且不依赖当前页、不是 out_of_scope。
	// 独立问题是否仍残留这/它或未命名的第二步来源，合同没有判定方法。
	return ineligible(reasonAmbiguous), nil
}

func ineligible(reason string) EvaluateCacheEligibilityResult {
	return EvaluateCacheEligibilityResult{Eligible: false, Reason: reason}
}
