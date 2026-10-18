package extract

import (
	"bytes"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	pdf "github.com/ledongthuc/pdf"
	"nice-wiki/libs/platform"
)

const PDFExtractorVersion = "pdf-text-v1+ledongthuc-6c8c28e0e8a0"
const maximumPDFTextBytes = 8 << 20

type PDFPage struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}
type PDFText struct {
	Pages       []PDFPage `json:"pages"`
	PageCount   int       `json:"pageCount"`
	Unsupported bool      `json:"unsupported"`
}

// ParsePDFPages 只在公共无网络沙箱的自身--pdf-worker入口内调用；不在服务goroutine直接解析。
// API来源github.com/ledongthuc/pdf固定v0.0.0-20260907135840-6c8c28e0e8a0，保留依赖原许可。
// 此处独立编写阅读序与预算规则，没有复制其源码或示例。
func ParsePDFPages(raw []byte) (PDFText, error) {
	if len(raw) == 0 || len(raw) > 50<<20 {
		return PDFText{}, platform.NewError(platform.CodeValidation, false, "PDF超出50MiB提取预算")
	}
	reader, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return PDFText{}, platform.NewError(platform.CodeUnsupportedSchema, false, "PDF格式、加密或编码不支持")
	}
	count := reader.NumPage()
	if count < 0 || count > 500 {
		return PDFText{}, platform.NewError(platform.CodeValidation, false, "PDF超过500页提取预算")
	}
	result := PDFText{Pages: make([]PDFPage, 0, count), PageCount: count, Unsupported: true}
	used := 0
	for number := 1; number <= count; number++ {
		page := reader.Page(number)
		if page.V.IsNull() {
			return PDFText{}, platform.NewError(platform.CodeUnsupportedSchema, false, "PDF页结构不完整")
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			return PDFText{}, platform.NewError(platform.CodeUnsupportedSchema, false, "PDF文字编码或页面指令不支持")
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Position > rows[j].Position })
		var text strings.Builder
		for _, row := range rows {
			sort.SliceStable(row.Content, func(i, j int) bool { return row.Content[i].X < row.Content[j].X })
			hasPrevious := false
			previousEnd, previousSize := 0.0, 0.0
			for _, part := range row.Content {
				if !utf8.ValidString(part.S) || strings.ContainsRune(part.S, 0) || math.IsNaN(part.X) || math.IsInf(part.X, 0) || math.IsNaN(part.W) || math.IsInf(part.W, 0) || math.IsNaN(part.FontSize) || math.IsInf(part.FontSize, 0) {
					return PDFText{}, platform.NewError(platform.CodeUnsupportedSchema, false, "PDF文字或布局坐标不合法")
				}
				if part.S == "" {
					continue
				}
				// 同行按左到右；明显字距只补一个分词空格，保留实际Unicode正文与页号。
				if hasPrevious && part.X-previousEnd > math.Max(previousSize, part.FontSize)*0.25 && !strings.HasSuffix(text.String(), " ") && !strings.HasPrefix(part.S, " ") {
					text.WriteByte(' ')
				}
				if used+text.Len()+len(part.S)+1 > maximumPDFTextBytes {
					return PDFText{}, platform.NewError(platform.CodeValidation, false, "PDF文字超过8MiB提取预算")
				}
				text.WriteString(part.S)
				previousEnd = part.X + part.W
				previousSize = part.FontSize
				hasPrevious = true
			}
			if hasPrevious {
				text.WriteByte('\n')
			}
		}
		value := text.String()
		used += len(value)
		if strings.TrimSpace(value) != "" {
			result.Unsupported = false
		}
		result.Pages = append(result.Pages, PDFPage{Page: number, Text: value})
	}
	return result, nil
}
