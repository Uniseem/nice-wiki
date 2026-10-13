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
	"nice-wiki/internal/prompts"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/textutil"
	"sort"
	"strings"
	"time"
)

type Retriever interface {
	Retrieve(context.Context, string) ([]retrieve.Unit, error)
}
type QueryEmbedder interface {
	EmbedQuery(context.Context, string) ([]float32, string, error)
}
type VectorRetriever interface {
	RetrieveVector(context.Context, string, []float32, string) ([]retrieve.Unit, error)
}
type Service struct {
	Models           *llm.Client
	Retriever        Retriever
	Cache            *cache.Store
	CacheTranslator  cache.Translator
	Limiter          *guard.Limiter
	MaxQuestion      int
	Conversations    *Conversations
	Records          *Records
	Persist          func(context.Context, Record) error
	LoadConversation func(context.Context, string) (Conversation, bool, error)
	SaveSummary      func(context.Context, string, string) error
	ResolveCitation  func(string) (string, string, bool)
	Budget           *guard.Budget
	IPBudget         *guard.Budget
	Compactor        CompactModel
	MaxConversation  int
	Ready            func() bool
	Settings         func() settings.AI
	BaseLanguage     func() string
	CurrentReference func(string, string) bool
	TranslatedText   func(string, string, string) (string, bool)
	RouteAll         bool
	Bans             *guard.Bans
	Events           *guard.Events
	StreamAnswers    bool
	PersistCache     func(context.Context, cache.Entry)
	Trace            func(DebugStep)
}

func (s *Service) Ask(ctx context.Context, session, question, language, memoryVersion, memory string, emit func(Event) error) error {
	return s.AskScoped(ctx, session, session, question, language, memoryVersion, memory, emit)
}

func (s *Service) AskScoped(ctx context.Context, session, ipScope, question, language, memoryVersion, memory string, emit func(Event) error) error {
	return s.AskPage(ctx, session, ipScope, "", question, language, memoryVersion, memory, emit)
}

