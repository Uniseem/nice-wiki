package siteconfig

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type ValidateAnnouncementBodyInput struct {
	Body gen.AnnouncementBody `json:"body"`
}

type ValidateAnnouncementBodyResult struct {
	Valid bool     `json:"valid"`
	Hash  gen.Hash `json:"hash"`
}

// ValidateAnnouncementBody 只接受 schemaVersion=1 的纯文本和已有链接种类。
func ValidateAnnouncementBody(input ValidateAnnouncementBodyInput) (ValidateAnnouncementBodyResult, error) {
	raw, err := canonicalAnnouncementBody(input.Body)
	if err != nil {
		return ValidateAnnouncementBodyResult{}, err
	}
	return ValidateAnnouncementBodyResult{Valid: true, Hash: platform.HashBytes(raw)}, nil
}

func canonicalAnnouncementBody(body gen.AnnouncementBody) ([]byte, error) {
	version, ok := announcementSchemaVersion(body.SchemaVersion)
	if !ok || version != announcementJournalSchema {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "公告正文schema版本不支持")
	}
	if len([]rune(body.Text)) < 1 || len([]rune(body.Text)) > 2000 || strings.ContainsAny(body.Text, "<>") || strings.IndexFunc(body.Text, unicode.IsControl) >= 0 {
		return nil, platform.NewError(platform.CodeValidation, false, "公告正文须为不含标记的纯文本")
	}
	target := json.RawMessage("null")
	if body.Target != nil {
		raw, err := body.Target.MarshalJSON()
		if err != nil {
			return nil, platform.NewError(platform.CodeValidation, false, "公告链接无法序列化")
		}
		target = raw
	}
	canonical, err := platform.CanonicalJSON(struct {
		SchemaVersion int             `json:"schemaVersion"`
		Target        json.RawMessage `json:"target"`
		Text          string          `json:"text"`
	}{version, target, body.Text})
	if err != nil {
		return nil, err
	}
	if err := validateAnnouncementBody(canonical); err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && strings.Contains(failure.Message, "schema") {
			return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "公告正文schema版本不支持")
		}
		return nil, platform.NewError(platform.CodeValidation, false, "公告正文或链接不合法")
	}
	return canonical, nil
}

func announcementSchemaVersion(value any) (int, bool) {
	switch version := value.(type) {
	case float64:
		return int(version), version == float64(announcementJournalSchema)
	case int:
		return version, version == announcementJournalSchema
	case int64:
		return int(version), version == int64(announcementJournalSchema)
	case json.Number:
		parsed, err := version.Int64()
		return int(parsed), err == nil && parsed == int64(announcementJournalSchema)
	default:
		return 0, false
	}
}
