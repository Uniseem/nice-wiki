package content

import (
	"encoding/json"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"strings"
)

type Meta struct {
	ID             string `json:"id"`
	ParentID       string `json:"parentId"`
	Title          string `json:"title"`
	Slug           string `json:"slug"`
	SortKey        string `json:"sortKey"`
	Published      bool   `json:"published"`
	Revision       int64  `json:"revision"`
	UpdatedAt      int64  `json:"updatedAt"`
	SEODescription string `json:"seoDescription"`
}
type File struct {
	Draft    *Draft       `json:"draft,omitempty"`
	Meta     Meta         `json:"meta"`
	Document doc.Document `json:"document"`
}

func (v File) Validate() error {
	if !datadir.ValidID(v.Meta.ID) || v.Meta.ParentID != "" && !datadir.ValidID(v.Meta.ParentID) {
		return invalid("文档身份无效")
	}
	if strings.TrimSpace(v.Meta.Title) == "" || len([]rune(v.Meta.Title)) > 200 || len([]rune(v.Meta.SEODescription)) > 500 {
		return invalid("文档标题或描述长度无效")
	}
	if err := ValidateSlug(v.Meta.Slug); err != nil {
		return err
	}
	if v.Meta.SortKey == "" || v.Meta.Revision < 1 {
		return invalid("文档排序或修订号无效")
	}
	return doc.Validate(v.Document)
}

func jsonMarshal(value any) ([]byte, error)      { return json.Marshal(value) }
func jsonUnmarshal(data []byte, value any) error { return json.Unmarshal(data, value) }

// ValidationError 的消息只包含可向调用者展示的静态规则，不包含凭据或路径。
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func invalid(message string) error       { return &ValidationError{Message: message} }