func (s *Service) AskPage(ctx context.Context, session, ipScope, page, question, language, memoryVersion, memory string, emit func(Event) error) error {
	maximum := s.MaxQuestion
	options := settings.AI{ExactCache: true, SemanticCache: true, SemanticCacheMode: "ai", SemanticThreshold: .90, MaxOutputTokens: 4096, MaxTurns: 100}
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
	if s.Budget != nil || s.IPBudget != nil {
		ctx = llm.WithSpending(ctx, func(amount float64) (func(float64), error) {
			if s.Settings != nil && !s.Settings().Enabled {
				return nil, errors.New("AI 问答尚未启用")
			}
			settlements := []func(float64){}
			for _, value := range []struct {
				budget *guard.Budget
				scope  string
			}{{s.Budget, session}, {s.IPBudget, ipScope}} {
				if value.budget == nil {
					continue
				}
				settle, err := value.budget.Reserve(value.scope, amount, time.Now())
				if err != nil {
					for _, rollback := range settlements {
						rollback(0)
					}
					return nil, err
				}
				settlements = append(settlements, settle)
			}
			return func(actual float64) {
				for _, settle := range settlements {
					settle(actual)
				}
			}, nil
		})
	}
	ctx = llm.WithOutputLimit(ctx, options.MaxOutputTokens)
	history := Conversation{}
	if s.Conversations != nil {
		var found bool
		history, found = s.Conversations.Get(session)
		if !found && s.LoadConversation != nil {
			loaded, exists, err := s.LoadConversation(ctx, session)
			if err != nil {
				return err
			}
			if exists {
				if err := s.Conversations.Put(loaded); err != nil {
					return err
				}
				history = loaded
			}
		}
		previousSummary := history.Summary
		if s.Compactor != nil {
			if err := Compact(ctx, s.Conversations, s.Compactor, session, s.MaxConversation); err != nil {
				return err
			}
		}
		history, _ = s.Conversations.Get(session)
		if s.SaveSummary != nil && history.Summary != previousSummary {
			if err := s.SaveSummary(ctx, session, history.Summary); err != nil {
				return err
			}
		}
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
	if s.RouteAll && !contextual && language == base && s.Cache != nil && options.ExactCache {
		if hit, ok, cacheErr := s.lookupCache(ctx, query, language, memoryVersion, true, false); cacheErr != nil {
			return cacheErr
		} else if ok {
			return s.finish(ctx, session, question, query, language, hit.Answer, s.cacheCitations(hit, language), true, emit)
		}
	}
	route := Route{Question: query}
	var queryVector []float32
	embeddingModel := ""
	if s.RouteAll || contextual || language != base {
		started := time.Now()
		route, err = RouteQuestion(ctx, s.Models, history, question, base, memory, maximum)
		if err != nil {
			return err
		}
		if s.Trace != nil {
			encoded, _ := json.Marshal(route)
			s.Trace(DebugStep{Name: "route", Input: question, Output: string(encoded), Duration: time.Since(started)})
		}
		query = route.Question
		modelRisk := guard.Risk(route.Risk)
		if modelRisk != guard.RiskLow && s.Events != nil {
			s.Events.Add(guard.Event{Scope: ipScope, Reason: "model", Risk: modelRisk, Created: time.Now()})
		}
		switch modelRisk {
		case guard.RiskMedium:
			return guard.ErrVerificationRequired
		case guard.RiskHigh:
			if s.Bans != nil {
				s.Bans.Ban(ipScope, time.Now())
			}
			return guard.ErrBanned
		}
		if route.OutOfScope {
			return guard.ErrOutOfScope
		}
	}
	if s.Cache != nil && route.Cacheable() {
		hit, ok, cacheErr := s.lookupCache(ctx, query, language, memoryVersion, options.ExactCache, false)
		if cacheErr != nil {
			return cacheErr
		}
		if !ok && options.SemanticCache {
			if embedder, supported := s.Retriever.(QueryEmbedder); supported {
				queryVector, embeddingModel, cacheErr = embedder.EmbedQuery(ctx, query)
				if cacheErr == nil {
					hit, ok, cacheErr = s.lookupVectorCache(ctx, query, language, memoryVersion, queryVector, embeddingModel, options)
				}
			} else {
				hit, ok, cacheErr = s.lookupCache(ctx, query, language, memoryVersion, false, true)
			}
		}
		if cacheErr != nil {
			return cacheErr
		}
		if ok {
			return s.finish(ctx, session, question, query, language, hit.Answer, s.cacheCitations(hit, language), true, emit)
		}
	}
	if err := emit(Event{Type: "progress", Message: "正在查找相关内容"}); err != nil {
		return err
	}
	queries := route.Subquestions
	if len(queries) == 0 {
		queries = []string{query}
	}
	if len(queryVector) == 0 {
		if embedder, supported := s.Retriever.(QueryEmbedder); supported {
			queryVector, embeddingModel, err = embedder.EmbedQuery(ctx, query)
			if err != nil {
				return err
			}
		}
	}
	retrievalStarted := time.Now()
	units, err := s.retrieveAll(ctx, queries, query, queryVector, embeddingModel)
	if err != nil {
		return err
	}
	if s.Trace != nil {
		encoded, _ := json.Marshal(units)
		s.Trace(DebugStep{Name: "retrieve", Input: strings.Join(queries, "\n"), Output: string(encoded), Duration: time.Since(retrievalStarted)})
	}
	if page != "" {
		sort.SliceStable(units, func(i, j int) bool { return units[i].DocumentID == page && units[j].DocumentID != page })
	}
	materialUnits := append([]retrieve.Unit{}, units...)
	if s.TranslatedText != nil {
		for i, unit := range materialUnits {
			if text, ok := s.TranslatedText(unit.DocumentID, unit.ID, language); ok {
				materialUnits[i].Text = text
			}
		}
	}
	materialUnits = LimitUnits(materialUnits, 24000)
	included := map[string]bool{}
	for _, unit := range materialUnits {
		included[unit.ID] = true
	}
	limited := units[:0]
	for _, unit := range units {
		if included[unit.ID] {
			limited = append(limited, unit)
		}
	}
	units = limited
	contextText, _ := json.Marshal(history)
	answerMemory := memory + "\n回答语言：" + language + "\n已有对话（仅供理解追问，事实须由材料支持）：" + string(contextText)
	var response protocol.Response
	answerStarted := time.Now()
	if s.StreamAnswers {
		response, err = AnswerStream(ctx, s.Models, question, Assemble(materialUnits, 24000), answerMemory, func(text string) error { return emit(Event{Type: "text", Text: text}) })
	} else {
		response, err = Answer(ctx, s.Models, question, Assemble(materialUnits, 24000), answerMemory)
	}
	if err != nil {
		return err
	}
	if s.Trace != nil {
		s.Trace(DebugStep{Name: "answer", Input: Assemble(materialUnits, 24000), Output: response.Text, Duration: time.Since(answerStarted)})
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
	if err := s.finish(ctx, session, question, query, language, response.Text, citations, !s.StreamAnswers, emit); err != nil {
		return err
	}
	if s.Cache != nil && route.Cacheable() {
		entry := cache.Entry{Question: query, Answer: response.Text, Language: language, MemoryVersion: memoryVersion, Created: time.Now(), Expires: time.Now().Add(24 * time.Hour), References: references, Sources: sources, Titles: titles, Vector: queryVector, EmbeddingModel: embeddingModel}
		s.Cache.Put(entry)
		if s.PersistCache != nil {
			s.PersistCache(context.WithoutCancel(ctx), entry)
		}
	}
	return nil
}
func (s *Service) lookupCache(ctx context.Context, query, language, memoryVersion string, exact, semantic bool) (cache.Entry, bool, error) {
	var hit cache.Entry
	var ok bool
	if exact {
		hit, ok = s.Cache.Get(query, language, memoryVersion, nil)
		if !ok {
			hit, ok = s.Cache.GetAny(query, memoryVersion)
		}
	}
	if !ok && semantic {
		hit, ok = s.Cache.FindSemantic(query, language, memoryVersion, .90)
		if !ok {
			hit, ok = s.Cache.FindSemanticAny(query, memoryVersion, .90)
		}
	}
	return s.prepareCacheHit(ctx, hit, ok, language)
}
func (s *Service) prepareCacheHit(ctx context.Context, hit cache.Entry, ok bool, language string) (cache.Entry, bool, error) {
	if !ok || !s.validCache(hit) {
		return cache.Entry{}, false, nil
	}
	if hit.Language == language {
		return hit, true, nil
	}
	if s.CacheTranslator == nil {
		return cache.Entry{}, false, nil
	}
	translated, err := cache.Translate(ctx, s.CacheTranslator, hit, language)
	if err != nil {
		return cache.Entry{}, false, err
	}
	translated.Created = time.Now()
	translated.Expires = hit.Expires
	s.Cache.Put(translated)
	if s.PersistCache != nil {
		s.PersistCache(context.WithoutCancel(ctx), translated)
	}
	return translated, true, nil
}
func (s *Service) lookupVectorCache(ctx context.Context, query, language, memoryVersion string, vector []float32, model string, options settings.AI) (cache.Entry, bool, error) {
	minimum := options.SemanticThreshold
	if options.SemanticCacheMode == "ai" {
		minimum = .75
	}
	candidates := s.Cache.VectorCandidates(vector, model, memoryVersion, minimum, 3)
	valid := candidates[:0]
	for _, candidate := range candidates {
		if s.validCache(candidate.Entry) {
			valid = append(valid, candidate)
		}
	}
	if len(valid) == 0 {
		return cache.Entry{}, false, nil
	}
	selected := -1
	if options.SemanticCacheMode == "threshold" {
		selected = 0
	} else {
		prompt, err := prompts.Get("cache-judge")
		if err != nil {
			return cache.Entry{}, false, err
		}
		payload := make([]map[string]any, len(valid))
		for index, candidate := range valid {
			answer := candidate.Entry.Answer
			if len(answer) > 2000 {
				answer = answer[:2000]
			}
			payload[index] = map[string]any{"index": index, "question": candidate.Entry.Question, "answer": answer, "similarity": candidate.Similarity}
		}
		encoded, err := json.Marshal(map[string]any{"question": query, "candidates": payload})
		if err != nil {
			return cache.Entry{}, false, err
		}
		_, err = s.Models.CompleteValidated(ctx, "cache-judge", []protocol.Message{{Role: "system", Content: prompt + "\n返回 JSON：{\"index\":候选序号}；没有等价候选时返回 -1。"}, {Role: "user", Content: string(encoded)}}, func(response protocol.Response) error {
			var judgment struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal([]byte(response.Text), &judgment); err != nil {
				return errors.New("缓存判断格式无效")
			}
			if judgment.Index < -1 || judgment.Index >= len(valid) {
				return errors.New("缓存判断索引无效")
			}
			selected = judgment.Index
			return nil
		})
		if err != nil {
			return cache.Entry{}, false, err
		}
	}
	if selected < 0 {
		return cache.Entry{}, false, nil
	}
	return s.prepareCacheHit(ctx, valid[selected].Entry, true, language)
}
func (s *Service) retrieveAll(ctx context.Context, queries []string, main string, vector []float32, model string) ([]retrieve.Unit, error) {
	type result struct {
		index int
		units []retrieve.Unit
		err   error
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(queries))
	for index, query := range queries {
		go func() {
			var units []retrieve.Unit
			var err error
			if query == main && len(vector) > 0 {
				if retriever, ok := s.Retriever.(VectorRetriever); ok {
					units, err = retriever.RetrieveVector(ctx, query, vector, model)
				} else {
					units, err = s.Retriever.Retrieve(ctx, query)
				}
			} else {
				units, err = s.Retriever.Retrieve(ctx, query)
			}
			results <- result{index: index, units: units, err: err}
		}()
	}
	ordered := make([][]retrieve.Unit, len(queries))
	for range queries {
		value := <-results
		if value.err != nil {
			cancel()
			return nil, value.err
		}
		ordered[value.index] = value.units
	}
	units := []retrieve.Unit{}
	for _, values := range ordered {
		units = append(units, values...)
	}
	return units, nil
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
func (s *Service) finish(ctx context.Context, session, question, normalized, language, answer string, citations []Citation, sendText bool, emit func(Event) error) error {
	if strings.TrimSpace(answer) == "" {
		return errors.New("模型未返回有效回答")
	}
	created := time.Now()
	sum := sha256.Sum256([]byte(session + question + created.String()))
	record := Record{ID: hex.EncodeToString(sum[:]), ConversationID: session, Question: question, NormalizedQuestion: normalized, Answer: answer, Language: language, Citations: citations, Created: created}
	if s.Persist != nil {
		if err := s.Persist(ctx, record); err != nil {
			return err
		}
	}
	if s.Conversations != nil {
		if err := s.Conversations.Append(session, Turn{Role: "user", Text: question, Created: created}); err != nil {
			return err
		}
		if err := s.Conversations.Append(session, Turn{Role: "assistant", Text: answer, Created: created, RecordID: record.ID}); err != nil {
			return err
		}
	}
	if s.Records != nil {
		s.Records.Add(record)
	}
	if sendText {
		if err := emit(Event{Type: "text", Text: answer}); err != nil {
			return err
		}
	}
	if len(citations) > 0 {
		if err := emit(Event{Type: "citations", Citations: citations}); err != nil {
			return err
		}
	}
	return emit(Event{Type: "done", ConversationID: session, RecordID: record.ID})
}
