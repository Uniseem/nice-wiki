package runtime

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
)

// CheckLanguageSuggestionApproval每次新付费尝试读取localization持久管理员批准，不信任调用方自报enabled。
func CheckLanguageSuggestionApproval(ctx context.Context, op OperationContext, input gen.CreateInvocationInput) error {
	if input.TaskId != "localization.suggest" {
		if input.LanguageSuggestionApprovalId != nil {
			return platform.NewError(platform.CodeValidation, false, "语言建议批准仅适用于该任务")
		}
		return nil
	}
	if op.CostClass != platform.CostClassBackground || input.LanguageSuggestionApprovalId == nil {
		return platform.NewError(platform.CodeForbidden, false, "语言建议须管理员明确开启且使用后台费用")
	}
	options, err := ExecutionDependencies()
	if err != nil {
		return err
	}
	if options.LanguageApprovals == nil || options.Validators["LanguageSuggestionApprovalResult"] == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "语言建议批准权威未装配")
	}
	service := op
	service.CallerService = "model-gateway"
	service.Actor = platform.AuthContext{Kind: "service", SubjectID: "model-gateway", SiteID: op.Actor.SiteID, Grants: []string{}, ExpiresAt: op.Deadline}
	work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service)
	raw, err := options.LanguageApprovals.ReadLanguageSuggestionApproval(work, *input.LanguageSuggestionApprovalId)
	if err != nil {
		return err
	}
	var approval struct {
		ApprovalID string `json:"approvalId"`
		Enabled    bool   `json:"enabled"`
		SiteID     string `json:"siteId"`
		DataEpoch  string `json:"dataEpoch"`
		Version    int    `json:"version"`
		ApprovedAt string `json:"approvedAt"`
	}
	if options.Validators["LanguageSuggestionApprovalResult"](raw) != nil || json.Unmarshal(raw, &approval) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "语言建议批准权威响应不符合合同")
	}
	if !approval.Enabled || approval.ApprovalID != *input.LanguageSuggestionApprovalId || approval.SiteID != op.Actor.SiteID || approval.DataEpoch != op.DataEpoch {
		return platform.NewError(platform.CodeForbidden, false, "语言建议持久批准已停用、换版或不属于原站点代际")
	}
	return nil
}
