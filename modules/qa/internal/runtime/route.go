package runtime

import (
	"context"
	"encoding/json"

	"nice-wiki/libs/platform"
)

// ResolveTaskProfile 向模型网关解析某任务在当前费用类别下的档位，不创建调用或费用。
func ResolveTaskProfile(ctx context.Context, op OperationContext, taskID string) (string, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return "", err
	}
	if runtime.Models == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "模型任务路由未装配")
	}
	raw, err := runtime.Models.ResolveTaskRoute(ServiceReadContext(ctx, op), struct {
		TaskID    string `json:"taskId"`
		CostClass string `json:"costClass"`
	}{taskID, op.CostClass})
	if err != nil {
		return "", err
	}
	var reply struct {
		Resolved struct {
			Profile struct {
				ProfileID string `json:"profileId"`
			} `json:"profile"`
		} `json:"resolved"`
	}
	if json.Unmarshal(raw, &reply) != nil || reply.Resolved.Profile.ProfileID == "" {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "任务档位解析结果不完整")
	}
	return reply.Resolved.Profile.ProfileID, nil
}
