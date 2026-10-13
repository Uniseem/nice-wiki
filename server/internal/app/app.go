package app

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"nice-wiki/internal/ai/cache"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/ingest"
	"nice-wiki/internal/ai/meta"
	"nice-wiki/internal/ai/qa"
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/aitask"
	"nice-wiki/internal/api"
	"nice-wiki/internal/assets"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/backup"
	"nice-wiki/internal/config"
	"nice-wiki/internal/content"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/history"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/i18n"
	"nice-wiki/internal/i18n/suggest"
	itranslate "nice-wiki/internal/i18n/translate"
	"nice-wiki/internal/jobs"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/memory"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/setup"
	"nice-wiki/internal/stats"
	"nice-wiki/internal/store"
	"nice-wiki/internal/store/sqlc"
	"nice-wiki/internal/textutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type App struct {
	Assets          *assets.Store
	Config          config.Bootstrap
	Logger          *slog.Logger
	Layout          datadir.Layout
	Lock            *datadir.Lock
	DB              *sql.DB
	Auth            *auth.Service
	Settings        *settings.Store
	Runtime         *config.RuntimeStore
	Secrets         *config.Secrets
	Content         *content.Repo
	Channels        *llm.Channels
	Setup           *setup.Service
	Jobs            *jobs.Service
	History         *history.Repo
	Memory          *memory.Store
	MemoryMapping   *memory.Mapping
	MemoryAssistant *memory.Assistant
	Catalog         *llm.Catalog
	Translations    *itranslate.Files
	Glossary        *itranslate.Glossary
	Locale          *suggest.Decider
	QA              *qa.Service
	Conversations   *qa.Conversations
	Records         *qa.Records
	Bans            *guard.Bans
	GuardEvents     *guard.Events
	Budget          *guard.Budget
	Debug           *qa.DebugStore
	AnswerCache     *cache.Store
}

