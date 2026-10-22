package invocation

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"slices"
	"time"
)

// CreateProbeInvocation只接受本域已验证的超管诊断，不进入公开AI任务/档位入口。
func CreateProbeInvocation(ctx context.Context, op OperationContext, input gen.CreateConnectionProbeInput, model gen.Model) (gen.CreateConnectionProbeResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" || !slices.Contains(op.Actor.Grants, "superadmin") {
		return gen.CreateConnectionProbeResult{}, platform.NewError(platform.CodeForbidden, false, "连接探测须当前超管明确批准")
	}
	op.CostClass = platform.CostClassAdminProbe
	canonical := struct {
		Input                    gen.CreateConnectionProbeInput
		Actor, Site, Epoch, Root string
	}{input, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.RootOperationID}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return gen.CreateConnectionProbeResult{}, err
	}
	var result gen.CreateConnectionProbeResult
	replay, err := gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.CreateConnectionProbe", input.IdempotencyKey, canonical, &result)
	if err != nil {
		tx.Close()
		return result, err
	}
	if replay {
		record, e := gatewayruntime.ReadInvocationRecord(tx, op, result.Invocation.InvocationId)
		tx.Close()
		if e != nil {
			return result, e
		}
		options, e := gatewayruntime.ExecutionDependencies()
		if e != nil {
			return result, e
		}
		if record.Invocation.State == "accepted" || record.Invocation.State == "reserved" {
			if _, e = StartAcceptedInvocation(ctx, options, record); e != nil {
				return result, e
			}
		}
		result.Invocation = record.Invocation
		return result, nil
	}
	if err = RequireInvocationLookupTracking(tx, op); err != nil {
		tx.Close()
		return result, err
	}
	tx.Close()
	if !model.Enabled || model.ModelId != input.ModelId || model.ProviderId != input.ProviderId {
		return result, platform.NewError(platform.CodeUnsupportedCapability, false, "诊断模型须当前启用且属于选定渠道")
	}
	price, err := billing.ReadPriceSnapshot(ctx, op, model.ModelId, time.Now().UTC(), true)
	if err != nil {
		return result, err
	}
	options, err := gatewayruntime.ExecutionDependencies()
	if err != nil {
		return result, err
	}
	if options.Tasks == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, false, "诊断任务生命周期尚未装配")
	}
	tx, err = gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return result, err
	}
	defer tx.Close()
	replay, err = gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.CreateConnectionProbe", input.IdempotencyKey, canonical, &result)
	if err != nil {
		return result, err
	}
	if replay {
		record, e := gatewayruntime.ReadInvocationRecord(tx, op, result.Invocation.InvocationId)
		tx.Close()
		if e != nil {
			return result, e
		}
		if record.Invocation.State == "accepted" || record.Invocation.State == "reserved" {
			if _, e = StartAcceptedInvocation(ctx, options, record); e != nil {
				return result, e
			}
		}
		result.Invocation = record.Invocation
		return result, nil
	}
	if err = RequireInvocationLookupTracking(tx, op); err != nil {
		return result, err
	}
	var version int
	var enabled bool
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT version,enabled FROM models WHERE model_id=? AND data_epoch=?`, model.ModelId, op.DataEpoch).Scan(&version, &enabled)
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	if version != model.Version || !enabled {
		return result, platform.NewError(platform.CodeStaleResult, false, "诊断模型在批准期间已变化")
	}
	provider, err := ReadDispatchProvider(tx, op, input.ProviderId)
	if err != nil {
		return result, err
	}
	if !provider.Enabled || provider.CredentialVersion < 1 {
		return result, platform.NewError(platform.CodeDependencyUnavailable, false, "诊断渠道尚未启用或没有真实凭据")
	}
	request := gen.CreateInvocationInput{IdempotencyKey: input.IdempotencyKey, Messages: []gen.GatewayMessage{}, Texts: []string{}, CandidateIds: []string{}}
	// 一个供应方可同时挂对话和向量模型（向量走 /embeddings），按模型能力选探测方式。
	adapter := string(provider.Adapter)
	if slices.Contains(model.Capabilities, "embedding") {
		adapter = "openai_embeddings"
	}
	switch adapter {
	case "openai_embeddings", "gemini_embeddings":
		request.Capability = "embedding"
		request.Texts = []string{"OK"}
	case "rerank":
		request.Capability = "rerank"
		query := "OK"
		request.Query = &query
		request.Texts = []string{"OK"}
		request.CandidateIds = []string{"probe"}
	default:
		request.Capability = "chat"
		if !slices.Contains(model.Capabilities, "chat") && slices.Contains(model.Capabilities, "decision") {
			// 决策模型不生成文本：用一道是非题探测连接。
			request.Capability = "decision"
			request.Decision = &gen.DecisionRequest{State: "OK", Questions: map[string]gen.DecisionQuestion{"ok": {Type: "noul", Instructions: "Does the state say OK?"}}}
			break
		}
		if !slices.Contains(model.Capabilities, "chat") && slices.Contains(model.Capabilities, "translation") {
			request.Capability = "translation"
		}
		request.MaximumOutputTokens = 16
		request.Messages = []gen.GatewayMessage{{Role: "user", Content: "OK"}}
	}
	if !slices.Contains(model.Capabilities, string(request.Capability)) {
		return result, platform.NewError(platform.CodeUnsupportedCapability, false, "所选模型没有诊断适配器需要的明确能力")
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	if bound := now.Add(30 * time.Second); deadline.After(bound) {
		deadline = bound
	}
	op.Deadline = deadline.Format(time.RFC3339Nano)
	// 内部诊断请求没有AI task/profile/QA许可；其原始JSON及显式策略都被原executionHash覆盖。
	requestRaw, _ := platform.CanonicalJSON(struct {
		IdempotencyKey      string               `json:"idempotencyKey"`
		Capability          string               `json:"capability"`
		Messages            []gen.GatewayMessage `json:"messages"`
		Texts               []string             `json:"texts"`
		Candidates          []string             `json:"candidateIds"`
		Query               *string              `json:"query,omitempty"`
		MaximumOutputTokens int                  `json:"maximumOutputTokens"`
		Decision            *gen.DecisionRequest `json:"decision,omitempty"`
	}{input.IdempotencyKey, string(request.Capability), request.Messages, request.Texts, request.CandidateIds, request.Query, request.MaximumOutputTokens, request.Decision})
	policy := gatewayruntime.ProbePolicy{Kind: "admin-probe", SchemaVersion: 1, ApprovedBy: op.Actor.SubjectID, PermissionRevision: int64(op.Actor.PermissionRevision)}
	policyRaw, _ := platform.CanonicalJSON(policy)
	id := platform.NewIdentifier()
	fixed := gatewayruntime.FixedRoute{Version: "admin-probe-v1", Candidates: []gatewayruntime.FixedCandidate{{Model: model, Provider: provider, CredentialVersion: provider.CredentialVersion, Price: price}}, RequiredCapabilities: []string{string(request.Capability)}, AssetVersions: map[string]int{}, Fallback: false}
	routeRaw, _ := platform.CanonicalJSON(fixed)
	record := gatewayruntime.InvocationRecord{Invocation: gen.Invocation{InvocationId: id, RootOperationId: op.RootOperationID, CostClass: gen.CostClass(op.CostClass), State: "accepted", Deadline: deadline, PriceSnapshotId: price.PriceSnapshotId, Attempts: []gen.Attempt{}}, Request: request, Operation: op, Route: routeRaw, Probe: &policy, CreatedNS: now.UnixNano()}
	if _, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO connection_probes(invocation_id,provider_id,model_id,data_epoch,created_ns) VALUES(?,?,?,?,?)`, id, input.ProviderId, input.ModelId, op.DataEpoch, record.CreatedNS); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	accepted, err := AcceptPreparedInvocation(ctx, op, tx, record, fixed, requestRaw, policyRaw, "model-gateway.CreateConnectionProbe", input.IdempotencyKey, canonical, options)
	if err != nil {
		return result, err
	}
	result.Invocation = accepted.Invocation
	return result, nil
}
