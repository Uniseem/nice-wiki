package internal

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"time"

	"nice-wiki/libs/platform"
)

type ClassifyFailureInput struct {
	ErrorCode       string
	Attempt         int64
	CancelRequested bool
}

type ClassifyFailureResult struct {
	Retryable     bool
	TerminalState string
}

type ComputeRetryAtInput struct {
	Attempt int64
	Now     time.Time
	JobID   string
}

type ComputeRetryAtResult struct{ RetryAt time.Time }

// ClassifyFailure 使用固定错误码策略，未知错误不进入自动重试。
func ClassifyFailure(ctx context.Context, op OperationContext, in ClassifyFailureInput) (ClassifyFailureResult, error) {
	if _, known := platform.ProblemHTTPStatus[in.ErrorCode]; !known {
		return ClassifyFailureResult{}, platform.NewError(platform.CodeValidation, false, "任务失败必须使用公共错误码")
	}
	if in.ErrorCode == platform.CodeCancelled {
		if !in.CancelRequested {
			return ClassifyFailureResult{}, platform.NewError(platform.CodeValidation, false, "任务尚未请求安全取消")
		}
		return ClassifyFailureResult{TerminalState: "cancelled"}, nil
	}
	if (in.ErrorCode == platform.CodeDependencyUnavailable || in.ErrorCode == platform.CodeDeadlineExceeded) && in.Attempt < 5 {
		return ClassifyFailureResult{Retryable: true, TerminalState: "retry-wait"}, nil
	}
	return ClassifyFailureResult{TerminalState: "dead-letter"}, nil
}

// ComputeRetryAt 按固定退避表和确定抖动计算下一次调度，不生成新的付费操作。
func ComputeRetryAt(ctx context.Context, op OperationContext, in ComputeRetryAtInput) (ComputeRetryAtResult, error) {
	if in.Attempt < 1 || in.Attempt > 4 {
		return ComputeRetryAtResult{}, platform.NewError(platform.CodeValidation, false, "任务重试次数已耗尽")
	}
	delays := [...]time.Duration{5 * time.Second, 30 * time.Second, 120 * time.Second, 600 * time.Second}
	hash := sha256.Sum256([]byte(in.JobID + ":" + strconv.FormatInt(in.Attempt, 10)))
	jitter := time.Duration(binary.BigEndian.Uint64(hash[:8])%1000) * time.Millisecond
	return ComputeRetryAtResult{RetryAt: in.Now.Add(delays[in.Attempt-1] + jitter)}, nil
}
