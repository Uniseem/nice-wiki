package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/modules/content/internal/gen"
)

// ModelRuntime 是本域的模型网关端口：付费、续读、取消由平台统一实现，本域只解析任务档位、读取完整文本并按哈希缓存结果。
type ModelRuntime struct {
	gateway       platform.GatewayClient
	routes        *gen.ModelGatewayClient
	validateRoute func(json.RawMessage) error
	store         *Store
}

func NewModelRuntime(client *gen.ModelGatewayClient, store *Store) (*ModelRuntime, error) {
	validators := map[string]func(json.RawMessage) error{}
	for _, name := range []string{"CreateInvocationResult", "CancelInvocationResult", "StreamEvent", "GetInvocationResult", "LookupInvocationResult", "GatewayResolveTaskRouteResult"} {
		validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if err != nil {
			return nil, err
		}
		validators[name] = validator
	}
	gateway, err := modelclient.NewGatewayClient(client, modelclient.GatewayResponseValidators{"CreateInvocationResult": validators["CreateInvocationResult"], "CancelInvocationResult": validators["CancelInvocationResult"], "StreamEvent": validators["StreamEvent"], "GetInvocationResult": validators["GetInvocationResult"], "LookupInvocationResult": validators["LookupInvocationResult"]})
	if err != nil {
		return nil, err
	}
	return &ModelRuntime{gateway: gateway, routes: client, validateRoute: validators["GatewayResolveTaskRouteResult"], store: store}, nil
}

// modelOperation 把后台请求换成本服务身份、后台费用类别的调用上下文；根操作和截止沿用原请求。
func modelOperation(op OperationContext) OperationContext {
	op.CallerService = "content"
	op.CostClass = platform.CostClassBackground
	op.Actor = platform.AuthContext{Kind: "service", SubjectID: "content", SiteID: op.Actor.SiteID, Grants: []string{}, ExpiresAt: op.Deadline}
	return op
}

// InvokeModel 按任务当前档位取一次模型完整文本。同一任务、档位和输入哈希的结果存在本域，再次请求直接复用、不再付费；
// 网关幂等键带本次根操作，只让同一请求的重试复用原调用。
func InvokeModel(ctx context.Context, op OperationContext, models *ModelRuntime, taskID string, inputHash Hash, messages []platform.GatewayMessage, maxOutputTokens int64) (string, error) {
	work := modelOperation(op)
	call := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), work)
	raw, err := models.routes.ResolveTaskRoute(call, map[string]string{"taskId": taskID, "costClass": work.CostClass})
	if err != nil {
		// 任务没分配档位或档位不存在就是“没配 AI”，让后台提示手填；其他失败原样返回。
		var problem *platform.Error
		if errors.As(err, &problem) && problem.Code == platform.CodeNotFound {
			return "", platform.NewError(platform.CodeUnsupportedCapability, false, "任务没有可用的模型档位")
		}
		return "", err
	}
	var route struct {
		Resolved struct {
			Profile struct {
				ProfileID Identifier `json:"profileId"`
			} `json:"profile"`
		} `json:"resolved"`
		RouteVersion Hash `json:"routeVersion"`
	}
	if models.validateRoute(raw) != nil || json.Unmarshal(raw, &route) != nil || route.Resolved.Profile.ProfileID == "" {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "模型任务档位解析结果不完整")
	}
	inputKey := platform.HashBytes([]byte(route.Resolved.Profile.ProfileID + "\x00" + inputHash))
	// 空回复不算结果（例如模型把输出额度都花在思考上），不复用，重新问。
	if text, found, err := models.store.readModelSuggestion(ctx, op, taskID, inputKey); err != nil || found && strings.TrimSpace(text) != "" {
		return text, err
	}
	key := taskID + "." + platform.HashBytes([]byte(op.RootOperationID+"\x00"+inputKey))
	outcome, err := modelclient.InvokeModel(call, models.gateway, work, platform.InvokeModelRequest{IDempotencyKey: key, Request: platform.CreateInvocationInput{
		TaskID: taskID, ProfileID: route.Resolved.Profile.ProfileID, ExpectedRouteVersion: route.RouteVersion, Capability: platform.CapabilityChat,
		Messages: messages, MaximumOutputTokens: maxOutputTokens, IDempotencyKey: key}})
	if err != nil {
		return "", err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil || outcome.CurrentSnapshot.Output.Text == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "模型没有给出完整结果")
	}
	text := *outcome.CurrentSnapshot.Output.Text
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	if err := models.store.saveModelSuggestion(ctx, op, taskID, inputKey, text); err != nil {
		return "", err
	}
	return text, nil
}

func (s *Store) readModelSuggestion(ctx context.Context, op OperationContext, taskID string, inputKey Hash) (string, bool, error) {
	var text string
	err := s.db.QueryRowContext(ctx, `SELECT output_text FROM model_suggestions WHERE site_id=? AND data_epoch=? AND task_id=? AND input_key=?`, op.Actor.SiteID, op.DataEpoch, taskID, inputKey).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, contentStorageError()
	}
	return text, true, nil
}

func (s *Store) saveModelSuggestion(ctx context.Context, op OperationContext, taskID string, inputKey Hash, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO model_suggestions(site_id,data_epoch,task_id,input_key,output_text,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(site_id,data_epoch,task_id,input_key) DO UPDATE SET output_text=excluded.output_text,created_at=excluded.created_at WHERE model_suggestions.output_text=''`, op.Actor.SiteID, op.DataEpoch, taskID, inputKey, text, platform.Now()); err != nil {
		return contentStorageError()
	}
	return nil
}
