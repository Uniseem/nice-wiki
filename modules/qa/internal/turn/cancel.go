package turn

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
	"time"
)

type CancelTurnInput = gen.CancelTurnInput
type CancelTurnResult = gen.CancelTurnResult

// CancelTurn 先原子提交取消标记和唯一终态，随后幂等取消下游；重放继续清理未完成副作用。
func CancelTurn(ctx context.Context, op OperationContext, input CancelTurnInput) (CancelTurnResult, error) {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return CancelTurnResult{}, err
	}
	defer tx.Close()
	var result CancelTurnResult
	replay, err := qaruntime.ReplayReceipt(tx, op, "qa.CancelTurn", input.IdempotencyKey, input, &result)
	if err != nil {
		return result, err
	}
	record, err := qaruntime.ReadTurn(tx, op, input.TurnId, true)
	if err != nil {
		return result, err
	}
	if !replay {
		if !qaruntime.IsTerminal(record.State) {
			record.CancelRequested = true
			record.Turn.State = gen.TurnState("cancelled")
			record.State = "cancelled"
			record.TerminalNS = time.Now().UnixNano()
			if err := stream.SaveEvent(tx, &record, gen.AnswerEvent{Event: gen.AnswerEventEvent("cancelled"), Turn: &record.Turn}, "", nil); err != nil {
				return result, err
			}
		}
		result.Turn = record.Turn
		if err := qaruntime.SaveReceipt(tx, op, "qa.CancelTurn", input.IdempotencyKey, input, result); err != nil {
			return CancelTurnResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return CancelTurnResult{}, err
	}
	tx.Close()
	if record.State == "cancelled" {
		// 本进程里还在跑的就直接打断：在途模型调用随上下文取消，后续步骤不再开始。
		stopRunningTurn(record.Turn.TurnId, errTurnCancelled)
		if err := cancelTurnDependencies(ctx, record); err != nil {
			return CancelTurnResult{}, err
		}
	}
	return result, nil
}

func cancelTurnDependencies(ctx context.Context, record qaruntime.TurnRecord) error {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return err
	}
	// 取消常在原提问请求结束之后，原截止时间早已过去：控制请求沿用原调用、根操作和代次，只换新的短截止时间。
	op := record.Operation
	op.Deadline = time.Now().UTC().Add(15 * time.Second).Format(time.RFC3339Nano)
	callCtx := qaruntime.ServiceReadContext(ctx, op)
	if record.JobID != "" {
		if runtime.Jobs == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "取消任务客户端尚未装配")
		}
		raw, err := runtime.Jobs.CancelJob(callCtx, record.JobID, struct{}{}, "qa.cancel-job."+record.Turn.TurnId)
		if err != nil {
			return err
		}
		var response struct {
			State string `json:"state"`
		}
		if err := qaruntime.DecodeResponse(runtime, "JobsRequestCancelResult", raw, &response); err != nil {
			return err
		}
	}
	for _, id := range record.InvocationIDs {
		if runtime.Gateway == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "模型取消客户端尚未装配")
		}
		if err := runtime.Gateway.CancelInference(callCtx, "qa.cancel-model."+record.Turn.TurnId+":"+id, id, "user_cancelled"); err != nil {
			return err
		}
	}
	if runtime.Guard == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "取消排队客户端尚未装配")
	}
	raw, err := runtime.Guard.QueueSessionAdmission(callCtx, struct {
		SessionKey string `json:"sessionKey"`
		RequestID  string `json:"requestId"`
		Mode       string `json:"mode"`
	}{record.Permit.SessionBudgetScope, record.Operation.RootOperationID, "cancel"}, "qa.cancel-queue."+record.Turn.TurnId)
	if err != nil {
		return err
	}
	var queue struct {
		Queued   bool  `json:"queued"`
		Position int64 `json:"position"`
	}
	if err := qaruntime.DecodeResponse(runtime, "GuardQueueSessionAdmissionResult", raw, &queue); err != nil {
		return err
	}
	if queue.Queued {
		return platform.NewError(platform.CodeIntegrityError, false, "取消排队回执仍声明当前请求排队")
	}
	return releaseTurnPermit(ctx, record)
}
