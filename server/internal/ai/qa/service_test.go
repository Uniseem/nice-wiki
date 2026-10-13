package qa

import (
	"context"
	"encoding/json"
	"errors"
	"nice-wiki/internal/ai/cache"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/aitask"
	"nice-wiki/internal/content"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeChat struct {
	calls    []protocol.Request
	complete func(protocol.Request) (protocol.Response, error)
}

func (f *fakeChat) Complete(_ context.Context, r protocol.Request) (protocol.Response, error) {
	f.calls = append(f.calls, r)
	if f.complete != nil {
		return f.complete(r)
	}
	return protocol.Response{Text: "回答", Usage: protocol.Usage{InputTokens: 10, OutputTokens: 5}}, nil
}
func (f *fakeChat) Stream(context.Context, protocol.Request, func(protocol.Chunk) error) error {
	return errors.New("不使用流式模型")
}

type fakeProtocols struct{ chat *fakeChat }

func (f fakeProtocols) Chat(llm.Model) (protocol.Chat, error) { return f.chat, nil }

type fakeRetriever struct{ queries []string }

func (f *fakeRetriever) Retrieve(_ context.Context, q string) ([]retrieve.Unit, error) {
	f.queries = append(f.queries, q)
	return []retrieve.Unit{{ID: "block", DocumentID: "document", Text: "材料", Heading: "标题"}}, nil
}
func serviceFixture(t *testing.T) (*Service, *fakeChat, *fakeRetriever) {
	t.Helper()
	tiers := llm.NewTiers()
	model := llm.Model{ID: "model", Channel: "channel", InputPrice: 1, OutputPrice: 10}
	for _, tier := range []aitask.Tier{aitask.Fast, aitask.Smart} {
		if err := tiers.Set(tier, []llm.Model{model}); err != nil {
			t.Fatal(err)
		}
	}
	chat := &fakeChat{}
	retriever := &fakeRetriever{}
	service := &Service{Models: llm.NewClient(tiers, fakeProtocols{chat}, nil), Retriever: retriever, Cache: cache.New(), Limiter: guard.NewLimiter(100), MaxQuestion: 2000, Conversations: NewConversations(time.Hour), Records: &Records{}, ResolveCitation: func(string) (string, string, bool) { return "标题", "page", true }, CurrentReference: func(string, string) bool { return true }}
	return service, chat, retriever
}
func TestAskCacheEventsAndRecord(t *testing.T) {
	service, chat, retriever := serviceFixture(t)
	ctx := context.Background()
	emit := func(Event) error { return nil }
	if err := service.Ask(ctx, "first", "问题", "en", "memory", "", emit); err != nil {
		t.Fatal(err)
	}
	events := []Event{}
	if err := service.Ask(ctx, "second", "问题", "en", "memory", "", func(event Event) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(chat.calls) != 1 || len(retriever.queries) != 1 || len(events) != 3 || events[0].Type != "text" || events[0].Text != "回答" || events[1].Type != "citations" || events[2].Type != "done" {
		t.Fatal(events)
	}
	if events[1].Citations[0].SourceHash == "" {
		t.Fatal("缓存引用未保留来源哈希")
	}
	conversation, _ := service.Conversations.Get("second")
	if len(conversation.Turns) != 2 || conversation.Turns[1].Role != "assistant" || len(service.Records.List()) != 2 {
		t.Fatal(conversation)
	}
	encoded, err := json.Marshal(events[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"blockId":"block"`) || strings.Contains(string(encoded), "BlockID") {
		t.Fatal(string(encoded))
	}
	service.CurrentReference = func(string, string) bool { return false }
	if err := service.Ask(ctx, "third", "问题", "en", "memory", "", emit); err == nil {
		t.Fatal("接受已下线材料")
	}
	if len(retriever.queries) != 2 {
		t.Fatal("使用了失效缓存")
	}
}
func TestAskContextAndLanguage(t *testing.T) {
	service, chat, retriever := serviceFixture(t)
	service.BaseLanguage = func() string { return "zh-Hans" }
	service.TranslatedText = func(string, string, string) (string, bool) { return "Target material", true }
	service.Conversations.ReplaceSummary("session", "产品 A", []Turn{{Role: "assistant", Text: "先前回答"}})
	chat.complete = func(r protocol.Request) (protocol.Response, error) {
		if len(r.Messages) == 2 && strings.Contains(r.Messages[0].Content, "独立完整") {
			return protocol.Response{Text: `{"question":"产品 A 怎么安装"}`}, nil
		}
		return protocol.Response{Text: "Install A"}, nil
	}
	service.Cache.Put(cache.Entry{Question: "它怎么安装", Answer: "错误共享追问", Language: "en", Expires: time.Now().Add(time.Hour)})
	if err := service.Ask(context.Background(), "session", "它怎么安装", "en", "", "", func(Event) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(chat.calls) != 2 || retriever.queries[0] != "产品 A 怎么安装" || !strings.Contains(chat.calls[0].Messages[1].Content, "产品 A") || !strings.Contains(chat.calls[1].Messages[0].Content, "回答语言：en") || !strings.Contains(chat.calls[1].Messages[0].Content, "先前回答") || !strings.Contains(chat.calls[1].Messages[0].Content, "Target material") {
		t.Fatal(chat.calls)
	}
	records := service.Records.List()
	if records[0].Language != "en" || records[0].Citations[0].URL != "/en/page" {
		t.Fatal(records)
	}
}
func TestAskDisabledAndBudgetBeforeModel(t *testing.T) {
	service, chat, _ := serviceFixture(t)
	options := settings.DefaultAI()
	service.Settings = func() settings.AI { return options }
	if err := service.Ask(context.Background(), "disabled", "问题", "en", "", "", func(Event) error { return nil }); err == nil {
		t.Fatal("关闭后仍可问答")
	}
	if len(chat.calls) != 0 {
		t.Fatal("关闭后调用模型")
	}
	options.Enabled = true
	service.Budget = guard.NewBudget(.00001, 0)
	if err := service.Ask(context.Background(), "budget", "问题", "en", "", "", func(Event) error { return nil }); err == nil {
		t.Fatal("超限未拒绝")
	}
	if len(chat.calls) != 0 {
		t.Fatal("超限先调用了模型")
	}
	// 小额真实用量释放预留，后续请求能够继续执行。
	service.Budget = guard.NewBudget(.05, 0)
	for _, session := range []string{"one", "two"} {
		if err := service.Ask(context.Background(), session, session, "en", "", "", func(Event) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if len(chat.calls) != 2 {
		t.Fatal(chat.calls)
	}
}
func TestRuntimeModelsFollowCatalog(t *testing.T) {
	catalog, err := llm.OpenCatalog(filepath.Join(t.TempDir(), "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	tiers := llm.NewTiers()
	tiers.Source = func(aitask.Tier) []llm.Model { return catalog.List() }
	if _, err := tiers.Resolve("qa-answer"); err == nil {
		t.Fatal("空目录错误")
	}
	for _, id := range []string{"z", "a"} {
		if err := catalog.Merge([]llm.Model{{ID: id, Channel: "channel", Capabilities: []llm.Capability{llm.Chat}}}); err != nil {
			t.Fatal(err)
		}
	}
	models, err := tiers.Resolve("qa-answer")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "a" {
		t.Fatal(models)
	}
	if err := catalog.Merge([]llm.Model{{ID: "a", Channel: "channel", InputPrice: 3}}); err != nil {
		t.Fatal(err)
	}
	models, err = tiers.Resolve("qa-answer")
	if err != nil {
		t.Fatal(err)
	}
	if models[0].InputPrice != 3 {
		t.Fatal("模型修改需重启")
	}
}
func TestWithdrawRemovesRetrievalAndBlocksStaleCommit(t *testing.T) {
	ctx := context.Background()
	layout, _ := datadir.New(t.TempDir())
	if err := layout.Initialize(); err != nil {
		t.Fatal(err)
	}
	file := content.File{Meta: content.Meta{ID: "document", Title: "标题", Slug: "page", SortKey: "a0", Published: true, Revision: 1}, Document: doc.Document{SchemaVersion: doc.SchemaVersion, Content: doc.Node{Type: "doc", Content: []doc.Node{{Type: "paragraph", Attrs: map[string]any{"id": "block"}, Content: []doc.Node{{Type: "text", Text: "材料"}}}}}}}
	path, err := layout.Document(nil, "document")
	if err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteJSON(path, file); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, layout.Database())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo, err := content.Open(ctx, layout, db, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"INSERT INTO retrieval_units(id,document_id,kind,text,content_hash,updated_at) VALUES('block','document','text','材料','hash',0)",
		"INSERT INTO keyword_index(unit_id,document_id,text) VALUES('block','document','材料')",
		"INSERT INTO vectors(unit_id,dimensions,vector,model,content_hash) VALUES('block',1,x'00000000','model','hash')",
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	answerCache := cache.New()
	answerCache.Put(cache.Entry{Question: "问题", Language: "en", Answer: "旧答案", Expires: time.Now().Add(time.Hour)})
	repo.SetChangedHook(answerCache.InvalidateAll)
	if _, err := repo.Withdraw(ctx, "document"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"retrieval_units", "keyword_index", "vectors"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal(table, count)
		}
	}
	if _, ok := answerCache.Get("问题", "en", "", nil); ok {
		t.Fatal("撤回未清缓存")
	}
	committed := false
	if err := repo.WithPublished("document", 1, func() error { committed = true; return nil }); !errors.Is(err, content.ErrConflict) || committed {
		t.Fatal("撤回后旧任务写回")
	}
}

type fakeUsageWriter struct {
	costs    []float64
	statuses []string
}

func (f *fakeUsageWriter) WriteUsage(_ context.Context, _ string, _ llm.Model, _ protocol.Usage, cost float64, status string) error {
	f.costs = append(f.costs, cost)
	f.statuses = append(f.statuses, status)
	return nil
}
func TestFailedModelKeepsBudgetAndUsage(t *testing.T) {
	service, chat, _ := serviceFixture(t)
	writer := &fakeUsageWriter{}
	service.Models.Usage = writer
	service.Budget = guard.NewBudget(.05, 0)
	chat.complete = func(protocol.Request) (protocol.Response, error) {
		return protocol.Response{}, errors.New("无法确认渠道用量")
	}
	for _, session := range []string{"first", "second"} {
		if err := service.Ask(context.Background(), session, "问题", "en", "", "", func(Event) error { return nil }); err == nil {
			t.Fatal("失败未返回错误")
		}
	}
	if len(chat.calls) != 1 || len(writer.costs) != 1 || writer.costs[0] < .04 || writer.statuses[0] != "estimated_error" {
		t.Fatalf("calls=%d usage=%+v", len(chat.calls), writer)
	}
}

func TestLongRequestRetainsSessionLock(t *testing.T) {
	limiter := guard.NewLimiter(100)
	now := time.Now()
	if err := limiter.Begin("session", now); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Begin("session", now.Add(2*time.Minute)); err == nil {
		t.Fatal("分钟窗口重置丢失请求锁")
	}
	limiter.End("session")
	if err := limiter.Begin("session", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	limiter.End("session")
}
