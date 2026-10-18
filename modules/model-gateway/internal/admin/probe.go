package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/invocation"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"slices"
	"time"
)

type CreateConnectionProbeInput = gen.CreateConnectionProbeInput
type CreateConnectionProbeResult = gen.CreateConnectionProbeResult

// CreateConnectionProbe只由已即时授权的超管按钮调用，读真实模型后走显式诊断执行策略。
func CreateConnectionProbe(ctx context.Context, op OperationContext, input CreateConnectionProbeInput) (CreateConnectionProbeResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" || !slices.Contains(op.Actor.Grants, "superadmin") {
		return CreateConnectionProbeResult{}, platform.NewError(platform.CodeForbidden, false, "连接探测须超管明确批准")
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return CreateConnectionProbeResult{}, err
	}
	models, err := readCatalogModels(ctx, op, tx, nil, 1000)
	_ = tx.Rollback()
	if err != nil {
		return CreateConnectionProbeResult{}, err
	}
	for _, model := range models {
		if model.ModelId == input.ModelId {
			return invocation.CreateProbeInvocation(ctx, op, input, model)
		}
	}
	return CreateConnectionProbeResult{}, platform.NewError(platform.CodeNotFound, false, "诊断模型不存在")
}

// FeeProbeLimit的缺失等同本次用户明确批准的0不限，不借用QA额度。
func FeeProbeLimit(policy gen.FeeBudgetPolicy) int {
	if policy.DailyProbeLimit == nil {
		return 0
	}
	return policy.DailyProbeLimit.Micros
}

// ReadLatestProbes只读原诊断和实际响应证据；历史版本不冒当前渠道或当前密钥的成功证明。
func ReadLatestProbes(ctx context.Context, op OperationContext, tx *sql.Tx, providers []gen.ProviderSummary) error {
	execution := &gatewayruntime.ExecutionTransaction{Tx: tx, Context: ctx}
	for i := range providers {
		provider := &providers[i]
		var id, modelID string
		var created int64
		err := tx.QueryRowContext(ctx, `SELECT invocation_id,model_id,created_ns FROM connection_probes WHERE provider_id=? AND data_epoch=? ORDER BY created_ns DESC,invocation_id DESC LIMIT 1`, provider.ProviderId, op.DataEpoch).Scan(&id, &modelID, &created)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return adminStorageError(err)
		}
		record, err := gatewayruntime.ReadInvocationRecord(execution, op, id)
		if err != nil {
			return err
		}
		var fixed gatewayruntime.FixedRoute
		if record.Probe == nil || record.Operation.CostClass != platform.CostClassAdminProbe || record.CreatedNS != created || json.Unmarshal(record.Route, &fixed) != nil || len(fixed.Candidates) != 1 || fixed.Fallback || fixed.Assignment != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "最近诊断缺唯一原批准路线")
		}
		candidate := fixed.Candidates[0]
		if candidate.Model.ModelId != modelID || candidate.Provider.ProviderId != provider.ProviderId || candidate.CredentialVersion < 1 {
			return platform.NewError(platform.CodeIntegrityError, false, "最近诊断冻结来源矛盾")
		}
		summary := gen.ConnectionProbeSummary{Invocation: record.Invocation, ModelId: modelID, ProviderVersion: candidate.Provider.Version, ModelVersion: candidate.Model.Version, CredentialVersion: candidate.CredentialVersion, CreatedAt: time.Unix(0, created).UTC()}
		if len(record.Invocation.Attempts) > 1 {
			return platform.NewError(platform.CodeIntegrityError, false, "单次连接诊断出现多个收费尝试")
		}
		if len(record.Invocation.Attempts) == 1 {
			attempt, err := gatewayruntime.ReadAttemptRecord(execution, record.Invocation.Attempts[0].AttemptId)
			if err != nil {
				return err
			}
			listed, _ := platform.CanonicalJSON(record.Invocation.Attempts[0])
			actual, _ := platform.CanonicalJSON(attempt.Attempt)
			if platform.HashBytes(listed) != platform.HashBytes(actual) || attempt.Model.ModelId != modelID || attempt.Provider.ProviderId != provider.ProviderId || attempt.CredentialVersion != candidate.CredentialVersion {
				return platform.NewError(platform.CodeIntegrityError, false, "最近诊断原attempt与原调用不同")
			}
			var responded int64
			err = tx.QueryRowContext(ctx, `SELECT recorded_ns FROM attempt_evidence WHERE attempt_id=?`, attempt.Attempt.AttemptId).Scan(&responded)
			if err != nil && err != sql.ErrNoRows {
				return adminStorageError(err)
			}
			if err == nil {
				if attempt.DispatchedNS <= 0 || responded < attempt.DispatchedNS {
					return platform.NewError(platform.CodeIntegrityError, false, "最近诊断响应时间与真实派发矛盾")
				}
				latency := int((responded - attempt.DispatchedNS) / int64(time.Millisecond))
				summary.LatencyMs = &latency
			}
		}
		provider.LastProbe = &summary
	}
	return nil
}
