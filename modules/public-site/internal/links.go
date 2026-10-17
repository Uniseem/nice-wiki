package publicsite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/public-site/internal/gen"
)

// 正文核验上限：2MiB 来自 LoadPublishedBundle，块 ID 总数对齐 content 稳定 ID 上限。
// blockDepthLimit 只停止无界递归，超出视为无法核验，不据此伪造不可见。
const (
	linkCaller         = "site-config"
	linkBudget         = 2500 * time.Millisecond
	publishedBodyLimit = 2 << 20
	blockDepthLimit    = 256
	maxBlockIDs        = 10000
	bodySchemaVersion  = 1
)

var (
	canonicalPathPattern = regexp.MustCompile(`^/[^/\\?#\x00-\x20\x7F][^\\?#\x00-\x20\x7F]*$`)
	relativeLinkPattern  = regexp.MustCompile(`^/[^/?#\\\s][^?#\\\s]*(#[^?#\\\s]+)?$`)
)

// LinkDirectory 是公开链接解析使用的版本化只读客户端。
type LinkDirectory struct {
	SiteConfig   *gen.SiteConfigClient
	Content      *gen.ContentClient
	Localization *gen.LocalizationClient
}

type linkDirectoryKey struct{}

// WithLinkDirectory 把已装配的权威客户端放进调用链。
func WithLinkDirectory(ctx context.Context, directory LinkDirectory) context.Context {
	return context.WithValue(ctx, linkDirectoryKey{}, directory)
}

// ResolvePublishedLink 为 site-config 纯服务返回当前安全公开相对 URL。
// 未知依赖返回错误，不用 visible=false 掩盖。
func ResolvePublishedLink(ctx context.Context, op platform.OperationContext, input gen.ResolvePublishedLinkInput) (gen.ResolvePublishedLinkResult, error) {
	if err := authorizeLink(ctx, op); err != nil {
		return gen.ResolvePublishedLinkResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, linkBudget)
	defer cancel()
	ctx = platform.WithOperationContext(ctx, op)
	if err := ctx.Err(); err != nil {
		return gen.ResolvePublishedLinkResult{}, normalizeLinkError(ctx, err)
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, err
	}

	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, normalizeLinkError(ctx, err)
	}
	languages, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, normalizeLinkError(ctx, err)
	}
	tree, err := readTree(ctx, directory.Content)
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, normalizeLinkError(ctx, err)
	}
	node, found, err := findDocument(tree, input.DocumentId)
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, err
	}
	if !found {
		if err = confirmAbsence(ctx, directory, site, languages, tree.revision, input.DocumentId); err != nil {
			return gen.ResolvePublishedLinkResult{}, err
		}
		return hiddenResult(gen.Unavailable)
	}
	if !canonicalPathPattern.MatchString(node.canonicalPath) || !validEncodedPath(node.canonicalPath) {
		return gen.ResolvePublishedLinkResult{}, dependency("公开规范路径无法核验")
	}
	if string(node.reference.DataEpoch) != string(op.DataEpoch) || node.reference.DocumentId != input.DocumentId {
		return gen.ResolvePublishedLinkResult{}, stale()
	}

	reason := gen.ResolvePublishedLinkResult1Reason("")
	if _, ok := languages.versions[input.Language]; !ok {
		reason = gen.LanguageUnavailable
	}
	var blockID string
	if reason == "" && input.BlockId != nil {
		blockID = string(*input.BlockId)
		missing, err := anchorMissing(ctx, directory.Content, node.reference, blockID)
		if err != nil {
			return gen.ResolvePublishedLinkResult{}, normalizeLinkError(ctx, err)
		}
		if missing {
			reason = gen.AnchorMissing
		}
	}
	var href string
	if reason == "" {
		href, err = relativeLink(ctx, op, site.publicBaseURL, input.Language, node.canonicalPath, blockID)
		if err != nil {
			return gen.ResolvePublishedLinkResult{}, normalizeLinkError(ctx, err)
		}
	}
	if err = confirmPublication(ctx, directory, op, site, languages, tree.revision, node); errors.Is(err, errWithdrawn) {
		return hiddenResult(gen.Unavailable)
	} else if err != nil {
		return gen.ResolvePublishedLinkResult{}, err
	}
	if reason != "" {
		return hiddenResult(reason)
	}
	return visibleResult(href, input.Language, node.reference)
}

