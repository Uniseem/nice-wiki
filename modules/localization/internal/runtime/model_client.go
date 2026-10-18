package runtime

import (
	"context"
	"encoding/json"
	"math"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/localization/internal/gen"
)

type InvokeModelInput = platform.InvokeModelRequest
type InvokeModelResult = platform.InvokeModelOutcome

// PlanTaskRoute 是免费任务路由的保守主候选快照。
type PlanTaskRoute struct {
	TaskID            string
	ProfileID         string
	CostClass         string
	RouteVersion      platform.Hash
	ModelID           string
	ModelSnapshotHash platform.Hash
}

// ConfigureExecutionAdapters 仅注入平台适配器，构造期间不探测或调用模型。
func ConfigureExecutionAdapters(gateway platform.GatewayClient, publisher platform.EventPublisher) error {
	store, _ := LanguageGateway()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化存储未装配")
	}
	store.gateway = gateway
	store.publisher = publisher
	return nil
}

// ConfigurePlanAdapters 装配免费路由、token 估价、报价与任务提交端口。
func ConfigurePlanAdapters(gateway *gen.ModelGatewayClient, jobs *gen.JobsClient, validators map[string]func(json.RawMessage) error) error {
	store, _ := LanguageGateway()
	if store == nil || gateway == nil || jobs == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化计划生产方未装配")
	}
	for _, name := range []string{"GatewayResolveTaskRouteResult", "GatewayEstimateTokensResult", "GatewayQuoteCostResult", "JobsCreateJobResult"} {
		if validators[name] == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化计划响应校验器未装配")
		}
	}
	store.planGateway = gateway
	store.planJobs = jobs
	store.planValidators = validators
	return nil
}

func validatePlanPayload(name string, raw json.RawMessage) error {
	store, _ := LanguageGateway()
	if store == nil || store.planValidators[name] == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化计划响应校验未装配")
	}
	return store.planValidators[name](raw)
}

