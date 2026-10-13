package app

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/base64"
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
	"nice-wiki/internal/geoip"
	"nice-wiki/internal/history"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/i18n"
	"nice-wiki/internal/i18n/suggest"
	itranslate "nice-wiki/internal/i18n/translate"
	"nice-wiki/internal/jobs"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/memory"
	"nice-wiki/internal/prompts"
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
	Assets              *assets.Store
	Config              config.Bootstrap
	Logger              *slog.Logger
	Layout              datadir.Layout
	Lock                *datadir.Lock
	DB                  *sql.DB
	Auth                *auth.Service
	Settings            *settings.Store
	Runtime             *config.RuntimeStore
	Secrets             *config.Secrets
	Content             *content.Repo
	Channels            *llm.Channels
	Setup               *setup.Service
	Jobs                *jobs.Service
	History             *history.Repo
	Memory              *memory.Store
	MemoryMapping       *memory.Mapping
	MemoryAssistant     *memory.Assistant
	Catalog             *llm.Catalog
	Translations        *itranslate.Files
	Glossary            *itranslate.Glossary
	Locale              *suggest.Decider
	QA                  *qa.Service
	Conversations       *qa.Conversations
	Records             *qa.Records
	Bans                *guard.Bans
	GuardEvents         *guard.Events
	Budget              *guard.Budget
	IPBudget            *guard.Budget
	Debug               *qa.DebugStore
	AnswerCache         *cache.Store
	TranslationEstimate func(int) itranslate.Estimate
	GeoIP               *geoip.Database
}

type answerCachePayload struct {
	Language       string            `json:"language"`
	References     map[string]string `json:"references"`
	Sources        map[string]string `json:"sources"`
	Titles         map[string]string `json:"titles"`
	Vector         []float32         `json:"vector,omitempty"`
	EmbeddingModel string            `json:"embeddingModel,omitempty"`
}
type descriptionCacheStore struct{ db *sql.DB }