func authorizeLink(ctx context.Context, op platform.OperationContext) error {
	if op.CallerService == "" || op.Actor.Kind == "" {
		return platform.NewError(platform.CodeUnauthenticated, false, "服务身份缺失")
	}
	if op.CallerService != linkCaller || op.Actor.Kind != "service" || op.Actor.SessionID != "" || platform.PortalProofFrom(ctx) != "" {
		return platform.NewError(platform.CodeForbidden, false, "仅site-config纯服务可解析公开链接")
	}
	if op.DataEpoch == "" {
		return platform.NewError(platform.CodeVersionMismatch, false, "数据代次不是当前权威代次")
	}
	return nil
}

func directoryFrom(ctx context.Context) (LinkDirectory, error) {
	directory, ok := ctx.Value(linkDirectoryKey{}).(LinkDirectory)
	if !ok || directory.SiteConfig == nil || directory.Content == nil || directory.Localization == nil {
		return LinkDirectory{}, dependency("公开链接权威客户端未装配")
	}
	return directory, nil
}

type siteSnapshot struct {
	publicBaseURL string
	baseLanguage  string
	revision      int
}

type languageSnapshot struct {
	versions map[string]int
}

type treeSnapshot struct {
	revision int
	nodes    []publicNode
}

type publicNode struct {
	documentID         string
	canonicalPath      string
	visibilityRevision int
	reference          gen.PublishedRevisionRef
}