// ResolvePlanTaskRoute 只读取当前 eligible 主候选，不创建调用或费用。
func ResolvePlanTaskRoute(ctx context.Context, op OperationContext, taskID string) (PlanTaskRoute, error) {
	store, _ := LanguageGateway()
	if store == nil || store.planGateway == nil {
		return PlanTaskRoute{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型任务路由未装配")
	}
	raw, err := store.planGateway.ResolveTaskRoute(SourceOperation(ctx, op), struct {
		TaskID    string `json:"taskId"`
		CostClass string `json:"costClass"`
	}{taskID, op.CostClass})
	if err != nil {
		return PlanTaskRoute{}, err
	}
	if err = validatePlanPayload("GatewayResolveTaskRouteResult", raw); err != nil {
		return PlanTaskRoute{}, err
	}
	var response struct {
		Resolved struct {
			Profile struct {
				ProfileID string `json:"profileId"`
				CostClass string `json:"costClass"`
			} `json:"profile"`
			Assignment struct {
				TaskID string `json:"taskId"`
			} `json:"assignment"`
		} `json:"resolved"`
		Candidates []struct {
			Model struct {
				ModelID string `json:"modelId"`
			} `json:"model"`
			ModelSnapshotHash string `json:"modelSnapshotHash"`
		} `json:"candidates"`
		RouteVersion string `json:"routeVersion"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Resolved.Assignment.TaskID != taskID ||
		response.Resolved.Profile.CostClass != op.CostClass || response.Resolved.Profile.ProfileID == "" ||
		len(response.Candidates) == 0 || !platform.ValidHash(response.RouteVersion) ||
		!platform.ValidIdentifier(response.Candidates[0].Model.ModelID) ||
		!platform.ValidHash(response.Candidates[0].ModelSnapshotHash) {
		return PlanTaskRoute{}, platform.NewError(platform.CodeIntegrityError, false, "模型任务路由作用域或主候选不一致")
	}
	return PlanTaskRoute{
		TaskID:            taskID,
		ProfileID:         response.Resolved.Profile.ProfileID,
		CostClass:         response.Resolved.Profile.CostClass,
		RouteVersion:      platform.Hash(response.RouteVersion),
		ModelID:           response.Candidates[0].Model.ModelID,
		ModelSnapshotHash: platform.Hash(response.Candidates[0].ModelSnapshotHash),
	}, nil
}

// EstimatePlanTextTokens 按生产方接口上限分批估算完整 Unicode 文本，不截断。
func EstimatePlanTextTokens(ctx context.Context, op OperationContext, modelID string, texts []string) (int64, error) {
	store, _ := LanguageGateway()
	if store == nil || store.planGateway == nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "免费token估算未装配")
	}
	if !platform.ValidIdentifier(modelID) {
		return 0, platform.NewError(platform.CodeValidation, false, "token估算模型标识不合法")
	}
	chunks := make([]string, 0, 256)
	var total int64
	flush := func() error {
		if len(chunks) == 0 {
			return nil
		}
		raw, err := store.planGateway.EstimateTokens(SourceOperation(ctx, op), struct {
			ModelID  string                    `json:"modelId"`
			Texts    []string                  `json:"texts"`
			Messages []platform.GatewayMessage `json:"messages"`
		}{modelID, chunks, []platform.GatewayMessage{}})
		if err != nil {
			return err
		}
		if err = validatePlanPayload("GatewayEstimateTokensResult", raw); err != nil {
			return err
		}
		var response struct {
			InputTokens      int64  `json:"inputTokens"`
			EstimatorVersion string `json:"estimatorVersion"`
		}
		if json.Unmarshal(raw, &response) != nil || response.InputTokens < 0 || response.EstimatorVersion == "" ||
			response.InputTokens > math.MaxInt64-total {
			return platform.NewError(platform.CodeIntegrityError, false, "免费token估算回执不合法")
		}
		total += response.InputTokens
		chunks = chunks[:0]
		return nil
	}
	for _, text := range texts {
		if !utf8.ValidString(text) {
			return 0, platform.NewError(platform.CodeIntegrityError, false, "待估文本不是有效UTF8")
		}
		for len(text) > 0 {
			end := 0
			for end < len(text) && end < 4096 {
				_, size := utf8.DecodeRuneInString(text[end:])
				end += size
			}
			chunks = append(chunks, text[:end])
			text = text[end:]
			if len(chunks) == 256 {
				if err := flush(); err != nil {
					return 0, err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return total, nil
}

// QuotePlanCost 只读当前真实固定价目，不写预算、额度或账本。
func QuotePlanCost(ctx context.Context, op OperationContext, modelID string, inputTokens, maximumOutputTokens int64) (gen.QuoteCostResult, error) {
	store, _ := LanguageGateway()
	if store == nil || store.planGateway == nil {
		return gen.QuoteCostResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "免费费用报价未装配")
	}
	raw, err := store.planGateway.QuoteCost(SourceOperation(ctx, op), struct {
		ModelID             string `json:"modelId"`
		InputTokens         int64  `json:"inputTokens"`
		MaximumOutputTokens int64  `json:"maximumOutputTokens"`
	}{modelID, inputTokens, maximumOutputTokens})
	if err != nil {
		return gen.QuoteCostResult{}, err
	}
	if err = validatePlanPayload("GatewayQuoteCostResult", raw); err != nil {
		return gen.QuoteCostResult{}, err
	}
	var result gen.QuoteCostResult
	if json.Unmarshal(raw, &result) != nil || result.PriceSnapshot.ModelId != modelID ||
		(result.PricingKnown && result.MaximumCost == nil) || (!result.PricingKnown && result.MaximumCost != nil) {
		return gen.QuoteCostResult{}, platform.NewError(platform.CodeIntegrityError, false, "免费费用报价回执不合法")
	}
	return result, nil
}

// CreatePlanJob 持久提交一条任务并按严格响应识别真实接受回执。
func CreatePlanJob(ctx context.Context, op OperationContext, input any, idempotencyKey string) (gen.AcceptedJob, error) {
	store, _ := LanguageGateway()
	if store == nil || store.planJobs == nil {
		return gen.AcceptedJob{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化计划任务端口未装配")
	}
	raw, err := store.planJobs.CreateJob(SourceOperation(ctx, op), input, idempotencyKey)
	if err != nil {
		return gen.AcceptedJob{}, err
	}
	if err = validatePlanPayload("JobsCreateJobResult", raw); err != nil {
		return gen.AcceptedJob{}, err
	}
	var response struct {
		Job gen.AcceptedJob `json:"job"`
	}
	if json.Unmarshal(raw, &response) != nil || !platform.ValidIdentifier(response.Job.JobId) ||
		!platform.ValidIdentifier(response.Job.OperationId) || response.Job.State != "queued" {
		return gen.AcceptedJob{}, platform.NewError(platform.CodeIntegrityError, false, "任务接受回执不合法")
	}
	return response.Job, nil
}
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error) {
	store, _ := LanguageGateway()
	if store == nil || store.gateway == nil {
		return InvokeModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化模型适配器未装配")
	}
	return modelclient.InvokeModel(ctx, store.gateway, op, input)
}

// PublishBatch 使用平台单批outbox；租约、旧代丢弃和失败退避只由平台实现。
func PublishBatch(ctx context.Context, limit int) (platform.RelayBatchResult, error) {
	store, runtimeClient := LanguageGateway()
	if store == nil || runtimeClient == nil || store.publisher == nil {
		return platform.RelayBatchResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化事件发布依赖未装配")
	}
	return outbox.RunRelayBatch(ctx, outbox.NewStore(store.db), store.publisher, platform.RelayConfig{BatchLimit: limit, PollMillis: 1000, Runtime: runtimeClient.ReadRuntimeState, WriteWindow: func(work context.Context) (time.Time, error) {
		var state string
		if err := store.db.QueryRowContext(work, `SELECT state FROM local_barrier WHERE singleton=1`).Scan(&state); err != nil {
			return time.Time{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "本地化屏障不可读取")
		}
		if state != "released" {
			return time.Time{}, platform.NewError(platform.CodeLocked, true, "本地化屏障暂停事件转发")
		}
		return time.Time{}, nil
	}})
}
