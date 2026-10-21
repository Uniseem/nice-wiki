package suggestion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	localization "nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type SuggestLanguageInput = gen.SuggestLanguageInput
type SuggestLanguageResult = gen.SuggestLanguageResult

// minimumConfidence 是模型建议切换的最低置信度；不到就当作不建议。
const minimumConfidence = 0.6

// suggestNotes 写给决策模型的判断规则（状态里的数据说明，不是指令）。
const suggestNotes = "浏览器语言最重要，时区其次，IP 国家只是弱信号。浏览器首选语言是简体中文（zh-CN、zh-Hans、zh-SG 或 zh）时，IP 国家多半因为代理不可信，基本忽略它，只在其他信号打平时参考。"

var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

// signals 是归一化后的访客信号；同样的信号组合共用一份模型判断。
type signals struct {
	Browser  []string `json:"browserLanguages"`
	TimeZone string   `json:"timeZone,omitempty"`
	Country  string   `json:"ipCountry,omitempty"`
	Current  string   `json:"pageLanguage"`
}

// SuggestLanguage 给访客一个语言建议：先按规则（浏览器语言 → 非简中浏览器时参考 IP 国家）；
// 管理员开启后，再把信号交给决策模型（如 Jev）做一道单选题，结果按信号组合缓存，模型失败或超时就用规则结果。
func SuggestLanguage(ctx context.Context, op OperationContext, input SuggestLanguageInput) (SuggestLanguageResult, error) {
	if input.PreviouslyRejected || (input.ExplicitPreference != nil && *input.ExplicitPreference != "") {
		return SuggestLanguageResult{Reason: "none"}, nil
	}
	var enabled []string
	var policy localization.LanguageSuggestionPolicy
	err := localization.ReadData(ctx, op, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT language_tag FROM languages WHERE data_epoch=? AND enabled=1 ORDER BY default_language DESC, language_tag`, op.DataEpoch)
		if err != nil {
			return localization.StorageError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var tag string
			if err := rows.Scan(&tag); err != nil {
				return localization.StorageError(err)
			}
			enabled = append(enabled, tag)
		}
		if err := rows.Err(); err != nil {
			return localization.StorageError(err)
		}
		policy, err = localization.CurrentSuggestionPolicy(ctx, tx, op)
		return err
	})
	if err != nil {
		return SuggestLanguageResult{}, err
	}
	current := ""
	for _, tag := range enabled {
		if strings.EqualFold(tag, input.CurrentLanguage) {
			current = tag
		}
	}
	if current == "" {
		return SuggestLanguageResult{}, platform.NewError(platform.CodeValidation, false, "当前语言未启用")
	}
	if len(enabled) < 2 {
		return SuggestLanguageResult{Reason: "none"}, nil
	}
	s := normalize(input, current)
	rule := ruleSuggestion(s, enabled)
	if !policy.Enabled || policy.ApprovalID == "" {
		return rule, nil
	}
	key := platform.HashBytes([]byte(fmt.Sprintf("%v|%s", s, strings.Join(enabled, ","))))
	var cached SuggestLanguageResult
	err = localization.ReadData(ctx, op, func(tx *sql.Tx) error {
		var tag string
		var confidence float64
		err := tx.QueryRowContext(ctx, `SELECT language,confidence FROM language_suggestions WHERE cache_key=? AND data_epoch=?`, key, op.DataEpoch).Scan(&tag, &confidence)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return localization.StorageError(err)
		}
		cached = modelResult(tag, confidence, current)
		return nil
	})
	if err != nil {
		return SuggestLanguageResult{}, err
	}
	if cached.Reason != "" {
		return cached, nil
	}
	tag, confidence, err := decide(ctx, op, s, enabled, policy.ApprovalID)
	if err != nil {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "语言建议模型判断失败，改用规则 err=%v\n", err)
		}
		return rule, nil
	}
	err = localization.WriteData(ctx, op, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO language_suggestions(cache_key,data_epoch,language,confidence,created_at) VALUES(?,?,?,?,?) ON CONFLICT(cache_key) DO NOTHING`, key, op.DataEpoch, tag, confidence, platform.Now())
		return localization.StorageError(err)
	})
	if err != nil {
		return SuggestLanguageResult{}, err
	}
	return modelResult(tag, confidence, current), nil
}

func normalize(input SuggestLanguageInput, current string) signals {
	s := signals{Current: current, Browser: []string{}}
	for _, raw := range input.BrowserLanguages {
		if tag, err := language.Parse(raw); err == nil && len(s.Browser) < 5 {
			s.Browser = append(s.Browser, tag.String())
		}
	}
	if _, err := time.LoadLocation(input.TimeZone); err == nil && strings.Contains(input.TimeZone, "/") {
		s.TimeZone = input.TimeZone
	}
	if input.Country != nil && countryCode.MatchString(*input.Country) {
		s.Country = *input.Country
	}
	return s
}

