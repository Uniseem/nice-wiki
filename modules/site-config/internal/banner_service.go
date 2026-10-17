package siteconfig

import (
	"context"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type bannerRequest struct {
	Kind  string                `json:"kind"`
	Input gen.UpdateBannerInput `json:"input"`
}

// UpdateBanner 写入不可变公告正文后，把横幅指针放进同一条 settings 提交。
func UpdateBanner(ctx context.Context, op platform.OperationContext, input gen.UpdateBannerInput) (gen.UpdateBannerResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.UpdateBannerResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.UpdateBannerResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.settingsMu.Lock()
	defer store.settingsMu.Unlock()
	validated, err := ValidateAnnouncementBody(ValidateAnnouncementBodyInput{Body: input.Body})
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	if err := validateBannerWindow(input); err != nil {
		return gen.UpdateBannerResult{}, err
	}
	requestRaw, err := platform.CanonicalJSON(bannerRequest{Kind: "banner", Input: input})
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	requestHash := platform.HashBytes(requestRaw)
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	if receipt.State == "committed" {
		config, err := store.readConfigOperationResult(ctx, op, requestHash, receipt.ResultRevision)
		return gen.UpdateBannerResult{Config: config}, err
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.UpdateBannerResult{}, err
	}
	var candidate configCandidate
	var current gen.SiteConfig
	if receipt.State == "in_progress" {
		candidate, err = store.readConfigCandidate(ctx, store.db, op.OperationID, "banner")
		if err == nil && !bannerCandidateMatches(candidate.Config, input, validated.Hash) {
			err = platform.NewError(platform.CodeIntegrityError, false, "公告候选与原始请求不一致")
		}
	} else {
		current, err = store.loadCurrentConfig(ctx, op)
		if err != nil {
			return gen.UpdateBannerResult{}, err
		}
		if current.Revision != input.ExpectedRevision {
			return gen.UpdateBannerResult{}, platform.NewError(platform.CodeVersionMismatch, false, "配置expectedRevision已变化")
		}
		announcementID, visibility, idErr := bannerIdentity(current, input, validated.Hash)
		if idErr != nil {
			return gen.UpdateBannerResult{}, idErr
		}
		config, buildErr := bannerFromInput(current, input, announcementID, gen.Identifier(platform.NewIdentifier()), visibility, time.Now().UTC(), validated.Hash)
		if buildErr != nil {
			return gen.UpdateBannerResult{}, buildErr
		}
		candidate, err = store.reserveConfigCandidate(ctx, op, "banner", requestHash, config)
	}
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	if candidate.Config.Banner == nil {
		return gen.UpdateBannerResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告候选缺少横幅")
	}
	writtenBody, err := WriteAnnouncementRevision(ctx, op, WriteAnnouncementRevisionInput{
		AnnouncementId: candidate.Config.Banner.AnnouncementId, RevisionId: candidate.Config.Banner.RevisionId, Body: input.Body, BodyHash: validated.Hash,
	})
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	var expected *gen.Identifier
	if input.AnnouncementId != nil {
		if receipt.State == "in_progress" {
			current, err = store.loadCurrentConfig(ctx, op)
			if err != nil {
				return gen.UpdateBannerResult{}, err
			}
		}
		if current.Banner == nil || current.Banner.AnnouncementId != *input.AnnouncementId {
			return gen.UpdateBannerResult{}, platform.NewError(platform.CodeValidation, false, "公告标识与当前横幅不一致")
		}
		id := current.Banner.RevisionId
		expected = &id
	}
	if _, err := CommitAnnouncementHead(ctx, op, CommitAnnouncementHeadInput{AnnouncementId: candidate.Config.Banner.AnnouncementId, Object: writtenBody.Object, ExpectedDraftRevisionId: expected}); err != nil {
		return gen.UpdateBannerResult{}, err
	}
	written, err := WriteConfigRevision(ctx, op, WriteConfigRevisionInput{Config: candidate.Config, RevisionId: candidate.RevisionID})
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	committed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash, ChangedSections: []string{"banner"}})
	if err != nil {
		return gen.UpdateBannerResult{}, err
	}
	return gen.UpdateBannerResult{Config: committed.Config}, nil
}

func validateBannerWindow(input gen.UpdateBannerInput) error {
	if (input.StartsAt != nil && input.StartsAt.IsZero()) || (input.EndsAt != nil && input.EndsAt.IsZero()) {
		return platform.NewError(platform.CodeValidation, false, "公告时间窗不合法")
	}
	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return platform.NewError(platform.CodeValidation, false, "公告结束时间早于开始时间")
	}
	return nil
}

func bannerIdentity(current gen.SiteConfig, input gen.UpdateBannerInput, hash gen.Hash) (gen.Identifier, gen.Revision, error) {
	if input.AnnouncementId == nil {
		return gen.Identifier(platform.NewIdentifier()), 1, nil
	}
	if !platform.ValidIdentifier(string(*input.AnnouncementId)) || current.Banner == nil || current.Banner.AnnouncementId != *input.AnnouncementId {
		return "", 0, platform.NewError(platform.CodeValidation, false, "公告标识与当前横幅不一致")
	}
	visibility := current.Banner.VisibilityRevision
	if visibility < 1 {
		return "", 0, platform.NewError(platform.CodeIntegrityError, false, "当前公告可见性修订不合法")
	}
	if current.Banner.BodyHash != hash || current.Banner.Enabled != input.Enabled || !sameConfigTime(current.Banner.StartsAt, input.StartsAt) || !sameConfigTime(current.Banner.EndsAt, input.EndsAt) {
		visibility++
	}
	return *input.AnnouncementId, visibility, nil
}

func bannerFromInput(current gen.SiteConfig, input gen.UpdateBannerInput, announcementID, revisionID gen.Identifier, visibility gen.Revision, updatedAt time.Time, hash gen.Hash) (gen.SiteConfig, error) {
	config := current
	config.Revision = current.Revision + 1
	config.UpdatedAt = updatedAt.UTC()
	config.Banner = &gen.Announcement{
		AnnouncementId: announcementID, BodyHash: hash, Enabled: input.Enabled, EndsAt: input.EndsAt, Revision: config.Revision,
		RevisionId: revisionID, SiteId: config.SiteId, StartsAt: input.StartsAt, UpdatedAt: config.UpdatedAt, VisibilityRevision: visibility,
	}
	return config, nil
}

func bannerCandidateMatches(config gen.SiteConfig, input gen.UpdateBannerInput, hash gen.Hash) bool {
	banner := config.Banner
	if banner == nil || config.Revision != input.ExpectedRevision+1 || banner.BodyHash != hash || banner.Enabled != input.Enabled {
		return false
	}
	if input.AnnouncementId != nil && *input.AnnouncementId != banner.AnnouncementId {
		return false
	}
	return sameConfigTime(banner.StartsAt, input.StartsAt) && sameConfigTime(banner.EndsAt, input.EndsAt)
}
