package publicsite

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"time"
)

type IssuePreviewTokenInput struct {
	DocumentID      string `json:"documentId"`
	DraftRevisionID string `json:"draftRevisionId"`
	Revision        *int   `json:"revision,omitempty"`
}
type IssuePreviewTokenResult struct {
	PreviewID string    `json:"previewId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type ValidatePreviewTokenInput struct {
	PreviewID string
	Token     string
}
type PreviewMetadata struct {
	DocumentID      string `json:"documentId"`
	DraftRevisionID string `json:"draftRevisionId"`
	Title           string `json:"title"`
	SourceLanguage  string `json:"sourceLanguage"`
}
type ValidatePreviewTokenResult struct {
	Body     json.RawMessage `json:"body"`
	Metadata PreviewMetadata `json:"metadata"`
}

// IssuePreviewToken 直接返回content的grant，保留旧revision字段但不用于选取草稿或授权。
func IssuePreviewToken(ctx context.Context, op platform.OperationContext, in IssuePreviewTokenInput) (IssuePreviewTokenResult, error) {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" {
		return IssuePreviewTokenResult{}, platform.NewError(platform.CodeUnauthenticated, false, "需要当前后台会话")
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return IssuePreviewTokenResult{}, err
	}
	if directory.Identity == nil {
		return IssuePreviewTokenResult{}, dependency("即时授权客户端未装配")
	}
	authorize := apivalidate.NewIdentityAuthorizer(directory.Identity)
	if err = authorize(ctx, op, "content.preview", "public-site:issuePreviewToken"); err != nil {
		return IssuePreviewTokenResult{}, err
	}
	raw, err := directory.Content.CreatePreviewGrant(ctx, map[string]any{"documentId": in.DocumentID, "draftRevisionId": in.DraftRevisionID})
	if err != nil {
		return IssuePreviewTokenResult{}, err
	}
	var result struct {
		Grant *struct {
			PreviewID       string    `json:"previewId"`
			DocumentID      string    `json:"documentId"`
			DraftRevisionID string    `json:"draftRevisionId"`
			HolderAccountID string    `json:"holderAccountId"`
			Token           string    `json:"token"`
			ExpiresAt       time.Time `json:"expiresAt"`
		} `json:"grant"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Grant == nil || !platform.ValidIdentifier(result.Grant.PreviewID) || len(result.Grant.Token) < 32 || len(result.Grant.Token) > 4096 || result.Grant.ExpiresAt.IsZero() || result.Grant.DocumentID != in.DocumentID || result.Grant.DraftRevisionID != in.DraftRevisionID || result.Grant.HolderAccountID != string(op.Actor.SubjectID) {
		return IssuePreviewTokenResult{}, dependency("预览授权来源无法核验")
	}
	return IssuePreviewTokenResult{result.Grant.PreviewID, result.Grant.Token, result.Grant.ExpiresAt}, nil
}

// ValidatePreviewToken 每次均由content重验本人、会话、epoch、expiry及精确草稿，不消费刷新许可。
func ValidatePreviewToken(ctx context.Context, op platform.OperationContext, in ValidatePreviewTokenInput) (ValidatePreviewTokenResult, error) {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" {
		return ValidatePreviewTokenResult{}, platform.NewError(platform.CodeUnauthenticated, false, "需要当前后台会话")
	}
	if !platform.ValidIdentifier(in.PreviewID) || len(in.Token) < 32 || len(in.Token) > 4096 {
		return ValidatePreviewTokenResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_PREVIEW_TOKEN")
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return ValidatePreviewTokenResult{}, err
	}
	raw, err := directory.Content.ValidatePreviewGrant(ctx, map[string]any{"previewId": in.PreviewID, "token": in.Token, "accountId": op.Actor.SubjectID})
	if err != nil {
		return ValidatePreviewTokenResult{}, err
	}
	var result ValidatePreviewTokenResult
	if json.Unmarshal(raw, &result) != nil || len(result.Body) == 0 || result.Metadata.DocumentID == "" || result.Metadata.DraftRevisionID == "" {
		return ValidatePreviewTokenResult{}, dependency("预览正文权威不完整")
	}
	return result, nil
}
