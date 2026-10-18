package publicsite

import (
	"context"
	"net/url"
	"nice-wiki/libs/platform"
	"sort"
	"strings"
)

type BuildHreflangInput struct {
	DocumentID string
	DataEpoch  string
	BaseURL    string
}
type BuildHreflangResult struct{ Alternates []AlternateLocale }

// BuildHreflang 保留缺译语言的 URL，停用语言不进入 alternate。
func BuildHreflang(ctx context.Context, op platform.OperationContext, in BuildHreflangInput) (BuildHreflangResult, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return BuildHreflangResult{}, err
	}
	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return BuildHreflangResult{}, err
	}
	if site.publicBaseURL != in.BaseURL {
		return BuildHreflangResult{}, stale()
	}
	languages, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil {
		return BuildHreflangResult{}, err
	}
	tree, err := readTree(ctx, directory.Content)
	if err != nil {
		return BuildHreflangResult{}, err
	}
	node, found, err := findDocument(tree, in.DocumentID)
	if err != nil {
		return BuildHreflangResult{}, err
	}
	if !found {
		return BuildHreflangResult{}, platform.NewError(platform.CodeNotFound, false, "NOT_PUBLISHED")
	}
	if string(node.reference.DataEpoch) != in.DataEpoch || in.DataEpoch != string(op.DataEpoch) {
		return BuildHreflangResult{}, platform.NewError(platform.CodeVersionMismatch, false, "EPOCH_MISMATCH")
	}
	tags := make([]string, 0, len(languages.versions))
	for tag := range languages.versions {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	alternates := make([]AlternateLocale, 0, len(tags)+1)
	for _, tag := range tags {
		canonical, err := BuildCanonical(ctx, op, BuildCanonicalInput{in.BaseURL, "/" + url.PathEscape(tag) + node.canonicalPath})
		if err != nil {
			return BuildHreflangResult{}, err
		}
		alternates = append(alternates, AlternateLocale{tag, canonical.CanonicalURL})
	}
	base, err := parsePublicBase(in.BaseURL)
	if err != nil {
		return BuildHreflangResult{}, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/"
	base.RawPath = ""
	alternates = append(alternates, AlternateLocale{"x-default", base.String()})
	if err = confirmPublication(ctx, directory, op, site, languages, tree.revision, node); err != nil {
		return BuildHreflangResult{}, err
	}
	return BuildHreflangResult{alternates}, nil
}
