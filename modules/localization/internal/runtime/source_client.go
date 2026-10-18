package runtime

import (
	"context"
	"encoding/json"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
)

type PublishedSnapshot struct {
	Reference          gen.PublishedRevisionRef
	Body               json.RawMessage
	SourceLanguage     string
	Title              string
	VisibilityRevision int
}

// PublicTreeDocument 是 content 公开树中通过生产方 schema 校验的文档引用。
type PublicTreeDocument struct {
	DocumentID string
	Reference  gen.PublishedRevisionRef
}

// PublicTreeSnapshot 固定一次公开树 revision 与完整文档引用集合。
type PublicTreeSnapshot struct {
	TreeRevision int64
	Documents    []PublicTreeDocument
}

// ConfigureSourceClient 仅装配生成端口与严格生产方载荷schema，不访问源正文。
func ConfigureSourceClient(client *gen.ContentClient, validators map[string]func(json.RawMessage) error, key platform.SigningKey) error {
	store, _ := LanguageGateway()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化存储未装配")
	}
	store.content = client
	store.sourceValidators = validators
	store.cursorKey = key
	return nil
}
func ValidateSourcePayload(name string, raw json.RawMessage) error {
	store, _ := LanguageGateway()
	if store == nil || store.sourceValidators[name] == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化源响应校验器未装配")
	}
	if err := store.sourceValidators[name](raw); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "发布来源或目标树不符合生产方合同")
	}
	return nil
}
func SourceOperation(ctx context.Context, op OperationContext) context.Context {
	service := op
	service.CallerService = "localization"
	service.Actor = platform.AuthContext{Kind: "service", SubjectID: "localization", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	return platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service)
}
func FetchPublishedSnapshot(ctx context.Context, op OperationContext, document gen.PublishedRevisionRef) (PublishedSnapshot, error) {
	return readPublishedSnapshot(ctx, op, document.DocumentId, &document)
}
func FetchCurrentPublishedSnapshot(ctx context.Context, op OperationContext, documentID string) (PublishedSnapshot, error) {
	return readPublishedSnapshot(ctx, op, documentID, nil)
}

