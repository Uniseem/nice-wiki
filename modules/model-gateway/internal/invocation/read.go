package invocation

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/provider"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"strings"
	"unicode/utf8"
)

type OperationContext = platform.OperationContext
type GetInvocationInput struct {
	InvocationID string `json:"invocationId"`
}
type GetInvocationResult struct {
	Invocation gen.Invocation        `json:"invocation"`
	Output     *platform.ModelOutput `json:"output,omitempty"`
}

func GetInvocation(ctx context.Context, op OperationContext, input GetInvocationInput) (GetInvocationResult, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return GetInvocationResult{}, err
	}
	defer tx.Close()
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		return GetInvocationResult{}, err
	}
	return ReadInvocationSnapshot(tx, record)
}

// ReadInvocationSnapshot核真实winning尝试与完整无错原证据；不以费用状态猜结果。
func ReadInvocationSnapshot(tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord) (GetInvocationResult, error) {
	result := GetInvocationResult{Invocation: record.Invocation}
	textual := record.Request.Capability == "chat" || record.Request.Capability == "vision" || record.Request.Capability == "translation"
	if record.Invocation.ResultAttemptId == nil {
		if record.Output != nil {
			return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "持久结果缺winning来源")
		}
		if textual && record.Invocation.State == "succeeded" {
			return GetInvocationResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "旧文本缺winning完整证明，不能当空结果或重新收费")
		}
		return result, nil
	}
	winner := *record.Invocation.ResultAttemptId
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, winner)
	if err != nil {
		if ErrorCode(err) == platform.CodeNotFound {
			return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "winning尝试记录缺失")
		}
		return GetInvocationResult{}, err
	}
	bound := false
	for _, item := range record.Invocation.Attempts {
		if item.AttemptId == winner {
			actual, _ := platform.CanonicalJSON(attempt.Attempt)
			listed, _ := platform.CanonicalJSON(item)
			bound = platform.HashBytes(actual) == platform.HashBytes(listed)
		}
	}
	if !bound || attempt.Attempt.InvocationId != record.Invocation.InvocationId || attempt.DispatchedNS == 0 {
		return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "winning尝试与调用完整快照不一致")
	}
	evidence, err := ReadAttemptEvidence(tx, winner)
	if err != nil {
		if ErrorCode(err) == platform.CodeNotFound {
			return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "原结果winning证据缺失")
		}
		return GetInvocationResult{}, err
	}
	if !evidence.Completed || evidence.Code != "" || evidence.HTTPStatus < 200 || evidence.HTTPStatus >= 300 || evidence.AcceptanceProof != "accepted" {
		return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "winning结果没有完整成功证据")
	}
	if record.Output == nil && evidence.Output == nil && textual {
		output, err := ReadLegacyTextResult(tx, record, evidence)
		if err != nil {
			return GetInvocationResult{}, err
		}
		result.Output = output
		return result, nil
	}
	if record.Output == nil || evidence.Output == nil || record.Output.Capability != string(record.Request.Capability) {
		return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "winning结果与原持久结果不完整")
	}
	stored, _ := platform.CanonicalJSON(record.Output)
	original, _ := platform.CanonicalJSON(evidence.Output)
	if platform.HashBytes(stored) != platform.HashBytes(original) {
		return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "原完整输出与winning证据不一致")
	}
	if textual && (record.Output.Text == nil || !utf8.ValidString(*record.Output.Text) || len(*record.Output.Text) > 65536 || record.Output.EmbeddingSpace != nil || len(record.Output.Vectors) != 0 || len(record.Output.CandidateIDs) != 0) {
		return GetInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "winning文本分支不符合完整UTF8/64KiB合同")
	}
	result.Output = record.Output
	return result, nil
}