func readSiteSnapshot(ctx context.Context, client *gen.SiteConfigClient) (siteSnapshot, error) {
	raw, err := client.ReadSiteConfig(ctx)
	if err != nil {
		return siteSnapshot{}, err
	}
	var envelope struct {
		Config *struct {
			PublicBaseURL *string `json:"publicBaseURL"`
			BaseLanguage  *string `json:"baseLanguage"`
			Revision      *int    `json:"revision"`
		} `json:"config"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil || envelope.Config == nil || envelope.Config.PublicBaseURL == nil || envelope.Config.BaseLanguage == nil || envelope.Config.Revision == nil {
		return siteSnapshot{}, dependency("站点配置权威不完整")
	}
	if *envelope.Config.PublicBaseURL == "" || *envelope.Config.BaseLanguage == "" || *envelope.Config.Revision < 0 {
		return siteSnapshot{}, dependency("站点配置权威不完整")
	}
	return siteSnapshot{
		publicBaseURL: *envelope.Config.PublicBaseURL,
		baseLanguage:  *envelope.Config.BaseLanguage,
		revision:      *envelope.Config.Revision,
	}, nil
}

func readLanguageSnapshot(ctx context.Context, client *gen.LocalizationClient) (languageSnapshot, error) {
	query := url.Values{}
	query.Set("includeDisabled", "false")
	raw, err := client.ListLanguages(ctx, query)
	if err != nil {
		return languageSnapshot{}, err
	}
	var envelope struct {
		Languages []struct {
			Tag     *string `json:"tag"`
			Version *int    `json:"version"`
			Enabled *bool   `json:"enabled"`
		} `json:"languages"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil || envelope.Languages == nil {
		return languageSnapshot{}, dependency("启用语言权威不完整")
	}
	versions := make(map[string]int, len(envelope.Languages))
	for _, language := range envelope.Languages {
		if language.Tag == nil || language.Version == nil || language.Enabled == nil || !*language.Enabled || *language.Tag == "" || *language.Version < 0 {
			return languageSnapshot{}, dependency("启用语言权威不完整")
		}
		if _, exists := versions[*language.Tag]; exists {
			return languageSnapshot{}, dependency("启用语言权威重复")
		}
		versions[*language.Tag] = *language.Version
	}
	return languageSnapshot{versions: versions}, nil
}

func readTree(ctx context.Context, client *gen.ContentClient) (treeSnapshot, error) {
	raw, err := client.ReadPublicTree(ctx)
	if err != nil {
		return treeSnapshot{}, err
	}
	var envelope struct {
		Nodes *[]struct {
			DocumentID         *string                   `json:"documentId"`
			CanonicalPath      *string                   `json:"canonicalPath"`
			VisibilityRevision *int                      `json:"visibilityRevision"`
			Reference          *gen.PublishedRevisionRef `json:"reference"`
		} `json:"nodes"`
		TreeRevision *int `json:"treeRevision"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil || envelope.Nodes == nil || envelope.TreeRevision == nil || *envelope.TreeRevision < 0 {
		return treeSnapshot{}, dependency("公开树权威不完整")
	}
	nodes := make([]publicNode, 0, len(*envelope.Nodes))
	for _, item := range *envelope.Nodes {
		if item.DocumentID == nil || item.CanonicalPath == nil || item.VisibilityRevision == nil || item.Reference == nil || *item.VisibilityRevision < 0 {
			return treeSnapshot{}, dependency("公开树节点不完整")
		}
		nodes = append(nodes, publicNode{
			documentID:         *item.DocumentID,
			canonicalPath:      *item.CanonicalPath,
			visibilityRevision: *item.VisibilityRevision,
			reference:          *item.Reference,
		})
	}
	return treeSnapshot{revision: *envelope.TreeRevision, nodes: nodes}, nil
}

func findDocument(tree treeSnapshot, documentID string) (publicNode, bool, error) {
	var found publicNode
	seen := false
	for _, node := range tree.nodes {
		if node.documentID != documentID {
			continue
		}
		if seen {
			return publicNode{}, false, dependency("公开树文档标识重复")
		}
		seen = true
		found = node
	}
	return found, seen, nil
}

func anchorMissing(ctx context.Context, client *gen.ContentClient, ref gen.PublishedRevisionRef, blockID string) (bool, error) {
	if !platform.ValidIdentifier(blockID) {
		return false, dependency("公开正文块标识无法核验")
	}
	query := url.Values{}
	query.Set("publishedRevisionId", ref.PublishedRevisionId)
	raw, err := client.ReadPublishedRevision(ctx, ref.DocumentId, query)
	if err != nil {
		return false, publicationReadError(err)
	}
	var envelope struct {
		Reference          *gen.PublishedRevisionRef `json:"reference"`
		Body               json.RawMessage           `json:"body"`
		VisibilityRevision *int                      `json:"visibilityRevision"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil || envelope.Reference == nil || len(envelope.Body) == 0 || envelope.VisibilityRevision == nil {
		return false, dependency("公开正文权威不完整")
	}
	if !samePublication(*envelope.Reference, ref) {
		return false, stale()
	}
	if err = verifyPublishedBody(envelope.Body, ref); err != nil {
		return false, err
	}
	counts, err := blockIDCounts(envelope.Body)
	if err != nil {
		return false, err
	}
	for id, count := range counts {
		if count > 1 || !platform.ValidIdentifier(id) {
			return false, dependency("公开正文块标识无法核验")
		}
	}
	return counts[blockID] != 1, nil
}

func publicationReadError(err error) error {
	var public *platform.Error
	if errors.As(err, &public) && (public.Code == platform.CodeNotFound || public.Code == platform.CodeVersionMismatch) {
		return stale()
	}
	return err
}

func verifyPublishedBody(body json.RawMessage, ref gen.PublishedRevisionRef) error {
	if len(body) > publishedBodyLimit || !platform.ValidHash(platform.Hash(ref.ContentHash)) || ref.SchemaVersion != bodySchemaVersion {
		return dependency("公开正文与发布引用不一致")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return dependency("公开正文无法核验")
	}
	object, ok := value.(map[string]any)
	if !ok || object["type"] != "doc" {
		return dependency("公开正文无法核验")
	}
	number, ok := object["schemaVersion"].(json.Number)
	parsed, parseErr := number.Int64()
	if !ok || parseErr != nil || parsed != bodySchemaVersion {
		return dependency("公开正文与发布引用不一致")
	}
	if err := orderMarks(value, 0); err != nil {
		return err
	}
	canonical, err := platform.CanonicalJSON(value)
	if err != nil || len(canonical) > publishedBodyLimit {
		return dependency("公开正文无法核验")
	}
	if string(platform.HashBytes(canonical)) != string(ref.ContentHash) {
		return dependency("公开正文与发布引用不一致")
	}
	return nil
}

func orderMarks(value any, depth int) error {
	if depth > blockDepthLimit {
		return dependency("公开正文无法核验")
	}
	switch node := value.(type) {
	case map[string]any:
		if marks, ok := node["marks"]; ok && marks != nil {
			list, ok := marks.([]any)
			if !ok {
				return dependency("公开正文无法核验")
			}
			for _, mark := range list {
				item, ok := mark.(map[string]any)
				kind, kindOK := "", false
				if ok {
					kind, kindOK = item["type"].(string)
				}
				if !ok || !kindOK || kind == "" {
					return dependency("公开正文无法核验")
				}
			}
			sort.SliceStable(list, func(i, j int) bool {
				return list[i].(map[string]any)["type"].(string) < list[j].(map[string]any)["type"].(string)
			})
		}
		for _, child := range node {
			if err := orderMarks(child, depth + 1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := orderMarks(child, depth + 1); err != nil {
				return err
			}
		}
	}
	return nil
}

func blockIDCounts(body json.RawMessage) (map[string]int, error) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, dependency("公开正文无法核验")
	}
	counts := map[string]int{}
	if err := walkBlockIDs(value, 0, counts); err != nil {
		return nil, err
	}
	return counts, nil
}

func walkBlockIDs(value any, depth int, counts map[string]int) error {
	if depth > blockDepthLimit {
		return dependency("公开正文块层级无法核验")
	}
	switch node := value.(type) {
	case map[string]any:
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if raw, exists := attrs["blockId"]; exists {
				id, ok := raw.(string)
				if !ok {
					return dependency("公开正文块标识无法核验")
				}
				counts[id]++
				if len(counts) > maxBlockIDs {
					return dependency("公开正文块标识无法核验")
				}
			}
		}
		for _, child := range node {
			if err := walkBlockIDs(child, depth + 1, counts); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := walkBlockIDs(child, depth + 1, counts); err != nil {
				return err
			}
		}
	}
	return nil
}

