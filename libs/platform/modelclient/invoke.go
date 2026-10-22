// Package modelclient只用原键观察真实模型事实，不重放付费或伪造历史事件。
package modelclient

import (
	"context"
	"errors"
	"io"
	"nice-wiki/libs/platform"
	"time"
)

const (
	maxEvents        = 4096
	resumeRetryPause = time.Second
	cancelBudget     = 5 * time.Second
	maxIdempotency   = 256
)

func InvokeModel(ctx context.Context, client platform.GatewayClient, op platform.OperationContext, input platform.InvokeModelRequest) (platform.InvokeModelOutcome, error) {
	outcome := platform.InvokeModelOutcome{Events: []platform.StreamEvent{}}
	if client == nil || input.IDempotencyKey == "" || len(input.IDempotencyKey) > maxIdempotency {
		return outcome, platform.NewError(platform.CodeValidation, false, "模型调用请求不完整")
	}
	lookup, ok := client.(platform.InvocationLookupReader)
	if !ok {
		return outcome, platform.NewError(platform.CodeUnsupportedCapability, false, "模型客户端缺原键只读查找，禁止盲目重新创建")
	}
	current, ok := client.(platform.CurrentInvocationReader)
	if !ok {
		return outcome, platform.NewError(platform.CodeUnsupportedCapability, false, "模型客户端缺当前结果权威读取")
	}
	original := platform.WithOperationContext(ctx, op)
	paid, cancelPaid, err := bindDeadline(original, op)
	if err != nil {
		return outcome, err
	}
	defer cancelPaid()
	read, cancelRead := gatewayReadContext(original, op)
	snapshot, lookupErr := lookup.LookupInference(read, input.IDempotencyKey, input.Request)
	cancelRead()
	if lookupErr == nil {
		outcome.InvocationID = snapshot.Invocation.InvocationID
		outcome.CurrentSnapshot = &snapshot
	} else {
		var problem *platform.Error
		if !errors.As(lookupErr, &problem) || problem.Code != platform.CodeInvocationNotAccepted {
			return outcome, lookupErr
		}
		if err = paid.Err(); err != nil {
			return outcome, platform.NormalizeIOError(err, platform.CodeDeadlineExceeded, false, "原付费截止已到或调用已取消，未创建新模型调用")
		}
		invocation, createErr := client.CreateInference(paid, input.IDempotencyKey, input.Request)
		if createErr != nil {
			// 传输失败可能发生在真实接受之后；只查同原输入/原键，不再次Create。
			read, cancelRead = gatewayReadContext(original, op)
			snapshot, lookupErr = lookup.LookupInference(read, input.IDempotencyKey, input.Request)
			cancelRead()
			if lookupErr != nil {
				var failure *platform.Error
				if !errors.As(lookupErr, &failure) || failure.Code != platform.CodeInvocationNotAccepted {
					return outcome, lookupErr
				}
				return outcome, createErr
			}
			outcome.InvocationID = snapshot.Invocation.InvocationID
			outcome.CurrentSnapshot = &snapshot
		} else {
			outcome.InvocationID = invocation.InvocationID
			outcome.CurrentSnapshot = &platform.GatewayInvocationSnapshot{Invocation: invocation}
			if err = readCurrentGatewaySnapshot(original, current, op, &outcome); err != nil {
				return outcome, err
			}
		}
	}
	if input.OnAccepted != nil {
		if err = input.OnAccepted(outcome.InvocationID); err != nil {
			_ = cancelCall(original, client, outcome.InvocationID, input.IDempotencyKey)
			return outcome, err
		}
	}
	var lastSequence int64
	for {
		complete, err := finishKnownGatewaySnapshot(&outcome, lastSequence)
		if err != nil {
			return outcome, err
		}
		if complete {
			return outcome, nil
		}
		if err = paid.Err(); err != nil {
			_ = cancelCall(original, client, outcome.InvocationID, input.IDempotencyKey)
			_ = readCurrentGatewaySnapshot(original, current, op, &outcome)
			return outcome, platform.NormalizeIOError(err, platform.CodeCancelled, false, "模型原调用已取消或到达截止，保留已接受事实")
		}
		stream, streamErr := client.ResumeInference(paid, outcome.InvocationID, lastSequence)
		terminal := false
		if streamErr == nil {
			terminal, streamErr = drain(paid, client, stream, input, &outcome, &lastSequence)
			_ = stream.Close()
		}
		// 历史终态和EOF都不足以覆盖后来对账或完整结果，必须读当前权威。
		if err = readCurrentGatewaySnapshot(original, current, op, &outcome); err != nil {
			return outcome, err
		}
		// 调用方取消或到截止时流读取先报错返回：先取消原调用，不留下继续生成、继续计费的模型调用。
		if err = paid.Err(); err != nil {
			if complete, _ := finishKnownGatewaySnapshot(&outcome, lastSequence); !complete {
				_ = cancelCall(original, client, outcome.InvocationID, input.IDempotencyKey)
				_ = readCurrentGatewaySnapshot(original, current, op, &outcome)
			}
			return outcome, platform.NormalizeIOError(err, platform.CodeCancelled, false, "模型原调用已取消或到达截止，保留已接受事实")
		}
		if terminal && streamErr == nil {
			if err = notifyCurrentModelTerminal(input, outcome); err != nil {
				_ = cancelCall(original, client, outcome.InvocationID, input.IDempotencyKey)
				_ = readCurrentGatewaySnapshot(original, current, op, &outcome)
				return outcome, err
			}
		}
		var problem *platform.Error
		if streamErr != nil && errors.As(streamErr, &problem) && !problem.Retryable {
			return outcome, streamErr
		}
		if terminal && streamErr != nil && !errors.Is(streamErr, io.EOF) {
			return outcome, streamErr
		}
		complete, err = finishKnownGatewaySnapshot(&outcome, lastSequence)
		if err != nil {
			return outcome, err
		}
		if complete {
			return outcome, nil
		}
		if !modelInvokeSleep(paid, resumeRetryPause) {
			continue
		}
	}
}

