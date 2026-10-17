package siteconfig

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

var themeColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type brandingRequest struct {
	Kind  string                  `json:"kind"`
	Input gen.UpdateBrandingInput `json:"input"`
}

type brandingAssetUse struct {
	ReferenceID string
	Asset       gen.AssetVersionRef
	Kind        string
	Animated    *bool
}

// brandingReferenceMaterial 复用首页引用结构；空切片仍参与 digest，以便媒体签发空 pin。
func brandingReferenceMaterial(config gen.SiteConfig, revisionID string) ([]homeResourceReference, string, error) {
	sourceID := referenceSourceObjectID("branding", config.SiteId)
	uses := []struct {
		id    string
		asset *gen.AssetVersionRef
	}{
		{"branding.logo", config.Logo},
		{"branding.favicon", config.Favicon},
		{"branding.share", config.DefaultShareImage},
	}
	references := make([]homeResourceReference, 0)
	for _, use := range uses {
		if use.asset == nil {
			continue
		}
		references = append(references, homeResourceReference{SourceService: "site-config", SourceObjectID: sourceID,
			SourceRevisionID: revisionID, SourceVersion: config.Revision, ReferenceID: use.id, Asset: *use.asset, Kind: "pending_pin"})
	}
	sort.Slice(references, func(i, j int) bool {
		left, right := references[i], references[j]
		if left.ReferenceID != right.ReferenceID {
			return left.ReferenceID < right.ReferenceID
		}
		if left.Asset.AssetId != right.Asset.AssetId {
			return left.Asset.AssetId < right.Asset.AssetId
		}
		if left.Asset.Version != right.Asset.Version {
			return left.Asset.Version < right.Asset.Version
		}
		return left.Asset.Sha256 < right.Asset.Sha256
	})
	raw, err := platform.CanonicalJSON(references)
	if err != nil {
		return nil, "", err
	}
	return references, platform.HashBytes(raw), nil
}

func brandingAssets(config gen.SiteConfig) []brandingAssetUse {
	uses := make([]brandingAssetUse, 0)
	if config.Logo != nil {
		uses = append(uses, brandingAssetUse{ReferenceID: "branding.logo", Asset: *config.Logo, Kind: "image"})
	}
	if config.Favicon != nil {
		uses = append(uses, brandingAssetUse{ReferenceID: "branding.favicon", Asset: *config.Favicon, Kind: "image"})
	}
	if config.DefaultShareImage != nil {
		uses = append(uses, brandingAssetUse{ReferenceID: "branding.share", Asset: *config.DefaultShareImage, Kind: "image"})
	}
	return uses
}

func validateBrandingInput(input gen.UpdateBrandingInput) error {
	if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 120 || strings.IndexFunc(input.Name, unicode.IsControl) >= 0 || !themeColorPattern.MatchString(input.ThemeColor) {
		return platform.NewError(platform.CodeValidation, false, "站名或主题色格式不合法")
	}
	for _, asset := range []*gen.AssetVersionRef{input.Logo, input.Favicon, input.DefaultShareImage} {
		if asset != nil && !strings.HasPrefix(asset.MediaType, "image/") {
			return platform.NewError(platform.CodeValidation, false, "品牌资源必须是图片")
		}
	}
	return nil
}

func validateBrandingAssets(ctx context.Context, op platform.OperationContext, siteID string, config gen.SiteConfig) error {
	return validateMediaAssets(ctx, op, siteID, brandingAssets(config))
}

func validateMediaAssets(ctx context.Context, op platform.OperationContext, siteID string, uses []brandingAssetUse) error {
	missing := make([]string, 0)
	for _, use := range uses {
		err := readReadyMediaAsset(ctx, op, siteID, use)
		if err == nil {
			continue
		}
		var failure *platform.Error
		if errors.As(err, &failure) && (failure.Code == platform.CodeValidation || failure.Code == platform.CodeNotFound) {
			missing = append(missing, use.ReferenceID)
			continue
		}
		return err
	}
	if len(missing) > 0 {
		return platform.NewError(platform.CodeValidation, false, "资源引用缺失或未就绪："+strings.Join(missing, "、"))
	}
	return nil
}

