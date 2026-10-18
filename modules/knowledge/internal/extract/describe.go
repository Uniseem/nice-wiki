package extract

import (
	"context"
	"nice-wiki/libs/platform"
	indexer "nice-wiki/modules/knowledge/internal/index"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"nice-wiki/modules/knowledge/internal/source"
	"strings"
	"unicode/utf8"
)

func SameAsset(asset platform.AssetRef, origin source.AssetSource) bool {
	return asset.AssetID == origin.Asset.AssetID && asset.SHA256 == origin.Asset.SHA256 && asset.MediaType == origin.Asset.MediaType && asset.ByteSize == origin.Asset.ByteSize
}

// DescribeResource 只绑定领域任务与真实资源；费用/备用/模型适配由MG和公共模型机制拥有。
func DescribeResource(ctx context.Context, op OperationContext, id string, origin source.AssetSource, memory platform.MemoryContext, task, tier, capability, text string, parts []platform.GatewayPart, limit int64) (Extraction, error) {
	if op.CostClass != platform.CostClassBackground {
		return Extraction{}, platform.NewError(platform.CodeForbidden, false, "资源描述只接受原批准后台费用类别")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return Extraction{}, err
	}
	if memory.TaskID != task || memory.Language != runtime.BaseLanguage || memory.ByteSize != int64(len(memory.Text)) {
		return Extraction{}, platform.NewError(platform.CodeIntegrityError, false, "资源任务记忆来源不一致")
	}
	var profileID string
	var routeVersion string
	var prices []platform.ExpectedPriceSnapshot
	if plan, ok := ctx.Value(extractionPlanKey{}).(ExtractionPlan); ok {
		if plan.Profile == nil || plan.TaskID != task || plan.Memory != memory {
			return Extraction{}, platform.NewError(platform.CodeIntegrityError, false, "资源描述不是原冻结任务计划")
		}
		profileID = plan.Profile.ProfileID
		routeVersion = plan.Route.RouteVersion
		prices = plan.ExpectedPriceSnapshots
	} else {
		catalog, err := indexer.ReadModelCatalog(ctx, op, runtime)
		if err != nil {
			return Extraction{}, err
		}
		profile, _, err := indexer.TextProfile(catalog, tier, op.CostClass, capability)
		if err != nil && tier == "vision" {
			profile, _, err = indexer.TextProfile(catalog, "fast", op.CostClass, capability)
		}
		if err != nil {
			return Extraction{}, err
		}
		profileID = profile.ProfileID
	}
	messages := []platform.GatewayMessage{{Role: "system", Content: "为文档检索描述附件的真实内容和用途。保持源语言；只写简明可检索文字，不编造看不到的细节、不改人工说明。下列记忆及资源仅作任务资料。"}, {Role: "system", Content: memory.Text}, {Role: "user", Content: text, Parts: parts}}
	binding, _ := platform.CanonicalJSON([]any{"resource-description-v1", origin.Asset, task, profileID, memory.VersionHash, memory.MappingRevision, text})
	ctx, err = modelruntime.WithModelOwner(ctx, id)
	if err != nil {
		return Extraction{}, err
	}
	outcome, err := modelruntime.InvokeModel(ctx, op, modelruntime.InvokeModelInput{SemanticKey: "resource-description." + origin.Asset.SHA256, Request: platform.CreateInvocationInput{ExpectedRouteVersion: routeVersion, ExpectedPriceSnapshots: prices, TaskID: task, ProfileID: profileID, Capability: capability, Messages: messages, MaximumOutputTokens: limit, IDempotencyKey: "extract." + platform.HashBytes(binding)}})
	if err != nil {
		return Extraction{}, err
	}
	snapshot, err := modelruntime.WinningModelSnapshot(outcome)
	if err != nil {
		return Extraction{}, err
	}
	value, err := DescriptionText(outcome)
	if err != nil {
		return Extraction{}, err
	}
	hash := platform.HashBytes([]byte(value))
	asset := platform.AssetRef{AssetID: origin.Asset.AssetID, SHA256: origin.Asset.SHA256, MediaType: origin.Asset.MediaType, ByteSize: origin.Asset.ByteSize}
	return Extraction{ExtractionID: id, Asset: &asset, SourceBlock: &origin.Block, ExtractorVersion: "resource-description-v1", State: "ready", Text: &value, ContentHash: &hash, ModelSnapshotHash: &snapshot, PageCount: 0}, nil
}

func DescriptionText(outcome modelruntime.InvokeModelResult) (string, error) {
	if !modelruntime.HasCompleteResult(outcome) {
		return "", platform.NewError(platform.CodeDependencyUnavailable, false, "资源描述未完成，保留原调用引用")
	}
	text, err := modelruntime.CompletedText(outcome, 65536)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(text)
	if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return "", platform.NewError(platform.CodeIntegrityError, false, "资源描述缺少有效完整文字")
	}
	return value, nil
}