func relativeLink(ctx context.Context, op platform.OperationContext, baseURL, language, canonicalPath, blockID string) (string, error) {
	if canonicalPath == "/" + language || strings.HasPrefix(canonicalPath, "/" + language + "/") {
		return "", dependency("公开规范路径含语言前缀")
	}
	built, err := BuildCanonical(ctx, op, BuildCanonicalInput{
		BaseURL:       baseURL,
		CanonicalPath: "/" + url.PathEscape(language) + canonicalPath,
	})
	if err != nil {
		return "", dependency("公开规范地址无法组成")
	}
	parsed, err := url.Parse(built.CanonicalURL)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Host == "" {
		return "", dependency("公开规范地址无法组成")
	}
	href := parsed.EscapedPath()
	if blockID != "" {
		href += "#" + url.PathEscape(blockID)
	}
	if !relativeLinkPattern.MatchString(href) {
		return "", dependency("公开规范地址无法组成")
	}
	return href, nil
}

func confirmAbsence(ctx context.Context, directory LinkDirectory, site siteSnapshot, languages languageSnapshot, treeRevision int, documentID string) error {
	if err := confirmWatermarks(ctx, directory, site, languages); err != nil {
		return err
	}
	again, err := readTree(ctx, directory.Content)
	if err != nil {
		return normalizeLinkError(ctx, err)
	}
	if again.revision != treeRevision {
		return stale()
	}
	_, found, err := findDocument(again, documentID)
	if err != nil {
		return err
	}
	if found {
		return stale()
	}
	return nil
}

func confirmPublication(ctx context.Context, directory LinkDirectory, op platform.OperationContext, site siteSnapshot, languages languageSnapshot, treeRevision int, node publicNode) error {
	if err := confirmWatermarks(ctx, directory, site, languages); err != nil {
		return err
	}
	again, err := readTree(ctx, directory.Content)
	if err != nil {
		return normalizeLinkError(ctx, err)
	}
	if again.revision != treeRevision {
		return stale()
	}
	current, found, err := findDocument(again, node.documentID)
	if err != nil {
		return err
	}
	if !found {
		return withdrawn()
	}
	if current.canonicalPath != node.canonicalPath || current.visibilityRevision != node.visibilityRevision || !samePublication(current.reference, node.reference) {
		return stale()
	}
	pathQuery := url.Values{}
	pathQuery.Set("path", node.canonicalPath)
	pathRaw, err := directory.Content.ResolvePublicPath(ctx, pathQuery)
	if err != nil {
		return normalizeLinkError(ctx, err)
	}
	if err = confirmPath(pathRaw, node); err != nil {
		return err
	}
	validationRaw, err := directory.Content.ValidatePublications(ctx, map[string]any{
		"refs": []gen.PublishedRevisionRef{node.reference},
	})
	if err != nil {
		return normalizeLinkError(ctx, err)
	}
	return confirmValidation(validationRaw, node, op)
}