func readReadyMediaAsset(ctx context.Context, op platform.OperationContext, siteID string, use brandingAssetUse) error {
	store := currentActiveStore()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	client := store.homeDependencies().Media
	if client == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "媒体资源客户端未装配")
	}
	remoteCtx, _ := homeServiceContext(ctx, op, siteID)
	raw, err := client.ReadAsset(remoteCtx, use.Asset.AssetId, url.Values{"version": []string{strconv.Itoa(use.Asset.Version)}})
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			return platform.NewError(platform.CodeValidation, false, "资源不存在或版本不符")
		}
		return err
	}
	var response struct {
		Asset struct {
			Ref      gen.AssetVersionRef `json:"ref"`
			SiteID   string              `json:"siteId"`
			Kind     string              `json:"kind"`
			State    string              `json:"state"`
			Animated bool                `json:"animated"`
		} `json:"asset"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "媒体资源响应损坏")
	}
	asset := response.Asset
	if asset.SiteID != siteID || asset.Ref != use.Asset || asset.State != "ready" || asset.Kind != use.Kind {
		return platform.NewError(platform.CodeValidation, false, "资源不属于本站、版本不符或尚未ready")
	}
	if use.Animated != nil && asset.Animated != *use.Animated {
		return platform.NewError(platform.CodeValidation, false, "资源动图声明与权威不符")
	}
	return nil
}

func brandingInputMatches(config gen.SiteConfig, input gen.UpdateBrandingInput) bool {
	return config.Revision == input.ExpectedRevision+1 && config.Name == input.Name && config.ThemeColor == input.ThemeColor &&
		sameAssetRef(config.Logo, input.Logo) && sameAssetRef(config.Favicon, input.Favicon) && sameAssetRef(config.DefaultShareImage, input.DefaultShareImage)
}

// UpdateBranding 把品牌资源接到既有 settings 提交链和首页 pin 决定表。
func UpdateBranding(ctx context.Context, op platform.OperationContext, input gen.UpdateBrandingInput) (gen.UpdateBrandingResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.UpdateBrandingResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.settingsMu.Lock()
	defer store.settingsMu.Unlock()
	requestRaw, err := platform.CanonicalJSON(brandingRequest{Kind: "branding", Input: input})
	if err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	requestHash := platform.HashBytes(requestRaw)
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	if receipt.State == "committed" {
		config, err := store.readConfigOperationResult(ctx, op, requestHash, receipt.ResultRevision)
		return gen.UpdateBrandingResult{Config: config}, err
	}
	if receipt.State == "in_progress" {
		intent, err := store.readHomeReferenceIntent(ctx, store.db, op.OperationID)
		if err != nil {
			return gen.UpdateBrandingResult{}, err
		}
		store.configMu.Lock()
		decided, decisionErr := store.committedHomeReference(ctx, intent)
		store.configMu.Unlock()
		if decisionErr != nil {
			return gen.UpdateBrandingResult{}, decisionErr
		}
		if decided {
			if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
				return gen.UpdateBrandingResult{}, err
			}
			record, err := readConfigRecord(configRecordPath(store.activeRoot, intent.SourceRevisionID, "COMMITTED"))
			if err != nil {
				return gen.UpdateBrandingResult{}, err
			}
			completed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: record.Object, ExpectedRevision: record.ExpectedRevision, RequestHash: requestHash, ChangedSections: record.ChangedSections})
			return gen.UpdateBrandingResult{Config: completed.Config}, err
		}
		if intent.Decision == "aborted" || intent.Decision == "rejected" {
			return gen.UpdateBrandingResult{}, homeTerminatedError(intent)
		}
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	if err := validateBrandingInput(input); err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	var candidate configCandidate
	if receipt.State == "in_progress" {
		candidate, err = store.readConfigCandidate(ctx, store.db, op.OperationID, "branding")
		if err == nil && !brandingInputMatches(candidate.Config, input) {
			err = platform.NewError(platform.CodeIntegrityError, false, "品牌候选与原始请求不一致")
		}
	} else {
		current, readErr := store.loadCurrentConfig(ctx, op)
		if readErr != nil {
			return gen.UpdateBrandingResult{}, readErr
		}
		if current.Revision != input.ExpectedRevision {
			return gen.UpdateBrandingResult{}, platform.NewError(platform.CodeVersionMismatch, false, "配置expectedRevision已变化")
		}
		config := current
		config.Name, config.Logo, config.Favicon, config.ThemeColor, config.DefaultShareImage = input.Name, input.Logo, input.Favicon, input.ThemeColor, input.DefaultShareImage
		config.Revision, config.UpdatedAt = current.Revision+1, time.Now().UTC()
		candidate = configCandidate{Config: config}
	}
	if err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	if err := validateBrandingAssets(ctx, op, candidate.Config.SiteId, candidate.Config); err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	if receipt.State != "in_progress" {
		candidate, err = store.reserveConfigCandidate(ctx, op, "branding", requestHash, candidate.Config)
		if err != nil {
			return gen.UpdateBrandingResult{}, err
		}
	}
	intent, err := store.readHomeReferenceIntent(ctx, store.db, op.OperationID)
	if err != nil {
		return gen.UpdateBrandingResult{}, err
	}
	intent, err = store.acquireHomePin(ctx, op, intent)
	if err != nil {
		return gen.UpdateBrandingResult{}, store.finishHomeFailure(ctx, op, err)
	}
	written, err := WriteConfigRevision(ctx, op, WriteConfigRevisionInput{Config: candidate.Config, RevisionId: candidate.RevisionID})
	if err != nil {
		return gen.UpdateBrandingResult{}, store.finishHomeFailure(ctx, op, err)
	}
	committed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash, ChangedSections: []string{"branding"}})
	if err != nil {
		return gen.UpdateBrandingResult{}, store.finishHomeFailure(ctx, op, err)
	}
	return gen.UpdateBrandingResult{Config: committed.Config}, nil
}
