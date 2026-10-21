package runtime

import (
	"context"
	"encoding/json"

	"nice-wiki/libs/platform"
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
			MaxInputTokens int `json:"maxInputTokens"`
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

// ResolveTaskWindow 返回任务路线上各备用模型输入窗口的最小值，任何一个备用模型都放得下；路线未声明窗口时返回 0。
func ResolveTaskWindow(ctx context.Context, op OperationContext, taskID string) (int, error) {
	route, err := resolveTaskRoute(ctx, op, taskID)
	if err != nil {
		return 0, err
	}
	window := 0
	for _, candidate := range route.Candidates {
		if limit := candidate.Model.MaxInputTokens; limit > 0 && (window == 0 || limit < window) {
			window = limit
		}
	}
	return window, nil
}
