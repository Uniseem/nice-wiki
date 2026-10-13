package qa

import (
	"context"
	"encoding/json"
	"errors"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/prompts"
	"strings"
	"unicode/utf8"
)

type Route struct {
	Question     string   `json:"question"`
	Subquestions []string `json:"subquestions"`
	Risk         string   `json:"risk"`
	Complete     *bool    `json:"complete"`
	OutOfScope   bool     `json:"outOfScope"`
}

func (r Route) Cacheable() bool { return r.Complete == nil || *r.Complete }

func RouteQuestion(ctx context.Context, client *llm.Client, history Conversation, question, baseLanguage, memory string, maximum int) (Route, error) {
	prompt, err := prompts.Get("router")
	if err != nil {
		return Route{}, err
	}
	payload, err := json.Marshal(map[string]any{"summary": history.Summary, "turns": history.Turns, "question": question, "baseLanguage": baseLanguage})
	if err != nil {
		return Route{}, err
	}
	system := memory + "\n" + prompt + "\n返回 JSON：{\"question\":\"独立完整问题\",\"subquestions\":[\"子问题\"],\"risk\":\"low|medium|high\",\"complete\":true,\"outOfScope\":false}。最多四个子问题。"
	var route Route
	_, err = client.CompleteValidated(ctx, "qa-route", []protocol.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}}, func(response protocol.Response) error {
		decoded, decodeErr := llm.DecodeStructured[Route](response.Text)
		if decodeErr == nil {
			route = decoded
		}
		return decodeErr
	})
	if err != nil {
		return Route{}, err
	}
	route.Question, err = ValidateQuestion(route.Question, maximum)
	if err != nil {
		return Route{}, err
	}
	if len(route.Subquestions) > 4 {
		route.Subquestions = route.Subquestions[:4]
	}
	valid := route.Subquestions[:0]
	for _, subquestion := range route.Subquestions {
		value, validateErr := ValidateQuestion(subquestion, maximum)
		if validateErr == nil {
			valid = append(valid, value)
		}
	}
	route.Subquestions = valid
	switch guard.Risk(route.Risk) {
	case guard.RiskLow, guard.RiskMedium, guard.RiskHigh:
	default:
		route.Risk = string(guard.RiskLow)
	}
	return route, nil
}

func ValidateQuestion(value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || len([]rune(value)) > maximum {
		return "", errors.New("问题为空、编码无效或超过长度上限")
	}
	return value, nil
}