// simplifiedChineseBrowser 判断浏览器首选语言是不是简体中文：这类访客常用代理，IP 国家不可信。
func simplifiedChineseBrowser(s signals) bool {
	if len(s.Browser) == 0 {
		return false
	}
	tag := language.Make(s.Browser[0])
	base, _ := tag.Base()
	script, _ := tag.Script()
	return base.String() == "zh" && script.String() == "Hans"
}

// ruleSuggestion 是不调模型的建议：先按浏览器语言做标准匹配；浏览器语言都对不上、且不是简中浏览器时，按 IP 国家的常用语言再匹配一次。
func ruleSuggestion(s signals, enabled []string) SuggestLanguageResult {
	tags := make([]language.Tag, len(enabled))
	for i, tag := range enabled {
		tags[i] = language.Make(tag)
	}
	matcher := language.NewMatcher(tags)
	wanted := make([]language.Tag, 0, len(s.Browser))
	for _, tag := range s.Browser {
		wanted = append(wanted, language.Make(tag))
	}
	if len(wanted) > 0 {
		if _, index, confidence := matcher.Match(wanted...); confidence != language.No {
			return ruleResult(enabled[index])
		}
	}
	if s.Country != "" && !simplifiedChineseBrowser(s) {
		if _, index, confidence := matcher.Match(language.Make("und-" + s.Country)); confidence != language.No {
			return ruleResult(enabled[index])
		}
	}
	return SuggestLanguageResult{Reason: "none"}
}

func ruleResult(tag string) SuggestLanguageResult {
	return SuggestLanguageResult{Reason: "rule", SuggestedLanguage: &tag, Confidence: 0.5}
}

// modelResult 把模型选择转成结果；置信度不够时当作建议留在当前语言。
func modelResult(tag string, confidence float64, current string) SuggestLanguageResult {
	if confidence < minimumConfidence {
		tag = current
	}
	return SuggestLanguageResult{Reason: "model", SuggestedLanguage: &tag, Confidence: float32(confidence)}
}

// decide 让决策模型做一道单选题：这位访客最可能想用哪种语言读。只能从已启用语言里选。
func decide(ctx context.Context, op OperationContext, s signals, enabled []string, approvalID string) (string, float64, error) {
	work := op
	work.CostClass = platform.CostClassBackground
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	route, err := localizationruntime.ResolvePlanTaskRoute(ctx, work, "localization.suggest")
	if err != nil {
		return "", 0, err
	}
	criteria := map[string]string{}
	for _, tag := range enabled {
		parsed := language.Make(tag)
		criteria[tag] = display.English.Tags().Name(parsed) + " / " + display.Self.Name(parsed)
	}
	matched := map[string]string{}
	tags := make([]language.Tag, len(enabled))
	for i, tag := range enabled {
		tags[i] = language.Make(tag)
	}
	matcher := language.NewMatcher(tags)
	for _, browser := range s.Browser {
		if _, index, confidence := matcher.Match(language.Make(browser)); confidence != language.No {
			matched[browser] = enabled[index]
		}
	}
	state := map[string]any{"visitor": s, "browserLanguagesOnThisSite": matched, "notes": suggestNotes}
	key := "suggest." + platform.NewIdentifier()
	outcome, err := localizationruntime.InvokeModel(ctx, work, platform.InvokeModelRequest{IDempotencyKey: key, Request: platform.CreateInvocationInput{
		TaskID: "localization.suggest", ProfileID: route.ProfileID, ExpectedRouteVersion: route.RouteVersion, Capability: platform.CapabilityDecision,
		Messages: []platform.GatewayMessage{}, Texts: []string{}, CandidateIDs: []platform.Identifier{}, IDempotencyKey: key, LanguageSuggestionApprovalID: approvalID,
		Decision: &platform.DecisionRequest{State: state, Questions: map[string]platform.DecisionQuestion{"language": {
			Type: "choice", Instructions: "这位文档站访客最可能希望用哪种语言阅读？只从选项里选。", Criteria: criteria,
		}}},
	}})
	if err != nil {
		return "", 0, err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil {
		return "", 0, platform.NewError(platform.CodeDependencyUnavailable, false, "语言建议模型结果未完成")
	}
	answer, ok := outcome.CurrentSnapshot.Output.Decision["language"]
	if !ok || answer.Type != "choice" || criteria[answer.Choice] == "" {
		return "", 0, platform.NewError(platform.CodeUnsupportedSchema, false, "语言建议模型给出了选项以外的结果")
	}
	confidence := answer.Probabilities[answer.Choice]
	if answer.Confidence != nil {
		confidence = *answer.Confidence
	}
	return answer.Choice, min(max(confidence, 0), 1), nil
}

func NewSuggestionHandlers() map[string]http.Handler {
	return map[string]http.Handler{"suggestLanguage": localization.JSONHandler(SuggestLanguage, func(i *SuggestLanguageInput, r *http.Request) error {
		// 国家只认入口用离线库查出的值，浏览器正文里的忽略。
		i.Country = nil
		if country := r.Header.Get(platform.HeaderVisitorCountry); country != "" {
			i.Country = &country
		}
		return nil
	})}
}
