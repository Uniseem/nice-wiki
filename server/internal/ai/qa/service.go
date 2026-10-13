package qa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"nice-wiki/internal/ai/cache"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/textutil"
	"strings"
	"time"
)

type Retriever interface {
	Retrieve(context.Context, string) ([]retrieve.Unit, error)
}
type Service struct {
	Models           *llm.Client
	Retriever        Retriever
	Cache            *cache.Store
	Limiter          *guard.Limiter
	MaxQuestion      int
	Conversations    *Conversations
	Records          *Records
	Persist          func(context.Context, Record) error
	ResolveCitation  func(string) (string, string, bool)
	Budget           *guard.Budget
	Compactor        CompactModel
	MaxConversation  int
	Ready            func() bool
	Settings         func() settings.AI
	BaseLanguage     func() string
	CurrentReference func(string, string) bool
	TranslatedText   func(string, string, string) (string, bool)
}

func (s *Service) Ask(ctx context.Context, session, question, language, memoryVersion, memory string, emit func(Event) error) error {
	maximum := s.MaxQuestion
	options := settings.AI{ExactCache: true, SemanticCache: true, MaxOutputTokens: 4096, MaxTurns: 100}
	if s.Settings != nil {
		options = s.Settings()
		maximum = options.MaxQuestionLength
		if !options.Enabled {
			return errors.New("AI 问答尚未启用")
		}
	}
	if s.Ready != nil && !s.Ready() {
		return errors.New("问答模型尚未配置")
	}
	question, err := ValidateQuestion(question, maximum)
	if err != nil {
		return err
	}
	if s.Limiter == nil || s.Retriever == nil || s.Models == nil {
		return errors.New("问答服务尚未完整配置")
	}
	if err := s.Limiter.Begin(session, time.Now()); err != nil {
		return err
	}
	defer s.Limiter.End(session)
	if s.Budget != nil {
		ctx = llm.WithSpending(ctx, func(amount float64) (func(float64), error) {
			if s.Settings != nil && !s.Settings().Enabled {
				return nil, errors.New("AI 问答尚未启用")
			}
			return s.Budget.Reserve(session, amount, time.Now())
		})
	}
	ctx = llm.WithOutputLimit(ctx, options.MaxOutputTokens)
	history := Conversation{}
	if s.Conversations != nil {
		if s.Compactor != nil {
			if err := Compact(ctx, s.Conversations, s.Compactor, session, s.MaxConversation); err != nil {
				return err
			}
		}
		history, _ = s.Conversations.Get(session)
		if len(history.Turns) >= options.MaxTurns*2 {
			return errors.New("对话轮数已达到限制，请清空对话")
		}
	}
	contextual := history.Summary != "" || len(history.Turns) > 0
	query := question
	base := language
	if s.BaseLanguage != nil {
		base = s.BaseLanguage()
	}
	if contextual || language != base {
		payload, _ := json.Marshal(map[string]any{"summary": history.Summary, "turns": history.Turns, "question": question})
		routed, err := s.Models.Complete(ctx, "qa-route", []protocol.Message{{Role: "system", Content: "根据对话把当前问题改写成独立完整的问题，使用语言 " + base + "。返回 JSON：{\"question\":\"完整问题\"}。"}, {Role: "user", Content: string(payload)}})
		if err != nil {
			return err
		}
		var route struct {
			Question string `json:"question"`
		}
		if err := json.Unmarshal([]byte(routed.Text), &route); err != nil {
			return errors.New("问题改写格式无效")
		}
		query, err = ValidateQuestion(route.Question, maximum)
		if err != nil {
			return err
		}
	}
	if s.Conversations != nil {
		if err := s.Conversations.Append(session, Turn{Role: "user", Text: question, Created: time.Now()}); err != nil {
			return err
		}
	}
	if !contextual && s.Cache != nil {
		var hit cache.Entry
		var ok bool
		if options.ExactCache {
			hit, ok = s.Cache.Get(query, language, memoryVersion, nil)
		}
		if !ok && options.SemanticCache {
			hit, ok = s.Cache.FindSemantic(query, language, memoryVersion, .90)
		}
		if ok && s.validCache(hit) {
			return s.finish(ctx, session, question, language, hit.Answer, s.cacheCitations(hit, language), emit)
		}
	}
	if err := emit(Event{Type: "progress", Message: "正在查找相关内容"}); err != nil {
		return err
	}
	units, err := s.Retriever.Retrieve(ctx, query)
	if err != nil {
		return err
	}
	materialUnits := append([]retrieve.Unit{}, units...)
	if s.TranslatedText != nil {
		for i, unit := range materialUnits {
			if text, ok := s.TranslatedText(unit.DocumentID, unit.ID, language); ok {
				materialUnits[i].Text = text
			}
		}
	}
	contextText, _ := json.Marshal(history)
	response, err := Answer(ctx, s.Models, question, Assemble(materialUnits, 24000), memory+"\n回答语言："+language+"\n已有对话（仅供理解追问，事实须由材料支持）："+string(contextText))
	if err != nil {
		return err
	}
	if s.CurrentReference != nil {
		for _, unit := range units {
			if !s.CurrentReference(unit.ID, textutil.Hash([]byte(unit.Text))) {
				return errors.New("参考内容已发生变化，请重新提问")
			}
		}
	}
	references := map[string]string{}
	sources := map[string]string{}
	titles := map[string]string{}
	citations := []Citation{}
	for _, unit := range units {
		references[unit.ID] = textutil.Hash([]byte(unit.Text))
		sources[unit.ID] = unit.DocumentID
		titles[unit.ID] = unit.Heading
		title, url := unit.Heading, ""
		if s.ResolveCitation != nil {
			if docTitle, path, ok := s.ResolveCitation(unit.DocumentID); ok {
				if title == "" {
					title = docTitle
				}
				url = "/" + language + "/" + path
			}
		}
		citations = append(citations, Citation{DocumentID: unit.DocumentID, BlockID: unit.ID, Title: title, URL: url, SourceHash: references[unit.ID]})
	}
	if !contextual && s.Cache != nil {
		s.Cache.Put(cache.Entry{Question: query, Answer: response.Text, Language: language, MemoryVersion: memoryVersion, Created: time.Now(), Expires: time.Now().Add(24 * time.Hour), References: references, Sources: sources, Titles: titles})
	}
	return s.finish(ctx, session, question, language, response.Text, citations, emit)
}
func (s *Service) validCache(hit cache.Entry) bool {
	for id, hash := range hit.References {
		if s.CurrentReference == nil || !s.CurrentReference(id, hash) {
			return false
		}
	}
	for _, id := range hit.Sources {
		if s.ResolveCitation != nil {
			if _, _, ok := s.ResolveCitation(id); !ok {
				return false
			}
		}
	}
	return true
}
func (s *Service) cacheCitations(hit cache.Entry, language string) []Citation {
	result := []Citation{}
	for block, id := range hit.Sources {
		if s.ResolveCitation != nil {
			title, path, ok := s.ResolveCitation(id)
			if ok {
				if strings.TrimSpace(hit.Titles[block]) != "" {
					title = hit.Titles[block]
				}
				result = append(result, Citation{DocumentID: id, BlockID: block, Title: title, URL: "/" + language + "/" + path, SourceHash: hit.References[block]})
			}
		}
	}
	return result
}
func (s *Service) finish(ctx context.Context, session, question, language, answer string, citations []Citation, emit func(Event) error) error {
	if err := emit(Event{Type: "text", Text: answer}); err != nil {
		return err
	}
	if len(citations) > 0 {
		if err := emit(Event{Type: "citations", Citations: citations}); err != nil {
			return err
		}
	}
	created := time.Now()
	sum := sha256.Sum256([]byte(session + question + created.String()))
	record := Record{ID: hex.EncodeToString(sum[:]), ConversationID: session, Question: question, Answer: answer, Language: language, Citations: citations, Created: created}
	if s.Conversations != nil {
		if err := s.Conversations.Append(session, Turn{Role: "assistant", Text: answer, Created: created, RecordID: record.ID}); err != nil {
			return err
		}
	}
	if s.Records != nil {
		s.Records.Add(record)
	}
	if s.Persist != nil {
		if err := s.Persist(ctx, record); err != nil {
			return err
		}
	}
	return emit(Event{Type: "done", ConversationID: session, RecordID: record.ID})
}
