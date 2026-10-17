package siteconfig

import (
	"context"
	"strings"
	"time"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type footerRequest struct {
	Kind  string                `json:"kind"`
	Input gen.UpdateFooterInput `json:"input"`
}

// UpdateFooter 只改页脚字段，并走既有 settings 回执、CAS 与 outbox。
func UpdateFooter(ctx context.Context, op platform.OperationContext, input gen.UpdateFooterInput) (gen.UpdateFooterResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.UpdateFooterResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.UpdateFooterResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.settingsMu.Lock()
	defer store.settingsMu.Unlock()
	if input.Footer.Links == nil {
		input.Footer.Links = []gen.FooterLink{}
	}
	requestRaw, err := platform.CanonicalJSON(footerRequest{Kind: "footer", Input: input})
	if err != nil {
		return gen.UpdateFooterResult{}, err
	}
	requestHash := platform.HashBytes(requestRaw)
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.UpdateFooterResult{}, err
	}
	if receipt.State == "committed" {
		config, err := store.readConfigOperationResult(ctx, op, requestHash, receipt.ResultRevision)
		return gen.UpdateFooterResult{Config: config}, err
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.UpdateFooterResult{}, err
	}
	if err := validateFooterUpdate(input.Footer); err != nil {
		return gen.UpdateFooterResult{}, err
	}
	var candidate configCandidate
	if receipt.State == "in_progress" {
		candidate, err = store.readConfigCandidate(ctx, store.db, op.OperationID, "footer")
		if err == nil && !footerInputMatches(candidate.Config, input) {
			err = platform.NewError(platform.CodeIntegrityError, false, "页脚候选与原始请求不一致")
		}
	} else {
		current, readErr := store.loadCurrentConfig(ctx, op)
		if readErr != nil {
			return gen.UpdateFooterResult{}, readErr
		}
		if current.Revision != input.ExpectedRevision {
			return gen.UpdateFooterResult{}, platform.NewError(platform.CodeVersionMismatch, false, "配置expectedRevision已变化")
		}
		config := current
		config.Footer = input.Footer
		config.Revision, config.UpdatedAt = current.Revision+1, time.Now().UTC()
		store.stateMu.RLock()
		localHTTP := store.allowLocalHTTP
		store.stateMu.RUnlock()
		if _, err := ValidateSiteConfig(ValidateSiteConfigInput{Config: config, AllowLocalHTTP: localHTTP}); err != nil {
			return gen.UpdateFooterResult{}, err
		}
		candidate, err = store.reserveConfigCandidate(ctx, op, "footer", requestHash, config)
	}
	if err != nil {
		return gen.UpdateFooterResult{}, err
	}
	written, err := WriteConfigRevision(ctx, op, WriteConfigRevisionInput{Config: candidate.Config, RevisionId: candidate.RevisionID})
	if err != nil {
		return gen.UpdateFooterResult{}, err
	}
	committed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash, ChangedSections: []string{"footer"}})
	if err != nil {
		return gen.UpdateFooterResult{}, err
	}
	return gen.UpdateFooterResult{Config: committed.Config}, nil
}

func validateFooterUpdate(footer gen.FooterConfig) error {
	if footer.Links == nil {
		return platform.NewError(platform.CodeValidation, false, "页脚链接必须是数组")
	}
	if footer.IcpNumber != nil {
		if len([]rune(*footer.IcpNumber)) < 1 || len([]rune(*footer.IcpNumber)) > 100 || strings.ContainsAny(*footer.IcpNumber, "<>") || strings.IndexFunc(*footer.IcpNumber, unicode.IsControl) >= 0 {
			return platform.NewError(platform.CodeValidation, false, "备案号须为纯文本")
		}
	}
	for _, link := range footer.Links {
		if err := validateConfigLink(link.Target); err != nil {
			return err
		}
	}
	if footer.IcpLink != nil {
		if err := validateConfigWebLink(*footer.IcpLink, false); err != nil {
			return err
		}
	}
	return validateConfigWebLink(footer.SourceCodeURL, false)
}

func footerInputMatches(config gen.SiteConfig, input gen.UpdateFooterInput) bool {
	if config.Revision != input.ExpectedRevision+1 {
		return false
	}
	left, err := platform.CanonicalJSON(config.Footer)
	if err != nil {
		return false
	}
	right, err := platform.CanonicalJSON(input.Footer)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}
