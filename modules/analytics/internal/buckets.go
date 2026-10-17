// Package analytics 实现统计与审计域中合同已写明的纯函数。
package analytics

import (
	"context"

	"nice-wiki/libs/platform"
)

// OperationContext 是平台可信操作上下文的本域别名。纯函数不读取它。
type OperationContext = platform.OperationContext

// BucketLatencyInput 是固定时延桶输入。ElapsedMs 为毫秒，类型用 int64。
type BucketLatencyInput struct {
	ElapsedMs int64
}

// BucketLatencyResult 是固定时延桶。失败时调用方只看错误，结果为零值。
type BucketLatencyResult struct {
	Bucket string
}

const latencyBucket0To99 = "0..99"
const latencyBucket100To499 = "100..499"
const latencyBucket500To999 = "500..999"
const latencyBucket1000To2999 = "1000..2999"
const latencyBucket3000Plus = "3000+"

// maxBucketElapsedMs 是合同允许的最大毫秒数，60000 本身仍落入 3000+。
const maxBucketElapsedMs int64 = 60000

// BucketLatency 把毫秒耗时分入固定闭区间桶。
// 0 到 99、100 到 499、500 到 999、1000 到 2999 分别对应四个有限桶。
// 3000 到 60000（含两端）为 "3000+"。小于 0 或大于 60000 返回 VALIDATION。
// 不根据 ctx 或 op 分支。
func BucketLatency(ctx context.Context, op OperationContext, in BucketLatencyInput) (BucketLatencyResult, error) {
	_ = ctx
	_ = op
	if in.ElapsedMs < 0 || in.ElapsedMs > maxBucketElapsedMs {
		return BucketLatencyResult{}, platform.NewError(platform.CodeValidation, false, "时延超出允许范围")
	}
	switch {
	case in.ElapsedMs <= 99:
		return BucketLatencyResult{Bucket: latencyBucket0To99}, nil
	case in.ElapsedMs <= 499:
		return BucketLatencyResult{Bucket: latencyBucket100To499}, nil
	case in.ElapsedMs <= 999:
		return BucketLatencyResult{Bucket: latencyBucket500To999}, nil
	case in.ElapsedMs <= 2999:
		return BucketLatencyResult{Bucket: latencyBucket1000To2999}, nil
	default:
		return BucketLatencyResult{Bucket: latencyBucket3000Plus}, nil
	}
}
