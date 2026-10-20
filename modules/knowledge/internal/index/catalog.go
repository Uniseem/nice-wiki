package index

import (
	"context"
	"encoding/json"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type ModelDeclaration struct {
	ModelID            string              `json:"modelId"`
	ModelVersion       string              `json:"providerModelVersion"`
	ModelSnapshotHash  string              `json:"modelSnapshotHash,omitempty"`
	Capabilities       []string            `json:"capabilities"`
	MaximumInputTokens int                 `json:"maxInputTokens"`
	Enabled            bool                `json:"enabled"`
	EmbeddingSpace     *gen.EmbeddingSpace `json:"embeddingSpace,omitempty"`
}
type ProfileDeclaration struct {
	Version   int64    `json:"version"`
	ProfileID string   `json:"profileId"`
	CostClass string   `json:"costClass"`
	Tier      string   `json:"tier"`
	Models    []string `json:"orderedModelIds"`
}
type ModelCatalog struct {
	Models   []ModelDeclaration
	Profiles []ProfileDeclaration
}

// ReadModelCatalog 只读网关权威无密钥目录，配置、选项和构建共同使用，不计算spaceId。
func ReadModelCatalog(ctx context.Context, op OperationContext, runtime Runtime) (ModelCatalog, error) {
	if runtime.Gateway == nil || runtime.Validators["GatewayReadModelsResult"] == nil || runtime.Validators["GatewayReadProfilesResult"] == nil {
		return ModelCatalog{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型目录权威未装配")
	}
	service := op
	service.CallerService = "knowledge"
	service.CostClass = platform.CostClassBackground
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service)
	modelsRaw, err := runtime.Gateway.ReadModels(work)
	if err != nil {
		return ModelCatalog{}, err
	}
	if err = runtime.Validators["GatewayReadModelsResult"](modelsRaw); err != nil {
		return ModelCatalog{}, err
	}
	profilesRaw, err := runtime.Gateway.ReadProfiles(work)
	if err != nil {
		return ModelCatalog{}, err
	}
	if err = runtime.Validators["GatewayReadProfilesResult"](profilesRaw); err != nil {
		return ModelCatalog{}, err
	}
	var models struct {
		Models []ModelDeclaration `json:"models"`
	}
	var profiles struct {
		Profiles []ProfileDeclaration `json:"profiles"`
	}
	if json.Unmarshal(modelsRaw, &models) != nil || json.Unmarshal(profilesRaw, &profiles) != nil {
		return ModelCatalog{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型目录响应不可读")
	}
	return ModelCatalog{Models: models.Models, Profiles: profiles.Profiles}, nil
}

// EmbeddingProfile 固定可信费用类别并要求完整空间，QA不能复用后台档位绕改费用。
func EmbeddingProfile(catalog ModelCatalog, profileID, costClass string, space gen.EmbeddingSpace) (ProfileDeclaration, ModelDeclaration, error) {
	for _, profile := range catalog.Profiles {
		if profileID != "" && profile.ProfileID != profileID {
			continue
		}
		if profile.CostClass != costClass || profile.Tier != "embedding" || len(profile.Models) == 0 {
			continue
		}
		for _, model := range catalog.Models {
			if model.ModelID == profile.Models[0] && model.Enabled && model.EmbeddingSpace != nil && *model.EmbeddingSpace == space {
				return profile, model, nil
			}
		}
	}
	return ProfileDeclaration{}, ModelDeclaration{}, platform.NewError(platform.CodeDependencyUnavailable, false, "当前费用类别未配置与索引完全相同的嵌入空间")
}

// TextProfile 选择可信费用类别下唯一档位的真实主模型，不复制网关覆盖或备用算法。
func TextProfile(catalog ModelCatalog, tier, costClass, capability string) (ProfileDeclaration, ModelDeclaration, error) {
	for _, profile := range catalog.Profiles {
		if profile.Tier != tier || profile.CostClass != costClass || len(profile.Models) == 0 {
			continue
		}
		for _, id := range profile.Models {
			for _, model := range catalog.Models {
				if model.ModelID == id && model.Enabled {
					for _, supported := range model.Capabilities {
						if supported == capability {
							return profile, model, nil
						}
					}
				}
			}
		}
	}
	return ProfileDeclaration{}, ModelDeclaration{}, platform.NewError(platform.CodeDependencyUnavailable, false, "本费用类别未配置所需文本模型档位")
}

type TaskAssignment struct {
	TaskID          string  `json:"taskId"`
	Tier            string  `json:"tier"`
	OverrideModelID *string `json:"overrideModelId,omitempty"`
	Version         int64   `json:"version"`
}

// ReadTaskAssignment 只读本域原任务覆盖；空列表表示未设置，不伪造一条配置记录。
func ReadTaskAssignment(ctx context.Context, op OperationContext, runtime Runtime, taskID string) (*TaskAssignment, error) {
	validate := runtime.Validators["GatewayReadTaskAssignmentsResult"]
	if runtime.Gateway == nil || validate == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "任务策略权威未装配")
	}
	service := op
	service.CallerService = "knowledge"
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	raw, err := runtime.Gateway.ReadTaskAssignments(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service), url.Values{"taskId": {taskID}})
	if err != nil {
		return nil, err
	}
	if err = validate(raw); err != nil {
		return nil, err
	}
	var result struct {
		Assignments []TaskAssignment `json:"assignments"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Assignments) > 1 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "单任务策略响应不完整")
	}
	if len(result.Assignments) == 0 {
		return nil, nil
	}
	if result.Assignments[0].TaskID != taskID {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "任务策略响应范围不一致")
	}
	return &result.Assignments[0], nil
}

// ProfileModelSnapshots 只收集当前安全声明的opaque来源hash，不重算MG哈希或选择winning。
func ProfileModelSnapshots(catalog ModelCatalog, profile ProfileDeclaration, assignment *TaskAssignment) []string {
	ids := append([]string{}, profile.Models...)
	if assignment != nil && assignment.OverrideModelID != nil {
		ids = []string{*assignment.OverrideModelID}
		if len(profile.Models) > 1 {
			ids = append(ids, profile.Models[1:]...)
		}
	}
	result := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		for _, model := range catalog.Models {
			if model.ModelID == id && model.Enabled && model.ModelSnapshotHash != "" && !seen[model.ModelSnapshotHash] {
				result = append(result, model.ModelSnapshotHash)
				seen[model.ModelSnapshotHash] = true
			}
		}
	}
	return result
}