// ReadLegacyTextResult只读原连续SSE；单一已证明winning外不猜增量归属，不改原费用/证据/hash。
func ReadLegacyTextResult(tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord, evidence provider.ProviderOutcome) (*platform.ModelOutput, error) {
	if len(record.Invocation.Attempts) != 1 || record.Invocation.Attempts[0].Ordinal != 1 || record.Invocation.Attempts[0].RetryReason != "initial" || record.Invocation.ResultAttemptId == nil || record.Invocation.Attempts[0].AttemptId != *record.Invocation.ResultAttemptId || record.TerminalNS <= 0 || record.Invocation.State != "succeeded" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "旧文本无单一终态来源，不能免费猜合并或重新收费")
	}
	var attempts int
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT count(*) FROM invocation_attempts WHERE invocation_id=?`, record.Invocation.InvocationId).Scan(&attempts); err != nil {
		return nil, gatewayruntime.ExecutionStorageError(err)
	}
	if attempts != 1 {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "旧文本存在多尝试且增量无独立来源")
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT sequence,event,payload_json FROM stream_events WHERE invocation_id=? ORDER BY sequence LIMIT 65541`, record.Invocation.InvocationId)
	if err != nil {
		return nil, gatewayruntime.ExecutionStorageError(err)
	}
	defer rows.Close()
	var text strings.Builder
	sequence := 0
	completed := false
	rawBytes := 0
	for rows.Next() {
		var current int
		var kind string
		var raw []byte
		if err = rows.Scan(&current, &kind, &raw); err != nil {
			return nil, gatewayruntime.ExecutionStorageError(err)
		}
		if current != sequence+1 || current > 65540 || completed {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "旧文本原流缺失或不连续，不能重建完整结果")
		}
		rawBytes += len(raw)
		if rawBytes > 64<<20 {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "旧文本原流超过64MiB读取预算")
		}
		if len(raw) > 1<<20 {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "旧文本单事件超读取预算")
		}
		var event gatewayruntime.StreamEvent
		if json.Unmarshal(raw, &event) != nil || event.Sequence != current || event.Event != kind {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本持久事件身份不一致")
		}
		original, e := platform.CanonicalJSON(json.RawMessage(raw))
		decoded, e2 := platform.CanonicalJSON(event)
		if e != nil || e2 != nil || platform.HashBytes(original) != platform.HashBytes(decoded) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本事件含未知字段，不能损失信息合并")
		}
		if event.Invocation != nil {
			snapshot := event.Invocation
			if snapshot.InvocationId != record.Invocation.InvocationId || snapshot.RootOperationId != record.Invocation.RootOperationId || snapshot.CostClass != record.Invocation.CostClass || !snapshot.Deadline.Equal(record.Invocation.Deadline) || snapshot.PriceSnapshotId != record.Invocation.PriceSnapshotId || snapshot.LastSequence != current {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本事件原调用绑定不一致")
			}
		}
		if event.Code != nil || event.Output != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本事件已有错误或冲突完整输出")
		}
		switch kind {
		case "accepted":
			if current != 1 || event.Invocation == nil || event.Invocation.State != "accepted" || event.Delta != nil {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本起始事件不完整")
			}
		case "delta":
			if current == 1 || event.Delta == nil || *event.Delta == "" || event.Invocation != nil || !utf8.ValidString(*event.Delta) || len(*event.Delta) > 16384 || text.Len()+len(*event.Delta) > 65536 {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本增量不符合原UTF8/大小边界")
			}
			_, _ = text.WriteString(*event.Delta)
		case "usage":
			if current == 1 || event.Delta != nil {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本用量事件混入文本")
			}
		case "completed":
			if current == 1 || event.Delta != nil || event.Invocation == nil || event.Invocation.State != "succeeded" || event.Invocation.ResultAttemptId == nil || *event.Invocation.ResultAttemptId != *record.Invocation.ResultAttemptId || len(event.Invocation.Attempts) != 1 || event.Invocation.Attempts[0].AttemptId != *record.Invocation.ResultAttemptId {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "旧文本完成事件缺单一winning证明")
			}
			completed = true
		default:
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "旧文本原流不是完整成功分支")
		}
		sequence = current
	}
	if err = rows.Err(); err != nil {
		return nil, gatewayruntime.ExecutionStorageError(err)
	}
	if !completed || sequence != record.Invocation.LastSequence || evidence.OutputStarted != (text.Len() > 0) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "旧文本缺完整原终态或增量证明")
	}
	complete := text.String()
	return &platform.ModelOutput{Capability: string(record.Request.Capability), Text: &complete}, nil
}
