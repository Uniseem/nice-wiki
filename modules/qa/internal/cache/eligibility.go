// Package cache 实现问答缓存的确定性准入。
// 指代是否补全只接受可信Planner字段，不扫描问题关键词。
package cache

import (
	"context"

	"nice-wiki/libs/platform"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
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
type QuestionPlan = qaruntime.QuestionPlan

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

// EvaluateCacheEligibility 只消费可信Planner判定及已验证的上下文标志。
// 私人化、当前页依赖、未补全、敏感动态或缺少明确指代补全证明均拒绝共享。
// firstTurn 不单独放行；不使用问题关键词、风险标签或相似度代替完整性判定。
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
	if !input.Plan.ResolvedReferences || input.Plan.CanonicalQuestion == "" || input.Plan.Classification == "needs_clarification" {
		return ineligible(reasonAmbiguous), nil
	}
	return EvaluateCacheEligibilityResult{Eligible: true, Reason: "complete"}, nil
}

func ineligible(reason string) EvaluateCacheEligibilityResult {
	return EvaluateCacheEligibilityResult{Eligible: false, Reason: reason}
}
