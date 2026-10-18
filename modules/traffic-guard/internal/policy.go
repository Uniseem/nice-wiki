// Package trafficguard 内聚本域风控参数规则；本文件不执行持久化或外部调用。
package trafficguard

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/traffic-guard/internal/gen"
)

// OperationContext 是服务间可信操作上下文的本域别名。
type OperationContext = platform.OperationContext

// GuardPolicy 与 types.json 中的无费用准入策略对应。
type GuardPolicy = gen.GuardPolicy

// ValidatePolicyInput 是风控参数校验输入。
type ValidatePolicyInput struct {
	PolicyJSON GuardPolicy `json:"policyJSON"`
}

// ValidatePolicyResult 是风控参数校验结果。
type ValidatePolicyResult struct {
	NormalizedJSON GuardPolicy `json:"normalizedJSON"`
}

const (
	minPolicyRate        = 1
	maxSubjectRate       = 1000
	maxBurst             = 100
	maxSiteRate          = 100000
	minPolicyConcurrency = 1
	maxPolicyConcurrency = 128
)

// ValidatePolicy 校验首版准入策略的整数速率、突发量和并发范围。
// GuardPolicy 本身不包含费用、余额、挑战密钥或 ban 凭证字段；未知字段由
// 严格 schema 解码边界拒绝。本函数只返回输入的规范化值，不产生副作用。
func ValidatePolicy(ctx context.Context, op OperationContext, in ValidatePolicyInput) (ValidatePolicyResult, error) {
	if !validPolicyRate(in.PolicyJSON.QaPerSubjectPerMinute, minPolicyRate, maxSubjectRate) ||
		!validPolicyRate(in.PolicyJSON.QaBurst, minPolicyRate, maxBurst) ||
		in.PolicyJSON.QaBurst > in.PolicyJSON.QaPerSubjectPerMinute ||
		!validPolicyRate(in.PolicyJSON.QaPerSitePerMinute, minPolicyRate, maxSiteRate) ||
		!validPolicyRate(networkPolicyRate(in.PolicyJSON), minPolicyRate, maxSiteRate) ||
		!validPolicyRate(in.PolicyJSON.QaSubjectConcurrency, minPolicyConcurrency, maxPolicyConcurrency) ||
		!validPolicyRate(in.PolicyJSON.QaSiteConcurrency, minPolicyConcurrency, maxPolicyConcurrency) {
		return ValidatePolicyResult{}, platform.NewError(platform.CodeValidation, false, "风控策略不合法")
	}

	normalized := in.PolicyJSON
	if normalized.QaPerNetworkPerMinute == nil {
		rate := 600
		normalized.QaPerNetworkPerMinute = &rate
	}
	return ValidatePolicyResult{NormalizedJSON: normalized}, nil
}

func validPolicyRate(value, minimum, maximum int) bool {
	return value >= minimum && value <= maximum
}

// networkPolicyRate 为旧策略缺失网络次数字段提供已确认的兼容默认，不推断费用比例。
func networkPolicyRate(policy GuardPolicy) int {
	if policy.QaPerNetworkPerMinute == nil {
		return 600
	}
	return *policy.QaPerNetworkPerMinute
}