// gatewayReadContext只给原根的只读观察/取消新的短RPC截止，不更改原业务op或付费请求。
func gatewayReadContext(ctx context.Context, op platform.OperationContext) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(context.WithoutCancel(platform.WithPortalProof(ctx, "")), cancelBudget)
	deadline, _ := work.Deadline()
	op.Deadline = deadline.UTC().Format(time.RFC3339Nano)
	op.OperationID = platform.NewIdentifier()
	return platform.WithOperationContext(work, op), cancel
}
func readCurrentGatewaySnapshot(ctx context.Context, client platform.CurrentInvocationReader, op platform.OperationContext, outcome *platform.InvokeModelOutcome) error {
	read, cancel := gatewayReadContext(ctx, op)
	defer cancel()
	snapshot, err := client.GetInference(read, outcome.InvocationID)
	if err != nil {
		var problem *platform.Error
		if errors.As(err, &problem) && problem.Code == platform.CodeNotFound {
			return platform.NewError(platform.CodeIntegrityError, false, "已接受调用的当前权威记录缺失，禁止重新付费")
		}
		return err
	}
	if snapshot.Invocation.InvocationID != outcome.InvocationID {
		return platform.NewError(platform.CodeIntegrityError, false, "当前模型读取更换了原调用ID")
	}
	outcome.CurrentSnapshot = &snapshot
	return nil
}
func finishKnownGatewaySnapshot(outcome *platform.InvokeModelOutcome, lastSequence int64) (bool, error) {
	if outcome.CurrentSnapshot == nil {
		return false, nil
	}
	snapshot := outcome.CurrentSnapshot
	if snapshot.Output != nil {
		outcome.State = "completed"
		return true, nil
	}
	state, terminal := terminalOf(snapshot.Invocation.State)
	if !terminal {
		return false, nil
	}
	if state != "completed" && snapshot.Invocation.ResultAttemptID == nil {
		outcome.State = state
		return true, nil
	}
	if lastSequence < snapshot.Invocation.LastSequence {
		return false, nil
	}
	// 兼容旧完整文本只有原持久SSE的情况，仍不伪造CurrentSnapshot.Output。
	for i := len(outcome.Events) - 1; i >= 0; i-- {
		event := outcome.Events[i]
		if event.Event != platform.StreamCompleted {
			continue
		}
		if event.Invocation != nil && event.Invocation.ResultAttemptID != nil && snapshot.Invocation.ResultAttemptID != nil && *event.Invocation.ResultAttemptID != *snapshot.Invocation.ResultAttemptID {
			continue
		}
		outcome.State = "completed"
		return true, nil
	}
	return false, platform.NewError(platform.CodeIntegrityError, false, "原终态缺完整输出或已登记持久完成事实，不能重派补算")
}
func modelEventTerminal(event platform.StreamEvent) bool {
	if event.Invocation != nil {
		if _, terminal := terminalOf(event.Invocation.State); terminal {
			return true
		}
	}
	return event.Event == platform.StreamCompleted || event.Event == platform.StreamError || event.Event == platform.StreamCancelled
}

