package runtime

import (
	"context"
	"encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/zcode"
)

// taskRoute 是网关任务路线里本域用到的部分：生效档位和按序备用模型的输入窗口。
type taskRoute struct {
	Resolved struct {
		Profile struct {
			ProfileID string `json:"profileId"`
		} `json:"profile"`
	} `json:"resolved"`
	Candidates []struct {
		Model struct {
			MaxInputTokens  int    `json:"maxInputTokens"`
			MaxOutputTokens int    `json:"maxOutputTokens"`
			ProviderID      string `json:"providerId"`
			ModelID         string `json:"modelId"`
		} `json:"model"`
	} `json:"candidates"`
}

func resolveTaskRoute(ctx context.Context, op OperationContext, taskID string) (taskRoute, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return taskRoute{}, err
	}
	if runtime.Models == nil {
		return taskRoute{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型任务路由未装配")
	}
	raw, err := runtime.Models.ResolveTaskRoute(ServiceReadContext(ctx, op), struct {
		TaskID    string `json:"taskId"`
		CostClass string `json:"costClass"`
	}{taskID, op.CostClass})
	if err != nil {
		return taskRoute{}, err
	}
	var route taskRoute
	if json.Unmarshal(raw, &route) != nil || route.Resolved.Profile.ProfileID == "" {
		return taskRoute{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务档位解析结果不完整")
	}
	return route, nil
}

// ResolveTaskProfile 向模型网关解析某任务在当前费用类别下的档位，不创建调用或费用。
func ResolveTaskProfile(ctx context.Context, op OperationContext, taskID string) (string, error) {
	route, err := resolveTaskRoute(ctx, op, taskID)
	if err != nil {
		return "", err
	}
	return route.Resolved.Profile.ProfileID, nil
}

// ResolveTaskModel 采用路由首选模型的默认窗口，备用模型不缩小主模型上下文。
func ResolveTaskModel(ctx context.Context, op OperationContext, taskID string, selected ...string) (zcode.Model, error) {
	route, err := resolveTaskRoute(ctx, op, taskID)
	if err != nil {
		return zcode.Model{}, err
	}
	if len(route.Candidates) == 0 {
		return zcode.Model{}, platform.NewError(platform.CodeDependencyUnavailable, false, "回答模型未配置")
	}
	model := route.Candidates[0].Model
	if len(selected) > 0 && selected[0] != "" {
		found := false
		for _, candidate := range route.Candidates {
			if candidate.Model.ModelID == selected[0] {
				model = candidate.Model
				found = true
				break
			}
		}
		if !found {
			return zcode.Model{}, platform.NewError(platform.CodeDependencyUnavailable, false, "实际模型窗口元数据不可读")
		}
	}
	if model.MaxInputTokens < 1 || model.MaxOutputTokens < 1 {
		return zcode.Model{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型默认窗口元数据不完整")
	}
	return zcode.Model{ProviderID: model.ProviderID, ModelID: model.ModelID, ContextWindow: model.MaxInputTokens, MaxOutputTokens: model.MaxOutputTokens}, nil
}

func ResolveTaskWindow(ctx context.Context, op OperationContext, taskID string) (int, error) {
	model, err := ResolveTaskModel(ctx, op, taskID)
	return model.ContextWindow, err
}
