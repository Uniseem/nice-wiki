// Package qa 实现问答域的纯规则与问答流程。
package qa

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"nice-wiki/libs/platform"
)

// OperationContext 是服务边界构建的可信操作上下文。
type OperationContext = platform.OperationContext

// NormalizeQuestionInput 是问题规范化输入。
type NormalizeQuestionInput struct {
	Question string `json:"question"`
	Language string `json:"language"`
}

// NormalizeQuestionResult 是问题规范化结果。
type NormalizeQuestionResult struct {
	Normalized   string         `json:"normalized"`
	QuestionHash platform.Hash   `json:"questionHash"`
	Anchors      []string       `json:"anchors"`
}

const (
	maxInputRunes      = 65536
	maxQuestionRunes   = 500
	maxQuestionBytes   = 8192
	maxQuestionAnchors = 256
)

// NormalizeQuestion 将问题转换成稳定的缓存基准文本，并提取无需模型即可确定的
// 数字、代码/产品标识、否定词和连续汉字锚点。该函数不读写存储，也不调用模型。
func NormalizeQuestion(ctx context.Context, op OperationContext, input NormalizeQuestionInput) (NormalizeQuestionResult, error) {
	startedAt := time.Now()
	if err := checkNormalizeContext(ctx, op, startedAt); err != nil {
		return NormalizeQuestionResult{}, err
	}
	if !validInputText(input.Question) || !validInputText(input.Language) {
		return NormalizeQuestionResult{}, platform.NewError(platform.CodeValidation, false, "问答输入不合法")
	}

	normalized := normalizeWhitespace(input.Question)
	normalized = foldNaturalCase(normalized)
	if err := checkNormalizeContext(ctx, op, startedAt); err != nil {
		return NormalizeQuestionResult{}, err
	}
	if normalized == "" || utf8.RuneCountInString(normalized) > maxQuestionRunes || len(normalized) > maxQuestionBytes {
		return NormalizeQuestionResult{}, platform.NewError(platform.CodeValidation, false, "问题长度不合法")
	}

	return NormalizeQuestionResult{
		Normalized:   normalized,
		QuestionHash: platform.HashBytes([]byte(input.Language + "\x00" + normalized)),
		Anchors:      extractAnchors(normalized),
	}, nil
}

func checkNormalizeContext(ctx context.Context, op OperationContext, startedAt time.Time) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "问答上下文缺失")
	}
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return platform.NewError(platform.CodeDeadlineExceeded, false, "问答操作已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, false, "问答操作已取消")
	default:
	}
	if op.Deadline != "" {
		deadline, err := platform.ParseTimestamp(op.Deadline)
		if err != nil {
			return platform.NewError(platform.CodeValidation, false, "问答截止时间不合法")
		}
		if !time.Now().UTC().Before(deadline) {
			return platform.NewError(platform.CodeDeadlineExceeded, false, "问答操作已超过期限")
		}
	}
	if time.Since(startedAt) >= 5*time.Second {
		return platform.NewError(platform.CodeDeadlineExceeded, false, "问答操作已超过期限")
	}
	return nil
}

func validInputText(value string) bool {
	if !utf8.ValidString(value) || value == "" || utf8.RuneCountInString(value) > maxInputRunes {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// normalizeWhitespace 先由NFKC处理全/半角等价字符，再把CRLF/CR和Unicode空白
// 统一成单个ASCII空格。缓存锚点不保留输入排版差异。
func normalizeWhitespace(value string) string {
	value = norm.NFKC.String(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")

	var b strings.Builder
	b.Grow(len(value))
	pendingSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if b.Len() != 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func foldNaturalCase(value string) string {
	parts := strings.Split(value, " ")
	for i, part := range parts {
		if !isProtectedToken(part) {
			parts[i] = strings.ToLower(part)
		}
	}
	return strings.Join(parts, " ")
}

func isProtectedToken(token string) bool {
	core := strings.Trim(token, "\"'“”‘’.,;:!?()[]{}<>")
	if core == "" {
		return false
	}
	lower := strings.ToLower(core)
	if _, ok := negationWords[lower]; ok {
		return true
	}
	for _, r := range core {
		if unicode.IsDigit(r) {
			return true
		}
	}
	if strings.ContainsAny(core, "_/\\:#@=+") {
		return true
	}
	return hasSignificantUppercase(core)
}

func hasSignificantUppercase(value string) bool {
	letters := 0
	upper := 0
	firstLetter := true
	for _, r := range value {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.IsUpper(r) {
			upper++
			if !firstLetter {
				return true
			}
		}
		firstLetter = false
	}
	return letters > 1 && upper == letters
}

var negationWords = map[string]struct{}{
	"不": {}, "没": {}, "没有": {}, "无": {}, "未": {}, "非": {}, "否": {},
	"不要": {}, "别": {}, "不是": {}, "不能": {}, "不可": {}, "不可以": {}, "无法": {},
	"并非": {}, "未曾": {}, "从不": {}, "not": {}, "no": {}, "none": {},
	"never": {}, "without": {}, "cannot": {}, "can't": {}, "don't": {},
	"doesn't": {}, "isn't": {}, "aren't": {}, "won't": {}, "wouldn't": {},
	"shouldn't": {}, "couldn't": {},
}

func extractAnchors(value string) []string {
	seen := make(map[string]struct{})
	anchors := make([]string, 0, 8)
	for _, token := range strings.Split(value, " ") {
		core := strings.Trim(token, "\"'“”‘’.,;:!?()[]{}<>")
		if core == "" || !isAnchorToken(core) {
			continue
		}
		if _, exists := seen[core]; exists {
			continue
		}
		seen[core] = struct{}{}
		anchors = append(anchors, core)
		if len(anchors) == maxQuestionAnchors {
			break
		}
	}
	return anchors
}

func isAnchorToken(token string) bool {
	if _, ok := negationWords[strings.ToLower(token)]; ok {
		return true
	}
	if strings.ContainsAny(token, "_/\\:#@=+") || hasSignificantUppercase(token) {
		return true
	}
	for _, r := range token {
		if unicode.IsDigit(r) || unicode.In(r, unicode.Han) {
			return true
		}
	}
	return false
}
