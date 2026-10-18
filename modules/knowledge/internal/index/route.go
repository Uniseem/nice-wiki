package index

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type TaskRoute struct {
	TaskID       string             `json:"taskId"`
	Profile      ProfileDeclaration `json:"profile"`
	Assignment   *TaskAssignment    `json:"assignment,omitempty"`
	Models       []ModelDeclaration `json:"models"`
	RouteVersion string             `json:"routeVersion"`
}

// ReadTaskRoute 只消费MG同一SQL快照真实eligible候选和opaque版本，不复制候选筛选/覆盖/hash算法。
func ReadTaskRoute(ctx context.Context, op OperationContext, runtime Runtime, taskID string) (TaskRoute, error) {
	result := TaskRoute{TaskID: taskID, Models: []ModelDeclaration{}}
	if runtime.Gateway == nil || runtime.Validators["GatewayResolveTaskRouteResult"] == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, false, "免费任务路由权威未装配")
	}
	service := op
	service.CallerService = "knowledge"
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	raw, err := runtime.Gateway.ResolveTaskRoute(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service), struct {
		TaskID    string `json:"taskId"`
		CostClass string `json:"costClass"`
	}{taskID, op.CostClass})
	if err != nil {
		return result, err
	}
	if err = runtime.Validators["GatewayResolveTaskRouteResult"](raw); err != nil {
		return result, err
	}
	var authority struct {
		Resolved struct {
			Profile    ProfileDeclaration `json:"profile"`
			Assignment TaskAssignment     `json:"assignment"`
		} `json:"resolved"`
		Candidates []struct {
			Model             ModelDeclaration    `json:"model"`
			ModelSnapshotHash string              `json:"modelSnapshotHash"`
			EmbeddingSpace    *gen.EmbeddingSpace `json:"embeddingSpace,omitempty"`
		} `json:"candidates"`
		RouteVersion string `json:"routeVersion"`
	}
	if json.Unmarshal(raw, &authority) != nil || authority.Resolved.Assignment.TaskID != taskID || authority.Resolved.Profile.CostClass != op.CostClass {
		return result, platform.NewError(platform.CodeIntegrityError, false, "任务路由范围或费用类别不一致")
	}
	result.Profile = authority.Resolved.Profile
	result.Assignment = &authority.Resolved.Assignment
	result.RouteVersion = authority.RouteVersion
	for _, candidate := range authority.Candidates {
		model := candidate.Model
		if model.MaximumInputTokens <= 0 {
			return result, platform.NewError(platform.CodeUnsupportedCapability, false, "任务候选未声明实际输入窗口")
		}
		model.ModelSnapshotHash = candidate.ModelSnapshotHash
		model.EmbeddingSpace = candidate.EmbeddingSpace
		result.Models = append(result.Models, model)
	}
	return result, nil
}
