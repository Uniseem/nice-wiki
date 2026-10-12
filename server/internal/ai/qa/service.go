package qa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"nice-wiki/internal/ai/cache"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/textutil"
	"time"
)

type Retriever interface {
	Retrieve(context.Context, string) ([]retrieve.Unit, error)
}
type Service struct {
	Models          *llm.Client
	Retriever       Retriever
	Cache           *cache.Store
	Limiter         *guard.Limiter
	MaxQuestion     int
	Conversations   *Conversations
	Records         *Records
	Persist         func(context.Context, Record) error
	ResolveCitation func(string) (string, string, bool)
	Budget          *guard.Budget
	Compactor       CompactModel
	MaxConversation int
}

func (s *Service) Ask(ctx context.Context, session, question, language, memoryVersion, memory string, emit func(Event) error) error {
	question, err := ValidateQuestion(question, s.MaxQuestion)
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
	if s.Conversations != nil {
		_ = s.Conversations.Append(session, Turn{Role: "user", Text: question, Created: time.Now()})
		if s.Compactor != nil {
			if err := Compact(ctx, s.Conversations, s.Compactor, session, s.MaxConversation); err != nil {
				return err
			}
		}
	}
	if hit, ok := s.Cache.Get(question, language, memoryVersion, nil); ok {
		return emit(Event{Type: "done", Text: hit.Answer, ConversationID: session})
	}
	if hit, ok := s.Cache.FindSemantic(question, language, memoryVersion, .90); ok {
		return emit(Event{Type: "done", Text: hit.Answer, ConversationID: session})
	}
	if err := emit(Event{Type: "progress", Message: "正在查找相关内容"}); err != nil {
		return err
	}
	units, err := s.Retriever.Retrieve(ctx, question)
	if err != nil {
		return err
	}
	response, err := Answer(ctx, s.Models, question, Assemble(units, 24000), memory)
	if err != nil {
		return err
	}
	if s.Budget != nil {
		estimated := float64(response.Usage.InputTokens+response.Usage.OutputTokens) / 1_000_000
		if err := s.Budget.Charge(session, estimated, time.Now()); err != nil {
			return err
		}
	}
	if err := emit(Event{Type: "text", Text: response.Text}); err != nil {
		return err
	}
	references := map[string]string{}
	citations := []Citation{}
	for _, unit := range units {
		references[unit.ID] = textutil.Hash([]byte(unit.Text))
		title, url := unit.Heading, ""
		if s.ResolveCitation != nil {
			if docTitle, path, ok := s.ResolveCitation(unit.DocumentID); ok {
				if title == "" {
					title = docTitle
				}
				url = "/" + language + "/" + path
			}
		}
		citations = append(citations, Citation{DocumentID: unit.DocumentID, BlockID: unit.ID, Title: title, URL: url})
	}
	if len(citations) > 0 {
		if err := emit(Event{Type: "citations", Citations: citations}); err != nil {
			return err
		}
	}
	if s.Conversations != nil {
		_ = s.Conversations.Append(session, Turn{Role: "assistant", Text: response.Text, Created: time.Now()})
	}
	if s.Records != nil {
		sum := sha256.Sum256([]byte(session + question + time.Now().String()))
		record := Record{ID: hex.EncodeToString(sum[:]), ConversationID: session, Question: question, Answer: response.Text, Language: language, Citations: citations, Created: time.Now()}
		s.Records.Add(record)
		if s.Persist != nil {
			if err := s.Persist(ctx, record); err != nil {
				return err
			}
		}
	}
	s.Cache.Put(cache.Entry{Question: question, Answer: response.Text, Language: language, MemoryVersion: memoryVersion, Created: time.Now(), Expires: time.Now().Add(24 * time.Hour), References: references})
	return emit(Event{Type: "done", ConversationID: session})
}