// ReadPublicTree 读取 content 已登记的公开树并校验完整生产方响应。
func ReadPublicTree(ctx context.Context, op OperationContext) (PublicTreeSnapshot, error) {
	store, _ := LanguageGateway()
	if store == nil || store.content == nil || store.sourceValidators["ContentReadPublicTreeResult"] == nil {
		return PublicTreeSnapshot{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化公开树客户端未装配")
	}
	raw, err := store.content.ReadPublicTree(SourceOperation(ctx, op))
	if err != nil {
		return PublicTreeSnapshot{}, err
	}
	if err = ValidateSourcePayload("ContentReadPublicTreeResult", raw); err != nil {
		return PublicTreeSnapshot{}, err
	}
	var response struct {
		Nodes []struct {
			DocumentID string                   `json:"documentId"`
			Reference  gen.PublishedRevisionRef `json:"reference"`
		} `json:"nodes"`
		TreeRevision int64 `json:"treeRevision"`
	}
	if json.Unmarshal(raw, &response) != nil || response.TreeRevision < 0 || len(response.Nodes) > 10000 {
		return PublicTreeSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "公开树响应不可读取")
	}
	result := PublicTreeSnapshot{TreeRevision: response.TreeRevision, Documents: make([]PublicTreeDocument, 0, len(response.Nodes))}
	seen := map[string]bool{}
	for _, node := range response.Nodes {
		if node.DocumentID == "" || node.Reference.DocumentId != node.DocumentID ||
			node.Reference.DataEpoch != op.DataEpoch || seen[node.DocumentID] {
			return PublicTreeSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "公开树文档引用重复、跨代次或身份不一致")
		}
		seen[node.DocumentID] = true
		result.Documents = append(result.Documents, PublicTreeDocument{DocumentID: node.DocumentID, Reference: node.Reference})
	}
	return result, nil
}
func readPublishedSnapshot(ctx context.Context, op OperationContext, documentID string, expected *gen.PublishedRevisionRef) (PublishedSnapshot, error) {
	store, _ := LanguageGateway()
	if store == nil || store.content == nil {
		return PublishedSnapshot{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化权威正文客户端未装配")
	}
	query := url.Values{}
	if expected != nil {
		query.Set("publishedRevisionId", expected.PublishedRevisionId)
	}
	raw, err := store.content.ReadPublishedRevision(SourceOperation(ctx, op), documentID, query)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	if err = ValidateSourcePayload("ContentReadPublishedRevisionResult", raw); err != nil {
		return PublishedSnapshot{}, err
	}
	var response struct {
		Metadata struct {
			SiteId         string `json:"siteId"`
			SourceLanguage string `json:"sourceLanguage"`
			Title          string `json:"title"`
		} `json:"metadata"`
		Reference          gen.PublishedRevisionRef `json:"reference"`
		Body               json.RawMessage          `json:"body"`
		VisibilityRevision int                      `json:"visibilityRevision"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return PublishedSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "权威正文响应不可读取")
	}
	if (expected != nil && response.Reference != *expected) || response.Reference.DocumentId != documentID || response.Reference.DataEpoch != op.DataEpoch || response.Metadata.SiteId != op.Actor.SiteID {
		return PublishedSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "翻译来源已变化或不属于本站")
	}
	canonical, err := platform.CanonicalJSON(response.Body)
	if err != nil || platform.HashBytes(canonical) != response.Reference.ContentHash {
		return PublishedSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "权威源正文摘要不匹配")
	}
	return PublishedSnapshot{Reference: response.Reference, Body: canonical, SourceLanguage: response.Metadata.SourceLanguage, Title: response.Metadata.Title, VisibilityRevision: response.VisibilityRevision}, nil
}
func CheckPublication(ctx context.Context, op OperationContext, document gen.PublishedRevisionRef, expectedVisibility *int) (int, error) {
	store, _ := LanguageGateway()
	if store == nil || store.content == nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化发布校验客户端未装配")
	}
	raw, err := store.content.ValidatePublications(SourceOperation(ctx, op), struct {
		Refs []gen.PublishedRevisionRef `json:"refs"`
	}{[]gen.PublishedRevisionRef{document}})
	if err != nil {
		return 0, err
	}
	if err = ValidateSourcePayload("ContentValidatePublicationsResult", raw); err != nil {
		return 0, err
	}
	var result struct {
		Validations []struct {
			Reference  gen.PublishedRevisionRef `json:"reference"`
			Visible    bool                     `json:"visible"`
			Matches    bool                     `json:"matchesCurrent"`
			Current    int                      `json:"currentRevision"`
			Visibility int                      `json:"visibilityRevision"`
		} `json:"validations"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Validations) != 1 || result.Validations[0].Reference != document {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "发布校验响应顺序或引用不匹配")
	}
	v := result.Validations[0]
	if !v.Visible || !v.Matches {
		return 0, platform.NewError(platform.CodeStaleResult, false, "翻译来源已撤回或被新发布替换")
	}
	if v.Current != document.Revision {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "发布校验的当前修订与引用矛盾")
	}
	if expectedVisibility != nil && v.Visibility != *expectedVisibility {
		return 0, platform.NewError(platform.CodeStaleResult, false, "翻译读取期间可见性水位改变")
	}
	return v.Visibility, nil
}

// CursorKey 返回已装配服务密钥引用，游标加解密机制只使用平台serviceauth。
func CursorKey() (platform.SigningKey, error) {
	store, _ := LanguageGateway()
	if store == nil || store.cursorKey.KeyID == "" {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化游标签名密钥未装配")
	}
	return store.cursorKey, nil
}