func (s descriptionCacheStore) GetDescription(ctx context.Context, hash, task string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT description FROM description_cache WHERE asset_hash=? AND context_hash='' AND task=?", hash, task).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}
func (s descriptionCacheStore) PutDescription(ctx context.Context, hash, task, value string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO description_cache(asset_hash,context_hash,task,description,model,updated_at) VALUES(?,'',?,?,?,?) ON CONFLICT(asset_hash,context_hash,task) DO UPDATE SET description=excluded.description,model=excluded.model,updated_at=excluded.updated_at", hash, task, value, "configured", time.Now().Unix())
	return err
}

type assetDescriptionModel struct {
	client   *llm.Client
	assets   *assets.Store
	memory   *memory.Store
	mapping  *memory.Mapping
	language func() string
}
type unitContextModel struct {
	client   *llm.Client
	document string
	memory   string
}

func (m unitContextModel) Contextualize(ctx context.Context, unit retrieve.Unit) (string, error) {
	prompt, err := prompts.Get("context")
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(map[string]any{"document": m.document, "unit": unit})
	if err != nil {
		return "", err
	}
	response, err := m.client.Complete(ctx, "contextualize", []protocol.Message{{Role: "system", Content: m.memory + "\n" + prompt}, {Role: "user", Content: string(data)}})
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(response.Text)
	if runes := []rune(value); len(runes) > 1000 {
		value = string(runes[:1000])
	}
	return value, nil
}

func (m assetDescriptionModel) Describe(ctx context.Context, hash, kind string) (string, error) {
	metadata, reader, err := m.assets.OpenFile(ctx, hash)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, (32<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 32<<20 {
		return "", errors.New("媒体描述输入无效或过大")
	}
	prompt, err := prompts.Get("describe-image")
	if err != nil {
		return "", err
	}
	language := ""
	if m.language != nil {
		language = m.language()
	}
	injected, _, err := memory.Inject(m.memory, m.mapping, "describe-image", language)
	if err != nil {
		return "", err
	}
	message := protocol.Message{Role: "user", Content: "请描述这个" + kind + "，文件名：" + metadata.Name}
	if strings.HasPrefix(metadata.MIME, "image/") {
		message.Images = []string{"data:" + metadata.MIME + ";base64," + base64.StdEncoding.EncodeToString(data)}
	} else {
		if len(data) > 1<<20 {
			data = data[:1<<20]
		}
		message.Content += "\n场景内容：\n" + string(data)
	}
	response, err := m.client.Complete(ctx, "describe-image", []protocol.Message{{Role: "system", Content: injected + "\n" + prompt}, message})
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(response.Text)
	if value == "" {
		return "", errors.New("媒体描述为空")
	}
	return value, nil
}

func (a *App) CreateBackup(ctx context.Context, includeSecrets bool) (api.BackupResult, error) {
	name := fmt.Sprintf("nice-wiki-%s.tar.gz", time.Now().UTC().Format("20060102T150405.000000000Z"))
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
	languages := a.Settings.Languages()
	enabled := false
	for _, code := range languages.Enabled {
		enabled = enabled || code == language
	}
	if !enabled || len(path) > 2048 {
		return nil
	}
	relative := strings.TrimPrefix(path, "/")
	if relative == "llms.txt" || relative == "llms-full.txt" {
		path = "@llms"
	} else {
		if relative == language {
			relative = ""
		} else if strings.HasPrefix(relative, language+"/") {
			relative = strings.TrimPrefix(relative, language+"/")
		} else {
			return nil
		}
		relative = strings.TrimSuffix(relative, ".md")
		switch relative {
		case "":
			path = "@home"
		case "ask":
			path = "@ask"
		case "llms.txt", "llms-full.txt":
			path = "@llms"
		default:
			file, ok := a.Content.ResolvePath(relative)
			if !ok || !file.Meta.Published {
				return nil
			}
			path = file.Meta.ID
		}
	}
	bot, ai := stats.Bot(r.UserAgent())
	day := time.Now().UTC().Format("2006-01-02")
	queries := sqlc.New(a.DB)
	salt, err := queries.GetDailySalt(ctx, day)
	if errors.Is(err, sql.ErrNoRows) {
		candidate := make([]byte, 32)
		if _, err = crand.Read(candidate); err != nil {
			return err
		}
		if _, err = a.DB.ExecContext(ctx, "INSERT OR IGNORE INTO daily_salts(day,salt) VALUES(?,?)", day, candidate); err != nil {
			return err
		}
		salt, err = queries.GetDailySalt(ctx, day)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	visitor := stats.Visitor(stats.Salt{Day: day, Value: salt}, httpx.ClientIP(ctx), r.UserAgent())
	country := ""
	if a.GeoIP != nil {
		country, err = a.GeoIP.Lookup(httpx.ClientIP(ctx))
		if err != nil {
			return err
		}
	}
	botValue, aiValue := int64(0), int64(0)
	if bot {
		botValue = 1
	}
	if ai {
		aiValue = 1
	}
	return queries.RecordPageView(ctx, sqlc.RecordPageViewParams{DocumentID: path, Language: language, Country: country, ReferrerDomain: stats.NormalizeReferrer(referrer), VisitorHash: visitor, Bot: botValue, AiBot: aiValue, CreatedAt: time.Now().Unix()})
}
func (a *App) RecordCrawler(ctx context.Context, r *http.Request, path, language string) error {
	_, ai := stats.Bot(r.UserAgent())
	if !ai {
		return nil
	}
	return a.RecordVisit(ctx, r, path, language, "")
}
func (a *App) StatsSummary(ctx context.Context) (stats.Summary, error) {
	now := time.Now().UTC()
	values, err := sqlc.New(a.DB).ListDailyStats(ctx, sqlc.ListDailyStatsParams{Day: now.AddDate(0, 0, -30).Format("2006-01-02"), Day_2: now.Format("2006-01-02")})
	if err != nil {
		return stats.Summary{}, err
	}
	daily := make([]stats.Daily, 0, len(values))
	for _, v := range values {
		row := stats.Daily{Day: v.Day, Dimension: v.Dimension, Value: v.Value, Views: v.Views, Visitors: v.Visitors}
		switch v.Dimension {
		case "path":
			row.Path = v.Value
		case "ai":
			row.AICrawls = v.Views
		}
		daily = append(daily, row)
	}
	summary := stats.Summarize(daily, 10)
	for index := range summary.Popular {
		id := summary.Popular[index].Title
		if id == "@home" {
			summary.Popular[index].Title = "首页"
			continue
		}
		if id == "@ask" {
			summary.Popular[index].Title = "AI 问答"
			continue
		}
		if file, getErr := a.Content.Get(id); getErr == nil {
			summary.Popular[index].Title = file.Meta.Title
			continue
		}
		// 兼容修复前按公开路径记录的历史汇总。
		legacy := strings.TrimPrefix(id, "/")
		if at := strings.IndexByte(legacy, '/'); at >= 0 {
			legacy = legacy[at+1:]
		}
		if file, ok := a.Content.ResolvePath(strings.TrimSuffix(legacy, ".md")); ok {
			summary.Popular[index].Title = file.Meta.Title
		}
	}
	return summary, nil
}
func (a *App) PersistQARecord(ctx context.Context, value qa.Record) error {
	if _, err := a.DB.ExecContext(ctx, "INSERT INTO conversations(id,session_hash,language,summary,created_at,expires_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET language=excluded.language,expires_at=excluded.expires_at", value.ConversationID, value.ConversationID, value.Language, "", value.Created.Unix(), value.Created.Add(30*24*time.Hour).Unix()); err != nil {
		return err
	}
	citations, err := json.Marshal(value.Citations)
	if err != nil {
		return err
	}
	normalized := value.NormalizedQuestion
	if normalized == "" {
		normalized = value.Question
	}
	return sqlc.New(a.DB).InsertQARecord(ctx, sqlc.InsertQARecordParams{ID: value.ID, ConversationID: value.ConversationID, Question: value.Question, NormalizedQuestion: cache.Normalize(normalized), Answer: value.Answer, Citations: string(citations), Language: value.Language, Status: "completed", CreatedAt: value.Created.Unix()})
}
func (a *App) PersistAnswerCache(ctx context.Context, entry cache.Entry) {
	payload, err := json.Marshal(answerCachePayload{Language: entry.Language, References: entry.References, Sources: entry.Sources, Titles: entry.Titles, Vector: entry.Vector, EmbeddingModel: entry.EmbeddingModel})
	if err != nil {
		a.Logger.Error("回答缓存编码失败")
		return
	}
	materialHash, err := textutil.HashJSON(entry.References)
	if err != nil {
		a.Logger.Error("回答缓存材料哈希失败")
		return
	}
	key := cache.Key(entry.Question, entry.Language)
	_, err = a.DB.ExecContext(ctx, "INSERT INTO answer_cache(id,question_hash,question,answer,citations,material_hash,memory_hash,hits,created_at,expires_at) VALUES(?,?,?,?,?,?,?,0,?,?) ON CONFLICT(question_hash) DO UPDATE SET question=excluded.question,answer=excluded.answer,citations=excluded.citations,material_hash=excluded.material_hash,memory_hash=excluded.memory_hash,created_at=excluded.created_at,expires_at=excluded.expires_at", key, key, entry.Question, entry.Answer, string(payload), materialHash, entry.MemoryVersion, entry.Created.Unix(), entry.Expires.Unix())
	if err != nil {
		a.Logger.Error("回答缓存持久化失败")
	}
}
func (a *App) ClearAnswerCache(ctx context.Context) error {
	_, err := a.DB.ExecContext(ctx, "DELETE FROM answer_cache")
	if err == nil && a.AnswerCache != nil {
		a.AnswerCache.InvalidateAll()
	}
	return err
}
func (a *App) LoadQAConversation(ctx context.Context, session string) (qa.Conversation, bool, error) {
	var summary string
	if err := a.DB.QueryRowContext(ctx, "SELECT summary FROM conversations WHERE id=? AND session_hash=? AND expires_at>?", session, session, time.Now().Unix()).Scan(&summary); errors.Is(err, sql.ErrNoRows) {
		return qa.Conversation{}, false, nil
	} else if err != nil {
		return qa.Conversation{}, false, err
	}
	rows, err := a.DB.QueryContext(ctx, "SELECT id,question,answer,created_at FROM qa_records WHERE conversation_id=? AND status='completed' ORDER BY created_at,rowid", session)
	if err != nil {
		return qa.Conversation{}, false, err
	}
	defer func() { _ = rows.Close() }()
	value := qa.Conversation{ID: session, Summary: summary, Turns: []qa.Turn{}}
	for rows.Next() {
		var id, question, answer string
		var created int64
		if err := rows.Scan(&id, &question, &answer, &created); err != nil {
			return qa.Conversation{}, false, err
		}
		when := time.Unix(created, 0)
		value.Turns = append(value.Turns, qa.Turn{Role: "user", Text: question, Created: when}, qa.Turn{Role: "assistant", Text: answer, Created: when, RecordID: id})
	}
	if err := rows.Err(); err != nil {
		return qa.Conversation{}, false, err
	}
	return value, true, nil
}
func (a *App) SaveQAConversationSummary(ctx context.Context, session, summary string) error {
	_, err := a.DB.ExecContext(ctx, "UPDATE conversations SET summary=? WHERE id=? AND session_hash=?", summary, session, session)
	return err
}
func (a *App) ClearQAConversation(ctx context.Context, session string) error {
	return store.Transaction(ctx, a.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE conversations SET summary='' WHERE id=? AND session_hash=?", session, session); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE qa_records SET status='cleared' WHERE conversation_id=?", session)
		return err
	})
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
	if !helpful {
		var question, language string
		if err := a.DB.QueryRowContext(ctx, "SELECT normalized_question,language FROM qa_records WHERE id=? AND conversation_id=?", record, session).Scan(&question, &language); err != nil {
			return err
		}
		a.AnswerCache.DeleteAutomatic(question, language)
		if _, err := a.DB.ExecContext(ctx, "DELETE FROM answer_cache WHERE question_hash=?", cache.Key(question, language)); err != nil {
			return err
		}
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

type retrievalPlanner struct {
	client   *llm.Client
	store    *memory.Store
	mapping  *memory.Mapping
	language func() string
}
type retrievalDecision struct {
	Action     string `json:"action"`
	Query      string `json:"query"`
	DocumentID string `json:"documentId"`
	Heading    string `json:"heading"`
}

func (p retrievalPlanner) Decide(ctx context.Context, question string, units []retrieve.Unit, observations []string, round int) (retrievalDecision, error) {
	prompt, err := prompts.Get("verify")
	if err != nil {
		return retrievalDecision{}, err
	}
	injected := ""
	if p.store != nil && p.mapping != nil {
		language := ""
		if p.language != nil {
			language = p.language()
		}
		injected, _, err = memory.Inject(p.store, p.mapping, "retrieve-verify", language)
		if err != nil {
			return retrievalDecision{}, err
		}
	}
	if len(units) > 30 {
		units = units[:30]
	}
	data, err := json.Marshal(map[string]any{"question": question, "round": round + 1, "materials": units, "observations": observations})
	if err != nil {
		return retrievalDecision{}, err
	}
	var decision retrievalDecision
	_, err = p.client.CompleteValidated(ctx, "retrieve-verify", []protocol.Message{{Role: "system", Content: injected + "\n" + prompt + "\n只返回 JSON 动作：{\"action\":\"done|search|read_section|tree\",\"query\":\"\",\"documentId\":\"\",\"heading\":\"\"}。"}, {Role: "user", Content: string(data)}}, func(response protocol.Response) error {
		value, decodeErr := llm.DecodeStructured[retrievalDecision](response.Text)
		if decodeErr != nil {
			return decodeErr
		}
		value.Query = strings.TrimSpace(value.Query)
		value.DocumentID = strings.TrimSpace(value.DocumentID)
		value.Heading = strings.TrimSpace(value.Heading)
		switch value.Action {
		case "done", "tree":
		case "search":
			if value.Query == "" || len([]rune(value.Query)) > 500 {
				return errors.New("检索核查返回无效查询")
			}
		case "read_section":
			if !datadir.ValidID(value.DocumentID) || value.Heading == "" || len([]rune(value.Heading)) > 500 {
				return errors.New("检索核查返回无效小节参数")
			}
		default:
			return errors.New("检索核查返回未知动作")
		}
		decision = value
		return nil
	})
	return decision, err
}

type hybridRetriever struct {
	db        *sql.DB
	tiers     *llm.TierStore
	protocols llm.ProtocolRegistry
	usage     llm.UsageWriter
}

type agenticRetriever struct {
	base    hybridRetriever
	planner retrievalPlanner
}

func (r agenticRetriever) EmbedQuery(ctx context.Context, question string) ([]float32, string, error) {
	return r.base.EmbedQuery(ctx, question)
}
func (r agenticRetriever) Retrieve(ctx context.Context, question string) ([]retrieve.Unit, error) {
	initial, err := r.base.Retrieve(ctx, question)
	if err != nil {
		return nil, err
	}
	return r.continueRetrieval(ctx, question, initial)
}
func (r agenticRetriever) RetrieveVector(ctx context.Context, question string, vector []float32, model string) ([]retrieve.Unit, error) {
	initial, err := r.base.RetrieveVector(ctx, question, vector, model)
	if err != nil {
		return nil, err
	}
	return r.continueRetrieval(ctx, question, initial)
}
func (r agenticRetriever) continueRetrieval(ctx context.Context, question string, values []retrieve.Unit) ([]retrieve.Unit, error) {
	result := append([]retrieve.Unit{}, values...)
	observations := []string{}
	for round := 0; round < 2; round++ {
		decision, err := r.planner.Decide(ctx, question, result, observations, round)
		if err != nil {
			if len(result) > 0 {
				break
			}
			return nil, err
		}
		if decision.Action == "done" {
			break
		}
		var more []retrieve.Unit
		switch decision.Action {
		case "search":
			more, err = r.base.Retrieve(ctx, decision.Query)
		case "read_section":
			more, err = r.base.ReadSection(ctx, decision.DocumentID, decision.Heading)
		case "tree":
			var tree string
			tree, err = r.base.Tree(ctx)
			if err == nil {
				observations = append(observations, tree)
			}
		}
		if err != nil {
			if len(result) > 0 {
				break
			}
			return nil, err
		}
		result = append(result, more...)
	}
	seen := map[string]bool{}
	unique := result[:0]
	for _, unit := range result {
		if !seen[unit.ID] {
			seen[unit.ID] = true
			unique = append(unique, unit)
		}
	}
	return unique, nil
}

func (h hybridRetriever) Retrieve(ctx context.Context, question string) ([]retrieve.Unit, error) {
	vector, model, err := h.EmbedQuery(ctx, question)
	if err != nil {
		return nil, err
	}
	return h.RetrieveVector(ctx, question, vector, model)
}
func (h hybridRetriever) EmbedQuery(ctx context.Context, question string) ([]float32, string, error) {
	models, err := h.tiers.Resolve("embed")
	if err != nil {
		return nil, "", err
	}
	model := models[0]
	embedder, err := h.protocols.Embedder(model)
	if err != nil {
		return nil, "", err
	}
	settle, err := llm.Reserve(ctx, llm.Cost(model, protocol.Usage{InputTokens: int64(len(question) + 64)}))
	if err != nil {
		return nil, "", err
	}
	release, err := h.protocols.Acquire(ctx, model.Channel)
	if err != nil {
		settle(0)
		return nil, "", err
	}
	vectors, usage, err := embedder.Embed(ctx, model.ID, []string{question})
	release()
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
			return nil, "", errors.Join(err, usageErr)
		}
	}
	if err != nil {
		return nil, "", err
	}
	if len(vectors) != 1 {
		return nil, "", errors.New("嵌入结果数量无效")
	}
	return vectors[0], model.ID, nil
}
func (h hybridRetriever) RetrieveVector(ctx context.Context, question string, queryVector []float32, modelID string) ([]retrieve.Unit, error) {
	index := retrieve.NewVectorIndex()
	rows, err := h.db.QueryContext(ctx, "SELECT unit_id,dimensions,vector FROM vectors WHERE model=?", modelID)
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
		if dimensions < 1 || dimensions > 65536 || len(raw) != dimensions*4 {
			_ = rows.Close()
			return nil, errors.New("向量索引数据无效")
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
	vectorHits := index.Search(queryVector, 20)
	tokens := textutil.Tokens(question)
	keywordHits := []retrieve.Hit{}
	if len(tokens) > 0 {
		parts := make([]string, len(tokens))
		for i, token := range tokens {
			parts[i] = "\"" + strings.ReplaceAll(token, "\"", "\"\"") + "\""
		}
		keywordRows, err := h.db.QueryContext(ctx, "SELECT unit_id,-bm25(keyword_index) FROM keyword_index WHERE keyword_index MATCH ? ORDER BY bm25(keyword_index) LIMIT 20", strings.Join(parts, " OR "))
		if err != nil {
			return nil, err
		}
		for keywordRows.Next() {
			var hit retrieve.Hit
			if err := keywordRows.Scan(&hit.ID, &hit.Score); err != nil {
				_ = keywordRows.Close()
				return nil, err
			}
			keywordHits = append(keywordHits, hit)
		}
		if err := keywordRows.Err(); err != nil {
			_ = keywordRows.Close()
			return nil, err
		}
		if err := keywordRows.Close(); err != nil {
			return nil, err
		}
	}
	fused := retrieve.Fuse(vectorHits, keywordHits)
	fused, err = h.rerank(ctx, question, fused)
	if err != nil {
		return nil, err
	}
	if len(fused) > 20 {
		fused = fused[:20]
	}
	return h.expandSections(ctx, fused)
}

func (h hybridRetriever) rerank(ctx context.Context, question string, hits []retrieve.Hit) ([]retrieve.Hit, error) {
	models, err := h.tiers.Resolve("rerank")
	if err != nil || len(hits) < 2 {
		return hits, nil
	}
	model := models[0]
	reranker, err := h.protocols.Reranker(model)
	if err != nil {
		return nil, err
	}
	valid := []retrieve.Hit{}
	documents := []string{}
	input := int64(len(question) + 64)
	for _, hit := range hits {
		var text string
		if err := h.db.QueryRowContext(ctx, "SELECT u.text FROM retrieval_units u JOIN documents d ON d.id=u.document_id WHERE u.id=? AND d.published=1", hit.ID).Scan(&text); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		valid = append(valid, hit)
		documents = append(documents, text)
		input += int64(len(text) + 32)
	}
	if len(valid) < 2 {
		return valid, nil
	}
	settle, err := llm.Reserve(ctx, llm.Cost(model, protocol.Usage{InputTokens: input}))
	if err != nil {
		return nil, err
	}
	release, err := h.protocols.Acquire(ctx, model.Channel)
	if err != nil {
		settle(0)
		return nil, err
	}
	scores, usage, callErr := reranker.Rerank(ctx, model.ID, question, documents)
	release()
	status := "success"
	if callErr != nil {
		status = "error"
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		usage.InputTokens = input
		status = "estimated_" + status
	}
	cost := llm.Cost(model, usage)
	settle(cost)
	if h.usage != nil {
		accounting, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		usageErr := h.usage.WriteUsage(accounting, "rerank", model, usage, cost, status)
		cancel()
		if usageErr != nil {
			return nil, errors.Join(callErr, usageErr)
		}
	}
	if callErr != nil {
		return valid, nil
	}
	if len(scores) != len(valid) {
		return nil, errors.New("重排结果数量无效")
	}
	for index := range valid {
		valid[index].Score = scores[index]
	}
	sort.SliceStable(valid, func(i, j int) bool { return valid[i].Score > valid[j].Score })
	return valid, nil
}

func (h hybridRetriever) expandSections(ctx context.Context, hits []retrieve.Hit) ([]retrieve.Unit, error) {
	result := []retrieve.Unit{}
	seen := map[string]bool{}
	for _, hit := range hits {
		var documentID, headingPath string
		err := h.db.QueryRowContext(ctx, "SELECT b.document_id,b.heading_path FROM retrieval_units u JOIN retrieval_unit_blocks rb ON rb.unit_id=u.id JOIN blocks b ON b.id=rb.block_id JOIN documents d ON d.id=b.document_id WHERE u.id=? AND d.published=1 ORDER BY rb.position LIMIT 1", hit.ID).Scan(&documentID, &headingPath)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rows, err := h.db.QueryContext(ctx, "SELECT id,text,position,heading_path FROM blocks WHERE document_id=? AND heading_path=? ORDER BY position LIMIT 24", documentID, headingPath)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var unit retrieve.Unit
			var encoded string
			if err := rows.Scan(&unit.ID, &unit.Text, &unit.Position, &encoded); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if seen[unit.ID] || strings.TrimSpace(unit.Text) == "" {
				continue
			}
			var headings []string
			if err := json.Unmarshal([]byte(encoded), &headings); err != nil {
				_ = rows.Close()
				return nil, err
			}
			unit.DocumentID = documentID
			if len(headings) > 0 {
				unit.Heading = headings[len(headings)-1]
			}
			seen[unit.ID] = true
			result = append(result, unit)
			if len(result) >= 80 {
				_ = rows.Close()
				return result, nil
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (h hybridRetriever) ReadSection(ctx context.Context, documentID, heading string) ([]retrieve.Unit, error) {
	rows, err := h.db.QueryContext(ctx, "SELECT b.id,b.text,b.position,b.heading_path FROM blocks b JOIN documents d ON d.id=b.document_id WHERE b.document_id=? AND d.published=1 ORDER BY b.position", documentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []retrieve.Unit{}
	for rows.Next() {
		var unit retrieve.Unit
		var encoded string
		if err := rows.Scan(&unit.ID, &unit.Text, &unit.Position, &encoded); err != nil {
			return nil, err
		}
		var headings []string
		if err := json.Unmarshal([]byte(encoded), &headings); err != nil {
			return nil, err
		}
		if len(headings) == 0 || headings[len(headings)-1] != heading || strings.TrimSpace(unit.Text) == "" {
			continue
		}
		unit.DocumentID = documentID
		unit.Heading = heading
		result = append(result, unit)
		if len(result) >= 24 {
			break
		}
	}
	return result, rows.Err()
}
func (h hybridRetriever) Tree(ctx context.Context) (string, error) {
	rows, err := h.db.QueryContext(ctx, "SELECT id,parent_id,title,path FROM documents WHERE published=1 ORDER BY sort_key,id")
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	values := []map[string]string{}
	for rows.Next() {
		var id, parent, title, path string
		if err := rows.Scan(&id, &parent, &title, &path); err != nil {
			return "", err
		}
		values = append(values, map[string]string{"id": id, "parentId": parent, "title": title, "path": path})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}

type memoryAssistantModel struct{ client *llm.Client }

func completeStructured[T any](ctx context.Context, client *llm.Client, task string, messages []protocol.Message) (T, error) {
	var result T
	_, err := client.CompleteValidated(ctx, task, messages, func(response protocol.Response) error {
		value, decodeErr := llm.DecodeStructured[T](response.Text)
		if decodeErr == nil {
			result = value
		}
		return decodeErr
	})
	return result, err
}

func (m memoryAssistantModel) Propose(ctx context.Context, message string, items []memory.Item) ([]memory.Proposal, error) {
	data, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	prompt, err := prompts.Get("memory-assistant")
	if err != nil {
		return nil, err
	}
	return completeStructured[[]memory.Proposal](ctx, m.client, "memory-assistant", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(data) + "\n用户消息：" + message}})
}

type translationModel struct{ client *llm.Client }
type compactModel struct{ client *llm.Client }
type cacheAnswerTranslator struct {
	client  *llm.Client
	store   *memory.Store
	mapping *memory.Mapping
}
type localeModel struct{ client *llm.Client }

func (m localeModel) DecideLanguage(ctx context.Context, signals suggest.Signals, enabled []string) (string, error) {
	prompt, err := prompts.Get("language-suggest")
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(map[string]any{"signals": signals, "enabled": enabled})
	if err != nil {
		return "", err
	}
	var result struct {
		Language   string  `json:"language"`
		Confidence float64 `json:"confidence"`
	}
	_, err = m.client.CompleteValidated(ctx, "language-suggest", []protocol.Message{{Role: "system", Content: prompt + "\n返回 JSON：{\"language\":\"语言代码\",\"confidence\":0.0}"}, {Role: "user", Content: string(data)}}, func(response protocol.Response) error {
		value, decodeErr := llm.DecodeStructured[struct {
			Language   string  `json:"language"`
			Confidence float64 `json:"confidence"`
		}](response.Text)
		if decodeErr != nil {
			return decodeErr
		}
		if value.Confidence < 0.75 {
			return errors.New("语言建议置信度不足")
		}
		result = value
		return nil
	})
	return result.Language, err
}

func (m cacheAnswerTranslator) TranslateAnswer(ctx context.Context, answer, source, target string) (string, error) {
	prompt, err := prompts.Get("translate-answer")
	if err != nil {
		return "", err
	}
	injected, _, err := memory.Inject(m.store, m.mapping, "translate-answer", target)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(map[string]string{"sourceLanguage": source, "targetLanguage": target, "answer": answer})
	if err != nil {
		return "", err
	}
	response, err := m.client.Complete(ctx, "translate-answer", []protocol.Message{{Role: "system", Content: injected + "\n" + prompt}, {Role: "user", Content: string(data)}})
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(response.Text)
	if value == "" {
		return "", errors.New("缓存回答译文为空")
	}
	return value, nil
}

func (m compactModel) Compact(ctx context.Context, summary string, turns []qa.Turn) (string, error) {
	data, err := json.Marshal(map[string]any{"summary": summary, "turns": turns})
	if err != nil {
		return "", err
	}
	prompt, err := prompts.Get("compact-conversation")
	if err != nil {
		return "", err
	}
	response, err := m.client.Complete(ctx, "compact-conversation", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(data)}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Text), nil
}

type overviewModel struct{ client *llm.Client }

func (m overviewModel) Summarize(ctx context.Context, text string) (string, error) {
	prompt, err := prompts.Get("site-summary")
	if err != nil {
		return "", err
	}
	response, err := m.client.Complete(ctx, "site-summary", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: text}})
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
	prompt, err := prompts.Get("translate-blocks")
	if err != nil {
		return nil, err
	}
	return completeStructured[map[string]string](ctx, m.client, "translate-blocks", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(data)}})
}
func (m translationModel) TranslateStrings(ctx context.Context, language string, values map[string]string) (map[string]string, error) {
	data, err := json.Marshal(map[string]any{"language": language, "strings": values})
	if err != nil {
		return nil, err
	}
	prompt, err := prompts.Get("translate-strings")
	if err != nil {
		return nil, err
	}
	return completeStructured[map[string]string](ctx, m.client, "translate-strings", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(data)}})
}
func (m translationModel) ExtractTerms(ctx context.Context, language, text string) ([]itranslate.Term, error) {
	prompt, err := prompts.Get("extract-terms")
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]string{"language": language, "text": text})
	if err != nil {
		return nil, err
	}
	return completeStructured[[]itranslate.Term](ctx, m.client, "extract-terms", []protocol.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(data)}})
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
			if app.GeoIP != nil {
				err = errors.Join(err, app.GeoIP.Close())
			}
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
	app.Catalog, err = llm.OpenCatalogWithSnapshot(layout.ModelCatalog())
	if err != nil {
		return nil, err
	}
	app.GeoIP, err = geoip.Open(layout.GeoIPDatabase())
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
		assignments := app.Settings.ModelTiers()
		selected := assignments[string(tier)]
		fallbackVision := false
		if tier == aitask.Vision && len(selected) == 0 {
			selected = assignments[string(aitask.Fast)]
			fallbackVision = true
		}
		catalog := map[string]llm.Model{}
		for _, model := range llm.AvailableModels(app.Catalog, app.Channels) {
			catalog[model.Channel+":"+model.ID] = model
		}
		result := []llm.Model{}
		for _, key := range selected {
			model, exists := catalog[key]
			if !exists {
				continue
			}
			capability := llm.Chat
			switch tier {
			case aitask.Embedding:
				capability = llm.Embedding
			case aitask.Vision:
				capability = llm.Vision
			case aitask.Rerank:
				capability = llm.Rerank
			}
			if !model.Has(capability) || (tier == aitask.Vision && !model.Has(llm.Chat)) || (fallbackVision && !model.Has(llm.Vision)) {
				continue
			}
			if tier == aitask.Embedding {
				if _, err := protocols.Embedder(model); err != nil {
					continue
				}
			} else if tier == aitask.Rerank {
				if _, err := protocols.Reranker(model); err != nil {
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
		_, routeErr := tiers.Resolve("qa-route")
		_, answerErr := tiers.Resolve("qa-answer")
		_, embedErr := tiers.Resolve("embed")
		return routeErr == nil && answerErr == nil && embedErr == nil
	}
	app.Settings.SetAIReadiness(ready)
	modelsClient := llm.NewClient(tiers, protocols, usageWriter{db: app.DB})
	app.Locale = suggest.NewDecider(localeModel{client: modelsClient})
	app.TranslationEstimate = func(characters int) itranslate.Estimate {
		models, err := tiers.Resolve("translate-blocks")
		if err != nil || len(models) == 0 {
			return itranslate.EstimateCost(characters, 0, 0)
		}
		return itranslate.EstimateCost(characters, models[0].InputPrice, models[0].OutputPrice)
	}
	app.Content.SetGenerators(meta.Slugger{Client: modelsClient}, meta.SEO{Client: modelsClient})
	app.MemoryAssistant = &memory.Assistant{Store: app.Memory, Model: memoryAssistantModel{client: modelsClient}, Pending: memory.NewProposals(), Mapping: app.MemoryMapping}
	app.Conversations = qa.NewConversations(30 * 24 * time.Hour)
	app.Records = &qa.Records{Retention: 30 * 24 * time.Hour}
	app.AnswerCache = cache.New()
	if rows, queryErr := app.DB.QueryContext(ctx, "SELECT question,answer,citations,memory_hash,created_at,expires_at FROM answer_cache WHERE expires_at>?", time.Now().Unix()); queryErr == nil {
		for rows.Next() {
			var question, answer, encoded, memoryVersion string
			var created, expires int64
			if rows.Scan(&question, &answer, &encoded, &memoryVersion, &created, &expires) != nil {
				continue
			}
			var payload answerCachePayload
			if json.Unmarshal([]byte(encoded), &payload) != nil || payload.Language == "" {
				continue
			}
			app.AnswerCache.Put(cache.Entry{Question: question, Answer: answer, Language: payload.Language, MemoryVersion: memoryVersion, Created: time.Unix(created, 0), Expires: time.Unix(expires, 0), References: payload.References, Sources: payload.Sources, Titles: payload.Titles, Vector: payload.Vector, EmbeddingModel: payload.EmbeddingModel})
		}
		_ = rows.Close()
	}
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
	app.IPBudget = guard.NewBudget(0, 0)
	app.Debug = qa.NewDebugStore()
	app.Budget.Limit = func() float64 { return app.Runtime.Get().DailySpendLimit }
	app.Budget.HourlyLimit = func() float64 { return app.Runtime.Get().SessionHourlySpendLimit }
	app.IPBudget.HourlyLimit = func() float64 { return app.Runtime.Get().IPHourlySpendLimit }
	app.Budget.Load = func(now time.Time) (float64, error) {
		var total float64
		err := app.DB.QueryRow("SELECT COALESCE(SUM(cost_usd),0) FROM llm_usage WHERE created_at>=? AND created_at<? AND task IN ('qa-route','qa-answer','compact-conversation','embed')", now.UTC().Truncate(24*time.Hour).Unix(), now.UTC().Truncate(24*time.Hour).Add(24*time.Hour).Unix()).Scan(&total)
		return total, err
	}
	qaRetriever := hybridRetriever{db: app.DB, tiers: tiers, protocols: protocols, usage: usageWriter{db: app.DB}}
	agent := agenticRetriever{base: qaRetriever, planner: retrievalPlanner{client: modelsClient, store: app.Memory, mapping: app.MemoryMapping, language: func() string { return app.Settings.Languages().Base }}}
	app.QA = &qa.Service{Models: modelsClient, Retriever: agent, Cache: app.AnswerCache, CacheTranslator: cacheAnswerTranslator{client: modelsClient, store: app.Memory, mapping: app.MemoryMapping}, Limiter: guard.NewLimiter(10), MaxQuestion: app.Settings.AI().MaxQuestionLength, Conversations: app.Conversations, Records: app.Records, Persist: app.PersistQARecord, LoadConversation: app.LoadQAConversation, SaveSummary: app.SaveQAConversationSummary, Budget: app.Budget, IPBudget: app.IPBudget, Compactor: compactModel{client: modelsClient}, MaxConversation: 64000, RouteAll: true, Bans: app.Bans, Events: app.GuardEvents, StreamAnswers: true, PersistCache: app.PersistAnswerCache, ResolveCitation: func(id string) (string, string, bool) {
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
	app.Content.SetChangedHook(func() {
		if err := app.ClearAnswerCache(context.Background()); err != nil {
			app.Logger.Error("正文变化后清理回答缓存失败")
		}
	})
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
		blocks, err := doc.Blocks(file.Document)
		if err != nil {
			return err
		}
		unitIndex := map[string]int{}
		for index, unit := range units {
			unitIndex[unit.ID] = index
		}
		descriptionModel := assetDescriptionModel{client: modelsClient, assets: app.Assets, memory: app.Memory, mapping: app.MemoryMapping, language: func() string { return app.Settings.Languages().Base }}
		descriptionStore := descriptionCacheStore{db: app.DB}
		generatedDefaults := map[string]string{}
		for _, media := range ingest.MediaItems(file.Document) {
			if media.ID == "" {
				continue
			}
			description := strings.TrimSpace(media.Title + " " + media.Description)
			switch media.Kind {
			case "image", "drawing":
				if media.Hash != "" {
					generated, describeErr := ingest.Describe(ctx, descriptionStore, descriptionModel, media.Hash, media.Kind)
					if describeErr == nil {
						description = strings.TrimSpace(description + "\n" + generated)
						if strings.TrimSpace(media.Title) == "" {
							generatedDefaults[media.ID] = generated
						}
					} else if description == "" {
						app.Logger.Warn("媒体描述跳过", "kind", media.Kind)
					}
				}
			case "attachment":
				if media.Hash != "" {
					metadata, reader, openErr := app.Assets.OpenFile(ctx, media.Hash)
					if openErr == nil {
						extracted, extractErr := ingest.ExtractAttachment(ctx, metadata.Name, metadata.MIME, reader, nil)
						_ = reader.Close()
						if extractErr == nil {
							if len(extracted) > 8000 {
								extracted = extracted[:8000]
							}
							description = strings.TrimSpace(description + "\n" + extracted)
						}
					}
				}
			case "video":
				description = strings.TrimSpace(description + " " + media.Name)
			}
			if description == "" {
				continue
			}
			position := -1
			heading := ""
			blockHash := ""
			for index, block := range blocks {
				if block.ID == media.ID {
					position = block.Position
					blockHash = block.ContentHash
					if len(block.HeadingPath) > 0 {
						heading = block.HeadingPath[len(block.HeadingPath)-1]
					}
					for _, neighbor := range []int{index - 1, index + 1} {
						if neighbor >= 0 && neighbor < len(blocks) && strings.TrimSpace(blocks[neighbor].Text) != "" {
							description += "\n上下文：" + blocks[neighbor].Text
						}
					}
					break
				}
			}
			if index, ok := unitIndex[media.ID]; ok {
				units[index].Text = strings.TrimSpace(units[index].Text + "\n" + description)
				units[index].SourceHash = textutil.Hash([]byte(units[index].SourceHash + "\x00" + textutil.Hash([]byte(description))))
			} else {
				unitIndex[media.ID] = len(units)
				units = append(units, retrieve.Unit{ID: media.ID, DocumentID: id, Heading: heading, Text: description, Position: position, BlockIDs: []string{media.ID}, SourceHash: textutil.Hash([]byte(blockHash + "\x00" + textutil.Hash([]byte(description))))})
			}
		}
		if len(generatedDefaults) > 0 {
			changed, err := app.Content.ApplyGeneratedDescriptions(ctx, id, file.Meta.Revision, generatedDefaults)
			if errors.Is(err, content.ErrConflict) {
				return nil
			}
			if err != nil {
				return err
			}
			if changed {
				if err := app.Jobs.Enqueue(ctx, jobs.IndexDocument, id); err != nil {
					return err
				}
				if err := app.Jobs.Enqueue(ctx, jobs.TranslateDocument, id); err != nil {
					return err
				}
				progress(100)
				return nil
			}
		}
		resolved, err := tiers.Resolve("embed")
		if err != nil {
			return err
		}
		model := resolved[0]
		current := map[string]retrieve.Unit{}
		changed := []retrieve.Unit{}
		for index, unit := range units {
			current[unit.ID] = unit
			var storedText, storedContext, storedSource string
			var storedModel, storedVectorHash sql.NullString
			err := app.DB.QueryRowContext(ctx, "SELECT u.text,u.context,u.content_hash,v.model,v.content_hash FROM retrieval_units u LEFT JOIN vectors v ON v.unit_id=u.id WHERE u.id=?", unit.ID).Scan(&storedText, &storedContext, &storedSource, &storedModel, &storedVectorHash)
			if errors.Is(err, sql.ErrNoRows) || err == nil && (storedSource != unit.SourceHash || !storedModel.Valid || storedModel.String != model.ID || !storedVectorHash.Valid || storedVectorHash.String != textutil.Hash([]byte(storedText))) {
				changed = append(changed, unit)
			} else if err != nil {
				return err
			} else {
				units[index].Text = storedText
				units[index].Context = storedContext
			}
		}
		if len(changed) > 0 {
			documentText, err := doc.RenderText(file.Document)
			if err != nil {
				return err
			}
			if runes := []rune(documentText); len(runes) > 200000 {
				documentText = string(runes[:200000])
			}
			injected, _, err := memory.Inject(app.Memory, app.MemoryMapping, "contextualize", app.Settings.Languages().Base)
			if err != nil {
				return err
			}
			changed, err = ingest.Contextualize(ctx, unitContextModel{client: modelsClient, document: documentText, memory: injected}, changed)
			if err != nil {
				return err
			}
			for _, unit := range changed {
				if index, ok := unitIndex[unit.ID]; ok {
					units[index] = unit
				}
			}
		}
		stale := []string{}
		rows, err := app.DB.QueryContext(ctx, "SELECT id FROM retrieval_units WHERE document_id=?", id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var unitID string
			if err := rows.Scan(&unitID); err != nil {
				_ = rows.Close()
				return err
			}
			if _, ok := current[unitID]; !ok {
				stale = append(stale, unitID)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		changedIDs := map[string]bool{}
		for _, unit := range changed {
			changedIDs[unit.ID] = true
		}
		err = app.Content.WithPublished(id, file.Meta.Revision, func() error {
			return store.Transaction(ctx, app.DB, func(tx *sql.Tx) error {
				for _, unitID := range stale {
					if _, err := tx.ExecContext(ctx, "DELETE FROM keyword_index WHERE unit_id=?", unitID); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, "DELETE FROM retrieval_units WHERE id=?", unitID); err != nil {
						return err
					}
				}
				for _, unit := range units {
					if _, err := tx.ExecContext(ctx, "INSERT INTO retrieval_units(id,document_id,kind,text,context,content_hash,embedding_model,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET document_id=excluded.document_id,kind=excluded.kind,text=excluded.text,context=excluded.context,content_hash=excluded.content_hash,embedding_model=excluded.embedding_model,updated_at=excluded.updated_at", unit.ID, id, "text", unit.Text, unit.Context, unit.SourceHash, model.ID, time.Now().Unix()); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, "DELETE FROM retrieval_unit_blocks WHERE unit_id=?", unit.ID); err != nil {
						return err
					}
					blockIDs := unit.BlockIDs
					if len(blockIDs) == 0 {
						blockIDs = []string{unit.ID}
					}
					for offset, blockID := range blockIDs {
						if _, err := tx.ExecContext(ctx, "INSERT INTO retrieval_unit_blocks(unit_id,block_id,position) VALUES(?,?,?)", unit.ID, blockID, unit.Position+offset); err != nil {
							return err
						}
					}
					if _, err := tx.ExecContext(ctx, "DELETE FROM keyword_index WHERE unit_id=?", unit.ID); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, "INSERT INTO keyword_index(unit_id,document_id,text) VALUES(?,?,?)", unit.ID, id, textutil.KeywordText(unit.Text)); err != nil {
						return err
					}
					if changedIDs[unit.ID] {
						if _, err := tx.ExecContext(ctx, "DELETE FROM vectors WHERE unit_id=?", unit.ID); err != nil {
							return err
						}
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
		if len(changed) == 0 {
			progress(100)
			return nil
		}
		embedder, err := protocols.Embedder(model)
		if err != nil {
			return err
		}
		release, err := protocols.Acquire(ctx, model.Channel)
		if err != nil {
			return err
		}
		texts := make([]string, len(changed))
		for index := range changed {
			texts[index] = changed[index].Text
		}
		vectors, usage, err := embedder.Embed(ctx, model.ID, texts)
		release()
		if err != nil {
			status := "error"
			if usage.InputTokens == 0 && usage.OutputTokens == 0 {
				for _, text := range texts {
					usage.InputTokens += int64(len(text) + 64)
				}
				status = "estimated_error"
			}
			_ = (usageWriter{db: app.DB}).WriteUsage(context.WithoutCancel(ctx), "embed", model, usage, llm.Cost(model, usage), status)
			return err
		}
		status := "success"
		if usage.InputTokens == 0 && usage.OutputTokens == 0 {
			for _, text := range texts {
				usage.InputTokens += int64(len(text) + 64)
			}
			status = "estimated_success"
		}
		if err := (usageWriter{db: app.DB}).WriteUsage(context.WithoutCancel(ctx), "embed", model, usage, llm.Cost(model, usage), status); err != nil {
			return err
		}
		if len(vectors) != len(changed) {
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
					hash := textutil.Hash([]byte(changed[index].Text))
					if _, err := tx.ExecContext(ctx, "INSERT INTO vectors(unit_id,dimensions,vector,model,content_hash) VALUES(?,?,?,?,?) ON CONFLICT(unit_id) DO UPDATE SET dimensions=excluded.dimensions,vector=excluded.vector,model=excluded.model,content_hash=excluded.content_hash", changed[index].ID, len(vector), encoded.Bytes(), model.ID, hash); err != nil {
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
		blocks, err := doc.Blocks(file.Document)
		if err != nil {
			return err
		}
		segments := []itranslate.Segment{{ID: itranslate.MetaTitleID, Text: file.Meta.Title, Heading: true}}
		hashes := map[string]string{}
		hashes[itranslate.MetaTitleID] = textutil.Hash([]byte(file.Meta.Title))
		if strings.TrimSpace(file.Meta.SEODescription) != "" {
			segments = append(segments, itranslate.Segment{ID: itranslate.MetaDescriptionID, Text: file.Meta.SEODescription})
			hashes[itranslate.MetaDescriptionID] = textutil.Hash([]byte(file.Meta.SEODescription))
		}
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
			taskTranslator := translator
			taskTranslator.Commit = func(commit func() error) error {
				return app.Content.WithPublished(id, file.Meta.Revision, func() error {
					if err := commit(); err != nil {
						return err
					}
					if _, err := app.History.Commit(ctx, "nice-wiki", "更新文档译文 "+language, []string{"i18n/" + language + "/" + id + ".json"}); err != nil {
						return err
					}
					return app.ClearAnswerCache(ctx)
				})
			}
			existing, err := app.Translations.ReadOptional(language, id)
			if err != nil {
				return err
			}
			if len(existing.Blocks) == 0 {
				var source strings.Builder
				for _, segment := range segments {
					source.WriteString(segment.Text)
					source.WriteByte('\n')
				}
				if err := itranslate.ExtractTerms(ctx, translationModel{client: modelsClient}, app.Glossary, id, language, source.String()); err != nil {
					return err
				}
				if _, err := app.History.Commit(ctx, "nice-wiki", "更新自动术语 "+language, []string{"i18n/glossary.json"}); err != nil {
					return err
				}
			}
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
			if _, err := app.History.Commit(ctx, "nice-wiki", "更新站点译文 "+language, []string{"i18n/" + language + "/_site.json", "i18n/" + language + "/_ui.json"}); err != nil {
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
	err = app.Jobs.Register(jobs.UpdateCatalog, func(ctx context.Context, _ string, progress func(int)) error {
		if err := app.Catalog.Refresh(ctx, http.DefaultClient, "https://models.dev/api.json?type=all"); err != nil {
			return err
		}
		progress(100)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = app.Jobs.Register(jobs.UpdateGeoIP, func(ctx context.Context, _ string, progress func(int)) error {
		target := app.Layout.GeoIPDatabase()
		if err := geoip.Download(ctx, http.DefaultClient, target, geoip.MonthlyURL(time.Now())); err != nil {
			return err
		}
		if err := app.GeoIP.Reload(target); err != nil {
			return err
		}
		progress(100)
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