func (a *App) CreateBackup(ctx context.Context, includeSecrets bool) (api.BackupResult, error) {
	name := fmt.Sprintf("nice-wiki-%s.tar.gz", time.Now().UTC().Format("20060102T150405Z"))
	target := filepath.Join(a.Layout.Backups(), name)
	if err := backup.Export(ctx, a.Layout.Root, target, backup.SQLSnapshotter{DB: a.DB}, includeSecrets); err != nil {
		return api.BackupResult{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return api.BackupResult{}, err
	}
	return api.BackupResult{Name: name, Size: info.Size(), CreatedAt: info.ModTime().Unix()}, nil
}
func (a *App) ListBackups(context.Context) ([]api.BackupResult, error) {
	entries, err := os.ReadDir(a.Layout.Backups())
	if err != nil {
		return nil, err
	}
	result := []api.BackupResult{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".gz" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, api.BackupResult{Name: entry.Name(), Size: info.Size(), CreatedAt: info.ModTime().Unix()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	return result, nil
}
func (a *App) OpenBackup(_ context.Context, name string) (io.ReadCloser, int64, error) {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".tar.gz") {
		return nil, 0, errors.New("备份文件名无效")
	}
	path := filepath.Join(a.Layout.Backups(), name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, errors.New("备份文件无效")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	return file, info.Size(), nil
}
func (a *App) RecordVisit(ctx context.Context, r *http.Request, path, language, referrer string) error {
	bot, ai := stats.Bot(r.UserAgent())
	day := time.Now().UTC().Format("2006-01-02")
	queries := sqlc.New(a.DB)
	salt, err := queries.GetDailySalt(ctx, day)
	if errors.Is(err, sql.ErrNoRows) {
		salt = make([]byte, 32)
		if _, err = crand.Read(salt); err != nil {
			return err
		}
		if _, err = a.DB.ExecContext(ctx, "INSERT INTO daily_salts(day,salt) VALUES(?,?)", day, salt); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	visitor := stats.Visitor(stats.Salt{Day: day, Value: salt}, httpx.ClientIP(ctx), r.UserAgent())
	botValue, aiValue := int64(0), int64(0)
	if bot {
		botValue = 1
	}
	if ai {
		aiValue = 1
	}
	return queries.RecordPageView(ctx, sqlc.RecordPageViewParams{DocumentID: path, Language: language, Country: "", ReferrerDomain: stats.NormalizeReferrer(referrer), VisitorHash: visitor, Bot: botValue, AiBot: aiValue, CreatedAt: time.Now().Unix()})
}
func (a *App) StatsSummary(ctx context.Context) (stats.Summary, error) {
	now := time.Now().UTC()
	values, err := sqlc.New(a.DB).ListDailyStats(ctx, sqlc.ListDailyStatsParams{Day: now.AddDate(0, 0, -30).Format("2006-01-02"), Day_2: now.Format("2006-01-02")})
	if err != nil {
		return stats.Summary{}, err
	}
	daily := make([]stats.Daily, 0, len(values))
	for _, v := range values {
		row := stats.Daily{Day: v.Day}
		switch v.Dimension {
		case "path":
			row.Path = v.Value
			row.Views = v.Views
			row.Visitors = v.Visitors
		case "ai":
			row.AICrawls = v.Views
		default:
			continue
		}
		daily = append(daily, row)
	}
	return stats.Summarize(daily, 10), nil
}
func (a *App) PersistQARecord(ctx context.Context, value qa.Record) error {
	if _, err := a.DB.ExecContext(ctx, "INSERT OR IGNORE INTO conversations(id,session_hash,language,summary,created_at,expires_at) VALUES(?,?,?,?,?,?)", value.ConversationID, value.ConversationID, value.Language, "", value.Created.Unix(), value.Created.Add(30*24*time.Hour).Unix()); err != nil {
		return err
	}
	citations, err := json.Marshal(value.Citations)
	if err != nil {
		return err
	}
	return sqlc.New(a.DB).InsertQARecord(ctx, sqlc.InsertQARecordParams{ID: value.ID, ConversationID: value.ConversationID, Question: value.Question, NormalizedQuestion: cache.Normalize(value.Question), Answer: value.Answer, Citations: string(citations), Language: value.Language, Status: "completed", CreatedAt: value.Created.Unix()})
}
func (a *App) ListQARecords(ctx context.Context) ([]qa.Record, error) {
	rows, err := sqlc.New(a.DB).ListQARecords(ctx, sqlc.ListQARecordsParams{Limit: 200, Offset: 0})
	if err != nil {
		return nil, err
	}
	result := make([]qa.Record, len(rows))
	for i, row := range rows {
		_ = json.Unmarshal([]byte(row.Citations), &result[i].Citations)
		result[i].ID = row.ID
		result[i].ConversationID = row.ConversationID
		result[i].Question = row.Question
		result[i].Answer = row.Answer
		result[i].Language = row.Language
		result[i].Created = time.Unix(row.CreatedAt, 0)
	}
	return result, nil
}
func (a *App) SaveQAFeedback(ctx context.Context, record, session string, helpful bool) error {
	value := int64(0)
	if helpful {
		value = 1
	}
	result, err := a.DB.ExecContext(ctx, "INSERT INTO qa_feedback(record_id,session_hash,helpful,created_at) SELECT id,?,?,? FROM qa_records WHERE id=? AND conversation_id=? ON CONFLICT(record_id,session_hash) DO UPDATE SET helpful=excluded.helpful,created_at=excluded.created_at", session, value, time.Now().Unix(), record, session)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (a *App) FeatureQAAnswer(ctx context.Context, record, title, sortKey string) error {
	var question, answer, language, citations string
	if err := a.DB.QueryRowContext(ctx, "SELECT question,answer,language,citations FROM qa_records WHERE id=?", record).Scan(&question, &answer, &language, &citations); err != nil {
		return err
	}
	entry, ok := a.curatedEntry(ctx, question, answer, language, citations)
	if !ok {
		return errors.New("回答引用已失效，不能加入精选缓存")
	}
	if _, err := a.DB.ExecContext(ctx, "INSERT INTO featured_answers(id,record_id,title,sort_key,created_at) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,sort_key=excluded.sort_key", record, record, title, sortKey, time.Now().Unix()); err != nil {
		return err
	}
	a.AnswerCache.Put(entry)
	return nil
}

// 旧记录缺少引用哈希时不自动进入共享缓存，避免重启复活过期内容。
func (a *App) curatedEntry(ctx context.Context, question, answer, language, encoded string) (cache.Entry, bool) {
	entry := cache.Entry{Question: question, Answer: answer, Language: language, Curated: true, References: map[string]string{}, Sources: map[string]string{}, Titles: map[string]string{}}
	var citations []qa.Citation
	if err := json.Unmarshal([]byte(encoded), &citations); err != nil {
		return cache.Entry{}, false
	}
	for _, citation := range citations {
		if citation.SourceHash == "" {
			return cache.Entry{}, false
		}
		var text string
		if err := a.DB.QueryRowContext(ctx, "SELECT b.text FROM blocks b JOIN documents d ON d.id=b.document_id WHERE b.id=? AND d.id=? AND d.published=1", citation.BlockID, citation.DocumentID).Scan(&text); err != nil || textutil.Hash([]byte(text)) != citation.SourceHash {
			return cache.Entry{}, false
		}
		entry.References[citation.BlockID] = citation.SourceHash
		entry.Sources[citation.BlockID] = citation.DocumentID
		entry.Titles[citation.BlockID] = citation.Title
	}
	_, version, err := memory.Inject(a.Memory, a.MemoryMapping, "qa-answer", language)
	if err != nil {
		return cache.Entry{}, false
	}
	entry.MemoryVersion = version
	return entry, true
}
func (a *App) UnfeatureQAAnswer(ctx context.Context, id string) error {
	var question, language string
	if err := a.DB.QueryRowContext(ctx, "SELECT question,language FROM qa_records WHERE id=?", id).Scan(&question, &language); err != nil {
		return err
	}
	if _, err := a.DB.ExecContext(ctx, "DELETE FROM featured_answers WHERE id=?", id); err != nil {
		return err
	}
	a.AnswerCache.Delete(question, language)
	return nil
}

type hybridRetriever struct {
	db        *sql.DB
	tiers     *llm.TierStore
	protocols llm.ProtocolRegistry
	usage     llm.UsageWriter
}

func (h hybridRetriever) Retrieve(ctx context.Context, question string) ([]retrieve.Unit, error) {
	models, err := h.tiers.Resolve("embed")
	if err != nil {
		return nil, err
	}
	model := models[0]
	embedder, err := h.protocols.Embedder(model)
	if err != nil {
		return nil, err
	}
	settle, err := llm.Reserve(ctx, llm.Cost(model, protocol.Usage{InputTokens: int64(len(question) + 64)}))
	if err != nil {
		return nil, err
	}
	vectors, usage, err := embedder.Embed(ctx, model.ID, []string{question})
	status := "success"
	if err != nil {
		status = "error"
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		usage.InputTokens = int64(len(question) + 64)
		status = "estimated_" + status
	}
	cost := llm.Cost(model, usage)
	settle(cost)
	if h.usage != nil {
		accounting, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		usageErr := h.usage.WriteUsage(accounting, "embed", model, usage, cost, status)
		cancel()
		if usageErr != nil {
			return nil, errors.Join(err, usageErr)
		}
	}
	if err != nil {
		return nil, err
	}
	index := retrieve.NewVectorIndex()
	rows, err := h.db.QueryContext(ctx, "SELECT unit_id,dimensions,vector FROM vectors WHERE model=?", model.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var dimensions int
		var raw []byte
		if err := rows.Scan(&id, &dimensions, &raw); err != nil {
			_ = rows.Close()
			return nil, err
		}
		reader := bytes.NewReader(raw)
		value := make([]float32, dimensions)
		for i := range value {
			var bits uint32
			if err := binary.Read(reader, binary.LittleEndian, &bits); err != nil {
				_ = rows.Close()
				return nil, err
			}
			value[i] = math.Float32frombits(bits)
		}
		index.Set(id, value)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, errors.New("嵌入结果数量无效")
	}
	vectorHits := index.Search(vectors[0], 20)
	tokens := textutil.Tokens(question)
	keywordHits := []retrieve.Hit{}
	if len(tokens) > 0 {
		parts := make([]string, len(tokens))
		for i, token := range tokens {
			parts[i] = "\"" + strings.ReplaceAll(token, "\"", "\"\"") + "\""
		}
		keywordRows, err := h.db.QueryContext(ctx, "SELECT unit_id,-bm25(keyword_index) FROM keyword_index WHERE keyword_index MATCH ? ORDER BY bm25(keyword_index) LIMIT 20", strings.Join(parts, " OR "))
		if err == nil {
			for keywordRows.Next() {
				var hit retrieve.Hit
				if keywordRows.Scan(&hit.ID, &hit.Score) == nil {
					keywordHits = append(keywordHits, hit)
				}
			}
			_ = keywordRows.Close()
		}
	}
	fused := retrieve.Fuse(vectorHits, keywordHits)
	if len(fused) > 20 {
		fused = fused[:20]
	}
	result := []retrieve.Unit{}
	for _, hit := range fused {
		var value retrieve.Unit
		if err := h.db.QueryRowContext(ctx, "SELECT u.id,u.document_id,u.text FROM retrieval_units u JOIN documents d ON d.id=u.document_id JOIN blocks b ON b.id=u.id WHERE u.id=? AND d.published=1 AND b.text=u.text", hit.ID).Scan(&value.ID, &value.DocumentID, &value.Text); err == nil {
			result = append(result, value)
		}
	}
	return result, nil
}

type memoryAssistantModel struct{ client *llm.Client }

func (m memoryAssistantModel) Propose(ctx context.Context, message string, items []memory.Item) ([]memory.Proposal, error) {
	data, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	response, err := m.client.Complete(ctx, "memory-assistant", []protocol.Message{{Role: "system", Content: "根据现有记忆提出 JSON 数组修改建议。"}, {Role: "user", Content: string(data) + "\n用户消息：" + message}})
	if err != nil {
		return nil, err
	}
	return llm.DecodeStructured[[]memory.Proposal](response.Text)
}

type translationModel struct{ client *llm.Client }
type compactModel struct{ client *llm.Client }

func (m compactModel) Compact(ctx context.Context, summary string, turns []qa.Turn) (string, error) {
	data, err := json.Marshal(map[string]any{"summary": summary, "turns": turns})
	if err != nil {
		return "", err
	}
	response, err := m.client.Complete(ctx, "compact-conversation", []protocol.Message{{Role: "user", Content: string(data)}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Text), nil
}

type overviewModel struct{ client *llm.Client }

func (m overviewModel) Summarize(ctx context.Context, text string) (string, error) {
	response, err := m.client.Complete(ctx, "site-summary", []protocol.Message{{Role: "user", Content: text}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Text), nil
}

func (m translationModel) Translate(ctx context.Context, language string, segments []itranslate.Segment, terms []itranslate.Term) (map[string]string, error) {
	data, err := json.Marshal(map[string]any{"language": language, "segments": segments, "terms": terms})
	if err != nil {
		return nil, err
	}
	response, err := m.client.Complete(ctx, "translate-blocks", []protocol.Message{{Role: "user", Content: string(data)}})
	if err != nil {
		return nil, err
	}
	return llm.DecodeStructured[map[string]string](response.Text)
}
func (m translationModel) TranslateStrings(ctx context.Context, language string, values map[string]string) (map[string]string, error) {
	data, err := json.Marshal(map[string]any{"language": language, "strings": values})
	if err != nil {
		return nil, err
	}
	response, err := m.client.Complete(ctx, "translate-strings", []protocol.Message{{Role: "user", Content: string(data)}})
	if err != nil {
		return nil, err
	}
	return llm.DecodeStructured[map[string]string](response.Text)
}

func New(ctx context.Context, cfg config.Bootstrap, logger *slog.Logger) (result *App, err error) {
	layout, err := datadir.New(cfg.Data)
	if err != nil {
		return nil, err
	}
	if err := layout.Initialize(); err != nil {
		return nil, err
	}
	lock, err := layout.Acquire()
	if err != nil {
		return nil, err
	}
	app := &App{Config: cfg, Logger: logger, Layout: layout, Lock: lock}
	defer func() {
		if err != nil {
			if app.DB != nil {
				err = errors.Join(err, app.DB.Close())
			}
			err = errors.Join(err, setup.RollbackPending(layout))
			err = errors.Join(err, lock.Close())
		}
	}()
	if err := setup.ApplyPending(layout); err != nil {
		return nil, err
	}
	if err := layout.Initialize(); err != nil {
		return nil, err
	}
	app.DB, err = store.Open(ctx, layout.Database())
	if err != nil {
		return nil, err
	}
	if err = store.Migrate(ctx, app.DB); err != nil {
		return nil, err
	}
	app.Runtime, err = config.OpenRuntime(layout.Runtime())
	if err != nil {
		return nil, err
	}
	secrets, err := config.OpenSecrets(layout.Secrets())
	if err != nil {
		return nil, err
	}
	app.Secrets = secrets
	app.Settings, err = settings.Open(layout, nil, i18n.Known)
	if err != nil {
		return nil, err
	}
	app.Memory, err = memory.Open(layout)
	if err != nil {
		return nil, err
	}
	app.MemoryMapping, err = memory.OpenMapping(layout)
	if err != nil {
		return nil, err
	}
	app.Catalog, err = llm.OpenCatalog(layout.ModelCatalog())
	if err != nil {
		return nil, err
	}
	app.Translations = itranslate.NewFiles(layout)
	app.Glossary, err = itranslate.OpenGlossary(layout.Glossary())
	if err != nil {
		return nil, err
	}
	app.Locale = suggest.NewDecider(nil)
	app.Auth, err = auth.Open(layout.Accounts(), sqlc.New(app.DB))
	if err != nil {
		return nil, err
	}
	app.Channels, err = llm.OpenChannels(layout.Channels(), secrets)
	if err != nil {
		return nil, err
	}
	versions, err := history.Open(layout.Root)
	if err != nil {
		return nil, err
	}
	app.History = versions
	app.Content, err = content.Open(ctx, layout, app.DB, versions, nil)
	if err != nil {
		return nil, err
	}
	tiers := llm.NewTiers()
	protocols := llm.ProtocolRegistry{Channels: app.Channels, HTTP: http.DefaultClient}
	tiers.Source = func(tier aitask.Tier) []llm.Model {
		result := []llm.Model{}
		for _, model := range app.Catalog.List() {
			capability := llm.Chat
			switch tier {
			case aitask.Embedding:
				capability = llm.Embedding
			case aitask.Vision:
				capability = llm.Vision
			case aitask.Rerank:
				capability = llm.Rerank
			}
			if !model.Has(capability) {
				continue
			}
			if tier == aitask.Embedding {
				if _, err := protocols.Embedder(model); err != nil {
					continue
				}
			} else {
				if _, err := protocols.Chat(model); err != nil {
					continue
				}
			}
			result = append(result, model)
		}
		return result
	}
	ready := func() bool {
		_, chatErr := tiers.Resolve("qa-answer")
		_, embedErr := tiers.Resolve("embed")
		return chatErr == nil && embedErr == nil
	}
	app.Settings.SetAIReadiness(ready)
	modelsClient := llm.NewClient(tiers, protocols, usageWriter{db: app.DB})
	app.Content.SetGenerators(meta.Slugger{Client: modelsClient}, meta.SEO{Client: modelsClient})
	app.MemoryAssistant = &memory.Assistant{Store: app.Memory, Model: memoryAssistantModel{client: modelsClient}, Pending: memory.NewProposals()}
	app.Conversations = qa.NewConversations(30 * 24 * time.Hour)
	app.Records = &qa.Records{Retention: 30 * 24 * time.Hour}
	app.AnswerCache = cache.New()
	if rows, queryErr := app.DB.QueryContext(ctx, "SELECT q.question,q.answer,q.language,q.citations FROM featured_answers f JOIN qa_records q ON q.id=f.record_id ORDER BY f.sort_key"); queryErr == nil {
		for rows.Next() {
			var question, answer, language, citations string
			if rows.Scan(&question, &answer, &language, &citations) == nil {
				if entry, ok := app.curatedEntry(ctx, question, answer, language, citations); ok {
					app.AnswerCache.Put(entry)
				}
			}
		}
		_ = rows.Close()
	}
	app.Bans = guard.NewBans()
	app.GuardEvents = &guard.Events{}
	app.Budget = guard.NewBudget(app.Runtime.Get().DailySpendLimit, 0)
	app.Debug = qa.NewDebugStore()
	app.Budget.Limit = func() float64 { return app.Runtime.Get().DailySpendLimit }
	app.Budget.Load = func(now time.Time) (float64, error) {
		var total float64
		err := app.DB.QueryRow("SELECT COALESCE(SUM(cost_usd),0) FROM llm_usage WHERE created_at>=? AND created_at<? AND task IN ('qa-route','qa-answer','compact-conversation','embed')", now.UTC().Truncate(24*time.Hour).Unix(), now.UTC().Truncate(24*time.Hour).Add(24*time.Hour).Unix()).Scan(&total)
		return total, err
	}
	qaRetriever := hybridRetriever{db: app.DB, tiers: tiers, protocols: protocols, usage: usageWriter{db: app.DB}}
	app.QA = &qa.Service{Models: modelsClient, Retriever: qaRetriever, Cache: app.AnswerCache, Limiter: guard.NewLimiter(10), MaxQuestion: app.Settings.AI().MaxQuestionLength, Conversations: app.Conversations, Records: app.Records, Persist: app.PersistQARecord, Budget: app.Budget, Compactor: compactModel{client: modelsClient}, MaxConversation: 64000, ResolveCitation: func(id string) (string, string, bool) {
		file, err := app.Content.Get(id)
		if err != nil || !file.Meta.Published {
			return "", "", false
		}
		path, err := app.Content.Path(id)
		return file.Meta.Title, path, err == nil
	}}
	app.QA.Ready = ready
	app.QA.Settings = app.Settings.AI
	app.QA.BaseLanguage = func() string { return app.Settings.Languages().Base }
	app.QA.CurrentReference = func(id, hash string) bool {
		var text string
		err := app.DB.QueryRow("SELECT b.text FROM blocks b JOIN documents d ON d.id=b.document_id WHERE b.id=? AND d.published=1", id).Scan(&text)
		return err == nil && textutil.Hash([]byte(text)) == hash
	}
	app.QA.TranslatedText = func(documentID, blockID, language string) (string, bool) {
		if language == app.Settings.Languages().Base {
			return "", false
		}
		file, err := app.Content.Get(documentID)
		if err != nil || !file.Meta.Published {
			return "", false
		}
		translated, err := app.Translations.ReadOptional(language, documentID)
		if err != nil {
			return "", false
		}
		block, exists := translated.Blocks[blockID]
		if !exists {
			return "", false
		}
		source, err := doc.Blocks(file.Document)
		if err != nil {
			return "", false
		}
		current := false
		for _, value := range source {
			if value.ID == blockID && value.ContentHash == block.SourceHash {
				current = true
				break
			}
		}
		if !current {
			return "", false
		}
		rendered, err := doc.Blocks(itranslate.Apply(file.Document, translated))
		if err != nil {
			return "", false
		}
		for _, value := range rendered {
			if value.ID == blockID {
				return value.Text, true
			}
		}
		return "", false
	}
	app.Content.SetChangedHook(app.AnswerCache.InvalidateAll)
	app.Assets = assets.Open(layout, app.DB, assetSources{content: app.Content, settings: app.Settings}, logger)
	app.Setup = &setup.Service{Auth: app.Auth, Settings: app.Settings, Channels: app.Channels, Layout: layout}
	app.Jobs, err = jobs.Open(app.DB, &mergeStore{db: app.DB})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.Cleanup, func(ctx context.Context, _ string, progress func(int)) error {
		if err := sqlc.New(app.DB).DeleteExpiredSessions(ctx, time.Now().Unix()); err != nil {
			return err
		}
		progress(100)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.Rollup, func(ctx context.Context, _ string, progress func(int)) error {
		if err := stats.Rollup(ctx, stats.SQLStore{DB: app.DB}, time.Now().UTC().AddDate(0, 0, -1), 30); err != nil {
			return err
		}
		progress(100)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.Backup, func(ctx context.Context, _ string, progress func(int)) error {
		if _, err := app.CreateBackup(ctx, false); err != nil {
			return err
		}
		if err := backup.Retain(app.Layout.Backups(), app.Runtime.Get().BackupKeep); err != nil {
			return err
		}
		runtime := app.Runtime.Get()
		if runtime.GitRemote != "" {
			username, password := "", ""
			_ = app.Secrets.Use("git-username", func(value string) error { username = value; return nil })
			_ = app.Secrets.Use("git-password", func(value string) error { password = value; return nil })
			if err := app.History.Push(ctx, runtime.GitRemote, username, password); err != nil {
				return err
			}
		}
		progress(100)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.IndexDocument, func(ctx context.Context, id string, progress func(int)) error {
		file, err := app.Content.Get(id)
		if err != nil {
			return err
		}
		if !file.Meta.Published {
			return nil
		}
		units, err := ingest.Units(id, file.Document)
		if err != nil {
			return err
		}
		err = app.Content.WithPublished(id, file.Meta.Revision, func() error {
			return store.Transaction(ctx, app.DB, func(tx *sql.Tx) error {
				for _, statement := range []string{"DELETE FROM keyword_index WHERE document_id=?", "DELETE FROM retrieval_unit_blocks WHERE unit_id IN (SELECT id FROM retrieval_units WHERE document_id=?)", "DELETE FROM vectors WHERE unit_id IN (SELECT id FROM retrieval_units WHERE document_id=?)", "DELETE FROM retrieval_units WHERE document_id=?"} {
					if _, err := tx.ExecContext(ctx, statement, id); err != nil {
						return err
					}
				}
				for _, unit := range units {
					hash := textutil.Hash([]byte(unit.Text))
					if _, err := tx.ExecContext(ctx, "INSERT INTO retrieval_units(id,document_id,kind,text,content_hash,updated_at) VALUES(?,?,?,?,?,?)", unit.ID, id, "text", unit.Text, hash, time.Now().Unix()); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, "INSERT INTO retrieval_unit_blocks(unit_id,block_id,position) VALUES(?,?,?)", unit.ID, unit.ID, unit.Position); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, "INSERT INTO keyword_index(unit_id,document_id,text) VALUES(?,?,?)", unit.ID, id, unit.Text); err != nil {
						return err
					}
				}
				return nil
			})
		})
		if errors.Is(err, content.ErrConflict) {
			return nil
		}
		if err != nil {
			return err
		}
		progress(50)
		resolved, err := tiers.Resolve("embed")
		if err != nil {
			return err
		}
		embedder, err := protocols.Embedder(resolved[0])
		if err != nil {
			return err
		}
		texts := make([]string, len(units))
		for index := range units {
			texts[index] = units[index].Text
		}
		vectors, _, err := embedder.Embed(ctx, resolved[0].ID, texts)
		if err != nil {
			return err
		}
		if len(vectors) != len(units) {
			return errors.New("嵌入结果数量无效")
		}
		err = app.Content.WithPublished(id, file.Meta.Revision, func() error {
			return store.Transaction(ctx, app.DB, func(tx *sql.Tx) error {
				for index, vector := range vectors {
					var encoded bytes.Buffer
					for _, number := range vector {
						if err := binary.Write(&encoded, binary.LittleEndian, math.Float32bits(number)); err != nil {
							return err
						}
					}
					hash := textutil.Hash([]byte(units[index].Text))
					if _, err := tx.ExecContext(ctx, "INSERT INTO vectors(unit_id,dimensions,vector,model,content_hash) VALUES(?,?,?,?,?) ON CONFLICT(unit_id) DO UPDATE SET dimensions=excluded.dimensions,vector=excluded.vector,model=excluded.model,content_hash=excluded.content_hash", units[index].ID, len(vector), encoded.Bytes(), resolved[0].ID, hash); err != nil {
						return err
					}
				}
				return nil
			})
		})
		if err == nil {
			progress(100)
		}
		if errors.Is(err, content.ErrConflict) {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	translator := itranslate.Pipeline{Files: app.Translations, Glossary: app.Glossary, Model: translationModel{client: modelsClient}, BatchSize: 4000, ReuseMinimum: 20}
	err = app.Jobs.Register(jobs.TranslateDocument, func(ctx context.Context, id string, progress func(int)) error {
		file, err := app.Content.Get(id)
		if err != nil {
			return err
		}
		if !file.Meta.Published {
			return nil
		}
		taskTranslator := translator
		taskTranslator.Commit = func(commit func() error) error { return app.Content.WithPublished(id, file.Meta.Revision, commit) }
		blocks, err := doc.Blocks(file.Document)
		if err != nil {
			return err
		}
		segments := []itranslate.Segment{}
		hashes := map[string]string{}
		for _, block := range blocks {
			if len(block.Translatable) == 0 {
				continue
			}
			segments = append(segments, itranslate.Segment{ID: block.ID, Text: strings.Join(block.Translatable, "\n"), Heading: block.Kind == "heading"})
			hashes[block.ID] = block.ContentHash
		}
		languages := app.Settings.Languages()
		targets := []string{}
		for _, language := range languages.Enabled {
			if language != languages.Base {
				targets = append(targets, language)
			}
		}
		for index, language := range targets {
			if _, err := taskTranslator.Run(ctx, id, language, segments, hashes); err != nil {
				if errors.Is(err, content.ErrConflict) {
					return nil
				}
				return err
			}
			progress((index + 1) * 100 / max(1, len(targets)))
		}
		if len(targets) == 0 {
			progress(100)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.UpdateSummary, func(ctx context.Context, _ string, progress func(int)) error {
		var text strings.Builder
		for _, meta := range app.Content.List() {
			if !meta.Published {
				continue
			}
			file, err := app.Content.Get(meta.ID)
			if err != nil {
				return err
			}
			plain, err := doc.RenderText(file.Document)
			if err != nil {
				return err
			}
			text.WriteString("# " + meta.Title + "\n" + plain + "\n")
		}
		if err := memory.UpdateOverview(ctx, app.Memory, overviewModel{client: modelsClient}, text.String()); err != nil {
			return err
		}
		progress(100)
		return nil
	})
	if err != nil {
		return nil, err
	}
	app.Content.SetPublishedHook(func(ctx context.Context, id string) error {
		app.AnswerCache.InvalidateAll()
		if !app.Settings.AI().Enabled {
			return nil
		}
		if err := app.Jobs.Enqueue(ctx, jobs.IndexDocument, id); err != nil {
			return err
		}
		if err := app.Jobs.Enqueue(ctx, jobs.TranslateDocument, id); err != nil {
			return err
		}
		return app.Jobs.Enqueue(ctx, jobs.UpdateSummary, "project")
	})
	err = app.Jobs.Register(jobs.TranslateSite, func(ctx context.Context, _ string, progress func(int)) error {
		site, home, banner := app.Settings.Site(), app.Settings.Home(), app.Settings.Banner()
		source := map[string]string{"site.name": site.Name, "site.description": site.Description, "home.title": home.Title, "home.subtitle": home.Subtitle, "banner.text": banner.Text}
		ui := map[string]string{"site.documentTree": "文档目录", "site.pageToc": "本页目录", "site.previous": "上一篇", "site.next": "下一篇", "site.theme": "亮暗主题", "site.copyCode": "复制", "site.copyMarkdown": "复制为 Markdown"}
		languages := app.Settings.Languages()
		targets := []string{}
		for _, language := range languages.Enabled {
			if language != languages.Base {
				targets = append(targets, language)
			}
		}
		model := translationModel{client: modelsClient}
		for index, language := range targets {
			sitePath, err := app.Layout.TranslationStrings(language, "site")
			if err != nil {
				return err
			}
			if err := itranslate.TranslateStrings(ctx, model, language, sitePath, source); err != nil {
				return err
			}
			uiPath, err := app.Layout.TranslationStrings(language, "ui")
			if err != nil {
				return err
			}
			if err := itranslate.TranslateStrings(ctx, model, language, uiPath, ui); err != nil {
				return err
			}
			progress((index + 1) * 100 / max(1, len(targets)))
		}
		if len(targets) == 0 {
			progress(100)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.RebuildIndex, func(ctx context.Context, _ string, progress func(int)) error {
		documents := app.Content.List()
		published := []content.Meta{}
		for _, document := range documents {
			if document.Published {
				published = append(published, document)
			}
		}
		for index, document := range published {
			if err := app.Jobs.Enqueue(ctx, jobs.IndexDocument, document.ID); err != nil {
				return err
			}
			progress((index + 1) * 100 / max(1, len(published)))
		}
		if len(published) == 0 {
			progress(100)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	app.Settings.SetOnChange(func(string) {
		if app.Settings.AI().Enabled {
			if err := app.Jobs.Enqueue(context.Background(), jobs.TranslateSite, "settings"); err != nil {
				app.Logger.Error("站点文字翻译任务投递失败")
			}
		}
	})
	if err = app.Auth.CheckReset(ctx, layout.ResetSuperadmin()); err != nil {
		return nil, err
	}
	if err := setup.CompleteRestore(layout); err != nil {
		return nil, err
	}
	return app, nil
}

// 数据库能力由组装层注入，jobs 保持既定依赖方向。
type mergeStore struct{ db *sql.DB }
type usageWriter struct{ db *sql.DB }

func (u usageWriter) WriteUsage(ctx context.Context, task string, model llm.Model, usage protocol.Usage, cost float64, status string) error {
	return sqlc.New(u.db).RecordUsage(ctx, sqlc.RecordUsageParams{ChannelID: model.Channel, Model: model.ID, Task: task, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, CachedTokens: 0, CostUsd: cost, Status: status, CreatedAt: time.Now().Unix()})
}

func (s *mergeStore) Enqueue(ctx context.Context, kind jobs.Kind, subject string, save func(*sql.Tx, int64) error) (queued bool, err error) {
	err = store.Transaction(ctx, s.db, func(tx *sql.Tx) error {
		queries := sqlc.New(tx)
		if err := queries.MarkJobGeneration(ctx, sqlc.MarkJobGenerationParams{Kind: string(kind), SubjectID: subject, UpdatedAt: time.Now().Unix()}); err != nil {
			return err
		}
		value, err := queries.GetJobGeneration(ctx, sqlc.GetJobGenerationParams{Kind: string(kind), SubjectID: subject})
		if err != nil {
			return err
		}
		if value.QueuedGeneration != 0 {
			return nil
		}
		if err := save(tx, value.Generation); err != nil {
			return err
		}
		if err := queries.SetQueuedGeneration(ctx, sqlc.SetQueuedGenerationParams{QueuedGeneration: value.Generation, Kind: string(kind), SubjectID: subject}); err != nil {
			return err
		}
		queued = true
		return nil
	})
	return queued, err
}
func (s *mergeStore) Current(ctx context.Context, kind jobs.Kind, subject string) (int64, error) {
	value, err := sqlc.New(s.db).GetJobGeneration(ctx, sqlc.GetJobGenerationParams{Kind: string(kind), SubjectID: subject})
	return value.Generation, err
}
func (s *mergeStore) Complete(ctx context.Context, kind jobs.Kind, subject string, generation int64) error {
	_, err := sqlc.New(s.db).ClearQueuedGeneration(ctx, sqlc.ClearQueuedGenerationParams{Kind: string(kind), SubjectID: subject, QueuedGeneration: generation})
	return err
}

// 业务服务之间的适配留在组装层。
type assetSources struct {
	content  *content.Repo
	settings *settings.Store
}

func (s assetSources) WithDocuments(visit func([]assets.ReferenceDocument) error) error {
	return s.settings.WithAssetSettings(func(site settings.Site, home settings.Home) error {
		return s.content.WithResourceDocuments(func(values []content.SourceDocument) error {
			result := []assets.ReferenceDocument{}
			for _, value := range values {
				result = append(result, assets.ReferenceDocument{ID: value.ID, Source: value.Source, Document: value.Document})
			}
			result = append(result, assets.ReferenceDocument{
				ID: "site-settings", Source: "settings",
				Document: doc.Document{SchemaVersion: doc.SchemaVersion, Content: doc.Node{
					Type: "doc", Content: []doc.Node{
						{Type: "image", Attrs: map[string]any{"id": "site-logo", "hash": site.LogoHash}},
						{Type: "image", Attrs: map[string]any{"id": "home-background", "hash": home.BackgroundHash, "posterHash": home.VideoPosterHash}},
					},
				}},
			})
			return visit(result)
		})
	})
}