// notifyCurrentModelTerminal只回调仍与当前事实相容的真实终态，旧错误仍原样保留Events。
func notifyCurrentModelTerminal(input platform.InvokeModelRequest, outcome platform.InvokeModelOutcome) error {
	if !input.Stream || input.OnEvent == nil || len(outcome.Events) == 0 || outcome.CurrentSnapshot == nil {
		return nil
	}
	event := outcome.Events[len(outcome.Events)-1]
	snapshot := outcome.CurrentSnapshot
	compatible := false
	switch event.Event {
	case platform.StreamCompleted:
		compatible = snapshot.Output != nil || snapshot.Invocation.State == platform.InvocationSucceeded || snapshot.Invocation.ResultAttemptID != nil
	case platform.StreamError:
		compatible = snapshot.Output == nil && snapshot.Invocation.ResultAttemptID == nil && (snapshot.Invocation.State == platform.InvocationFailed || snapshot.Invocation.State == platform.InvocationChargeUnknown || snapshot.Invocation.State == "reconciled")
	case platform.StreamCancelled:
		compatible = snapshot.Output == nil && snapshot.Invocation.ResultAttemptID == nil && snapshot.Invocation.State == platform.InvocationCancelled
	default:
		compatible = event.Invocation != nil && event.Invocation.State == snapshot.Invocation.State
	}
	if compatible && event.Invocation != nil && event.Invocation.ResultAttemptID != nil && snapshot.Invocation.ResultAttemptID != nil && *event.Invocation.ResultAttemptID != *snapshot.Invocation.ResultAttemptID {
		compatible = false
	}
	if compatible {
		if err := input.OnEvent(event); err != nil {
			return platform.NewError(platform.CodeCancelled, false, "调用方中止模型事件回调")
		}
	}
	return nil
}
func drain(ctx context.Context, client platform.GatewayClient, stream platform.ModelEventStream, input platform.InvokeModelRequest, outcome *platform.InvokeModelOutcome, lastSequence *int64) (bool, error) {
	for {
		event, err := stream.Next(ctx)
		if err != nil {
			return false, err
		}
		if event.Sequence <= *lastSequence {
			continue
		}
		if event.Sequence != *lastSequence+1 {
			return true, platform.NewError(platform.CodeIntegrityError, false, "模型持久事件序号缺失，不能拼接不完整结果")
		}
		if len(outcome.Events) >= maxEvents {
			_ = cancelCall(ctx, client, outcome.InvocationID, input.IDempotencyKey)
			return true, platform.NewError(platform.CodeBudgetExceeded, false, "模型持久结果超过读取事件上限")
		}
		*lastSequence = event.Sequence
		outcome.Events = append(outcome.Events, event)
		if modelEventTerminal(event) {
			return true, nil
		}
		if input.Stream && input.OnEvent != nil {
			if err = input.OnEvent(event); err != nil {
				_ = cancelCall(ctx, client, outcome.InvocationID, input.IDempotencyKey)
				return true, platform.NewError(platform.CodeCancelled, false, "调用方中止模型事件回调")
			}
		}
	}
}
func terminalOf(state string) (string, bool) {
	switch state {
	case platform.InvocationSucceeded:
		return "completed", true
	case platform.InvocationFailed, "reconciled":
		return "failed", true
	case platform.InvocationCancelled:
		return "cancelled", true
	case platform.InvocationChargeUnknown:
		return "charge_unknown", true
	}
	return "", false
}
func cancelCall(ctx context.Context, client platform.GatewayClient, invocationID platform.Identifier, idempotencyKey string) error {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "取消原模型调用缺可信根")
	}
	work, cancel := gatewayReadContext(ctx, op)
	defer cancel()
	return client.CancelInference(work, idempotencyKey+":cancel", invocationID, "调用方取消或截止")
}
func bindDeadline(ctx context.Context, op platform.OperationContext) (context.Context, context.CancelFunc, error) {
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "原模型付费截止不合法")
	}
	work, cancel := context.WithDeadline(ctx, deadline)
	return work, cancel, nil
}
func modelInvokeSleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// ObserveInvocation仅观察原已知调用，供owner锁外恢复查询；没有Create或Cancel分支。
func ObserveInvocation(ctx context.Context, client platform.GatewayClient, op platform.OperationContext, invocationID platform.Identifier) (platform.GatewayInvocationSnapshot, error) {
	reader, ok := client.(platform.CurrentInvocationReader)
	if !ok {
		return platform.GatewayInvocationSnapshot{}, platform.NewError(platform.CodeUnsupportedCapability, false, "模型客户端缺当前权威只读能力")
	}
	if invocationID == "" {
		return platform.GatewayInvocationSnapshot{}, platform.NewError(platform.CodeValidation, false, "原调用ID缺失")
	}
	outcome := platform.InvokeModelOutcome{InvocationID: invocationID}
	if err := readCurrentGatewaySnapshot(ctx, reader, op, &outcome); err != nil {
		return platform.GatewayInvocationSnapshot{}, err
	}
	return *outcome.CurrentSnapshot, nil
}

// LookupModelInvocation仅按原完整请求和原键只读查找，原付费截止有效也不会Create。
func LookupModelInvocation(ctx context.Context, client platform.GatewayClient, op platform.OperationContext, input platform.InvokeModelRequest) (platform.GatewayInvocationSnapshot, error) {
	reader, ok := client.(platform.InvocationLookupReader)
	if !ok {
		return platform.GatewayInvocationSnapshot{}, platform.NewError(platform.CodeUnsupportedCapability, false, "模型客户端缺原键只读查找能力")
	}
	if input.IDempotencyKey == "" || len(input.IDempotencyKey) > maxIdempotency {
		return platform.GatewayInvocationSnapshot{}, platform.NewError(platform.CodeValidation, false, "模型原键查找请求不完整")
	}
	read, cancel := gatewayReadContext(ctx, op)
	defer cancel()
	return reader.LookupInference(read, input.IDempotencyKey, input.Request)
}