func confirmWatermarks(ctx context.Context, directory LinkDirectory, site siteSnapshot, languages languageSnapshot) error {
	againSite, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return normalizeLinkError(ctx, err)
	}
	if againSite != site {
		return stale()
	}
	againLanguages, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil {
		return normalizeLinkError(ctx, err)
	}
	if len(againLanguages.versions) != len(languages.versions) {
		return stale()
	}
	for tag, version := range languages.versions {
		other, ok := againLanguages.versions[tag]
		if !ok || other != version {
			return stale()
		}
	}
	return nil
}

func confirmPath(raw json.RawMessage, node publicNode) error {
	var result struct {
		State         *string                   `json:"state"`
		DocumentID    *string                   `json:"documentId"`
		Reference     *gen.PublishedRevisionRef `json:"reference"`
		CanonicalPath *string                   `json:"canonicalPath"`
		RouteRevision *int                      `json:"routeRevision"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.State == nil || result.RouteRevision == nil {
		return dependency("公开路径权威不完整")
	}
	switch *result.State {
	case "gone", "not_found":
		return withdrawn()
	case "redirect":
		return stale()
	case "found":
		if result.DocumentID == nil || result.Reference == nil || result.CanonicalPath == nil {
			return dependency("公开路径权威不完整")
		}
		if *result.DocumentID != node.documentID || *result.CanonicalPath != node.canonicalPath || !samePublication(*result.Reference, node.reference) {
			return stale()
		}
		return nil
	default:
		return dependency("公开路径权威不完整")
	}
}

func confirmValidation(raw json.RawMessage, node publicNode, op platform.OperationContext) error {
	var envelope struct {
		Validations []struct {
			Reference          *gen.PublishedRevisionRef `json:"reference"`
			Visible            *bool                     `json:"visible"`
			MatchesCurrent     *bool                     `json:"matchesCurrent"`
			VisibilityRevision *int                      `json:"visibilityRevision"`
		} `json:"validations"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Validations) != 1 {
		return dependency("公开引用权威不完整")
	}
	item := envelope.Validations[0]
	if item.Reference == nil || item.Visible == nil || item.MatchesCurrent == nil || item.VisibilityRevision == nil {
		return dependency("公开引用权威不完整")
	}
	if !*item.Visible {
		return withdrawn()
	}
	if !*item.MatchesCurrent || *item.VisibilityRevision != node.visibilityRevision || !samePublication(*item.Reference, node.reference) {
		return stale()
	}
	if string(item.Reference.DataEpoch) != string(op.DataEpoch) {
		return stale()
	}
	return nil
}

func samePublication(left, right gen.PublishedRevisionRef) bool {
	return left.DocumentId == right.DocumentId &&
		left.PublishedRevisionId == right.PublishedRevisionId &&
		left.Revision == right.Revision &&
		left.ContentHash == right.ContentHash &&
		left.SchemaVersion == right.SchemaVersion &&
		left.DataEpoch == right.DataEpoch
}

func hiddenResult(reason gen.ResolvePublishedLinkResult1Reason) (gen.ResolvePublishedLinkResult, error) {
	var result gen.ResolvePublishedLinkResult
	err := result.FromResolvePublishedLinkResult1(gen.ResolvePublishedLinkResult1{
		Reason:  reason,
		Visible: gen.False,
	})
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, platform.NewError(platform.CodeInternal, false, "公开链接结果无法编码")
	}
	return result, nil
}

func visibleResult(href, language string, ref gen.PublishedRevisionRef) (gen.ResolvePublishedLinkResult, error) {
	var result gen.ResolvePublishedLinkResult
	err := result.FromResolvePublishedLinkResult0(gen.ResolvePublishedLinkResult0{
		Href:             href,
		Publication:      ref,
		ResolvedLanguage: language,
		Reason:           gen.Available,
		Visible:          gen.True,
	})
	if err != nil {
		return gen.ResolvePublishedLinkResult{}, platform.NewError(platform.CodeInternal, false, "公开链接结果无法编码")
	}
	return result, nil
}

func dependency(message string) error {
	return platform.NewError(platform.CodeDependencyUnavailable, true, "%s", message)
}

func stale() error {
	return platform.NewError(platform.CodeStaleResult, true, "公开链接权威水位已变化")
}

func withdrawn() error {
	return errWithdrawn
}

var errWithdrawn = errors.New("withdrawn")

func normalizeLinkError(ctx context.Context, err error) error {
	if err == nil || errors.Is(err, errWithdrawn) {
		return err
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "公开链接解析已到截止时间")
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, false, "公开链接解析已取消")
	}
	var public *platform.Error
	if errors.As(err, &public) {
		return public
	}
	return dependency("公开链接权威不可用")
}
