// Package modelclient 实现模型调用助手的通用机制。
//
// createInference只返回受理；本包按序号读取resumeInference直到终态，
// 流式时逐条回调；取消或截止时cancelInference；失败与备用重试由
// 网关负责，调用方不换模型、不重放。
package modelclient

import (
	"context"
	"errors"
	"io"
	"time"

	"nice-wiki/libs/platform"
)

// 事件与轮询参数边界。
const (
	maxEvents        = 4096
	resumeRetryPause = time.Second
	cancelBudget     = 5 * time.Second
	maxIdempotency   = 256
)

// InvokeModel 执行一次模型调用并读取到终态：
// createInference（幂等键由调用方按操作ID+步骤名给出）→按序号
// 读取resumeInference到终态（stream时逐条回调）→取消或截止时
// cancelInference；failed/charge_unknown以终态返回，不在调用方重放。
func InvokeModel(ctx context.Context, client platform.GatewayClient, op platform.OperationContext, input platform.InvokeModelRequest) (platform.InvokeModelOutcome, error) {
	if client == nil || input.IDempotencyKey == "" || len(input.IDempotencyKey) > maxIdempotency {
		return platform.InvokeModelOutcome{}, platform.NewError(platform.CodeValidation, false, "模型调用请求不完整")
	}
	ctx, cancelDeadline := bindDeadline(ctx, op)
	defer cancelDeadline()
	invocation, err := client.CreateInference(ctx, input.IDempotencyKey, input.Request)
	if err != nil {
		return platform.InvokeModelOutcome{}, err
	}
	outcome := platform.InvokeModelOutcome{InvocationID: invocation.InvocationID}
	if state, terminal := terminalOf(invocation.State); terminal {
		outcome.State = state
		return outcome, nil
	}
	var lastSequence int64 = -1
	if invocation.LastSequence > 0 {
		lastSequence = invocation.LastSequence
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = cancelCall(ctx, client, invocation.InvocationID, input.IDempotencyKey)
			return platform.InvokeModelOutcome{}, platform.NewError(platform.CodeCancelled, false, "模型调用已取消或到达截止")
		}
		stream, err := client.ResumeInference(ctx, invocation.InvocationID, lastSequence+1)
		if err != nil {
			if ctx.Err() != nil {
				_ = cancelCall(ctx, client, invocation.InvocationID, input.IDempotencyKey)
				return platform.InvokeModelOutcome{}, platform.NewError(platform.CodeCancelled, false, "模型调用已取消或到达截止")
			}
			if !sleep(ctx, resumeRetryPause) {
				_ = cancelCall(ctx, client, invocation.InvocationID, input.IDempotencyKey)
				return platform.InvokeModelOutcome{}, platform.NewError(platform.CodeCancelled, false, "模型调用已取消或到达截止")
			}
			continue
		}
		finished, err := drain(ctx, client, stream, input, &outcome, &lastSequence)
		_ = stream.Close()
		if finished {
			return outcome, err
		}
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
			// 流中断：按序号重连续读，直到终态或截止。
			if !sleep(ctx, resumeRetryPause) {
				_ = cancelCall(ctx, client, invocation.InvocationID, input.IDempotencyKey)
				return platform.InvokeModelOutcome{}, platform.NewError(platform.CodeCancelled, false, "模型调用已取消或到达截止")
			}
		}
	}
}

// drain 读取流内事件直到终态；返回是否已到终态。
func drain(ctx context.Context, client platform.GatewayClient, stream platform.ModelEventStream, input platform.InvokeModelRequest, outcome *platform.InvokeModelOutcome, lastSequence *int64) (bool, error) {
	for {
		event, err := stream.Next(ctx)
		if err != nil {
			return false, err
		}
		if event.Sequence <= *lastSequence {
			continue
		}
		*lastSequence = event.Sequence
		if len(outcome.Events) < maxEvents {
			outcome.Events = append(outcome.Events, event)
		}
		if input.Stream && input.OnEvent != nil {
			if err := input.OnEvent(event); err != nil {
				_ = cancelCall(ctx, client, outcome.InvocationID, input.IDempotencyKey)
				return true, platform.NewError(platform.CodeCancelled, false, "调用方中止模型事件回调")
			}
		}
		if event.Invocation != nil {
			if state, terminal := terminalOf(event.Invocation.State); terminal {
				outcome.State = state
				return true, nil
			}
		}
		switch event.Event {
		case platform.StreamCompleted:
			outcome.State = "completed"
			return true, nil
		case platform.StreamCancelled:
			outcome.State = "cancelled"
			return true, nil
		case platform.StreamError:
			outcome.State = "failed"
			return true, nil
		}
	}
}

// terminalOf 把网关调用终态映射为结果终态。
func terminalOf(state string) (string, bool) {
	switch state {
	case platform.InvocationSucceeded:
		return "completed", true
	case platform.InvocationFailed:
		return "failed", true
	case platform.InvocationCancelled:
		return "cancelled", true
	case platform.InvocationChargeUnknown:
		return "charge_unknown", true
	}
	return "", false
}

// cancelCall 取消网关调用；使用独立短超时上下文，确保退出路径可完成。
func cancelCall(ctx context.Context, client platform.GatewayClient, invocationID platform.Identifier, idempotencyKey string) error {
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cancelBudget)
	defer cancel()
	return client.CancelInference(cancelCtx, idempotencyKey+":cancel", invocationID, "调用方取消或截止")
}

// bindDeadline 把操作截止并入ctx；取两者较早值。
func bindDeadline(ctx context.Context, op platform.OperationContext) (context.Context, context.CancelFunc) {
	if op.Deadline == "" {
		return context.WithCancel(ctx)
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return context.WithCancel(ctx)
	}
	return context.WithDeadline(ctx, deadline)
}

// sleep 等待d或ctx取消；返回false表示已取消。
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
