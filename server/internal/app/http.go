package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/api"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/site"
	"nice-wiki/internal/videoembed"
	"nice-wiki/internal/webui"
)

func qaSigningSecret(secrets interface {
	Has(string) bool
	Set(string, string) error
	Use(string, func(string) error) error
}) ([]byte, error) {
	const key = "qa-session-signing"
	if !secrets.Has(key) {
		value := make([]byte, 32)
		if _, err := rand.Read(value); err != nil {
			return nil, err
		}
		if err := secrets.Set(key, hex.EncodeToString(value)); err != nil {
			return nil, err
		}
	}
	var value []byte
	err := secrets.Use(key, func(encoded string) error {
		decoded, err := hex.DecodeString(encoded)
		if err != nil || len(decoded) != 32 {
			return errors.New("问答会话签名密钥无效")
		}
		value = decoded
		return nil
	})
	return value, err
}

func (a *App) Handler() (http.Handler, error) {
	files, err := webui.Files()
	if err != nil {
		return nil, err
	}
	manifest, err := webui.LoadManifest(files)
	if err != nil && !a.Config.Dev {
		return nil, errors.New("前端尚未构建，请先运行 make build")
	}
	public, err := site.New(a.Content, a.Settings, manifest, a.Config.Dev)
	if err != nil {
		return nil, err
	}
	public.Translations = a.Translations
	public.Runtime = a.Runtime
	public.RecordCrawler = a.RecordCrawler
	secret, err := qaSigningSecret(a.Secrets)
	if err != nil {
		return nil, err
	}
	powSecret := make([]byte, 32)
	if _, err := rand.Read(powSecret); err != nil {
		return nil, err
	}
	var turnstile *guard.Turnstile
	if a.Secrets.Has("turnstile") {
		_ = a.Secrets.Use("turnstile", func(value string) error {
			turnstile = &guard.Turnstile{Secret: value, Client: http.DefaultClient}
			return nil
		})
	}
	checks := guard.NewChecks()
	apiServer := &api.Server{Preview: public.Preview, Assets: a.Assets, Auth: a.Auth, Content: a.Content, History: a.History, Settings: a.Settings, Runtime: a.Runtime, Secrets: a.Secrets, Channels: a.Channels, Setup: a.Setup, Logger: a.Logger, Dev: a.Config.Dev, Videos: videoembed.NewRegistry(http.DefaultClient), QA: a.QA, Conversations: a.Conversations, Records: a.Records, Bans: a.Bans, Checks: checks, GuardEvents: a.GuardEvents, GuardScope: func(value string) string { return guard.IPScope(powSecret, value) }, Debug: a.Debug, AnswerCache: a.AnswerCache, Memory: a.Memory, MemoryMapping: a.MemoryMapping, MemoryAssistant: a.MemoryAssistant, Catalog: a.Catalog, Translations: a.Translations, Glossary: a.Glossary, Locale: a.Locale, LocaleLimiter: guard.NewLimiter(10), CountryLookup: a.GeoIP.Lookup, Jobs: a.Jobs, Sessions: guard.NewSessions(secret), PoW: guard.NewPoW(powSecret), Turnstile: turnstile, CreateBackupFile: a.CreateBackup, ListBackupFiles: a.ListBackups, OpenBackupFile: a.OpenBackup, RecordVisit: a.RecordVisit, StatsSummary: a.StatsSummary, ListQARecordRows: a.ListQARecords, LoadQAConversation: a.LoadQAConversation, ClearQAConversation: a.ClearQAConversation, SaveQAFeedback: a.SaveQAFeedback, FeatureQAAnswer: a.FeatureQAAnswer, UnfeatureQAAnswer: a.UnfeatureQAAnswer, TranslationEstimate: a.TranslationEstimate, ClearAnswerCacheData: a.ClearAnswerCache}
	mux := http.NewServeMux()
	apiHandler, err := apiServer.Handler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/api/", apiHandler)
	mux.Handle("/healthz", apiHandler)
	mux.Handle("/admin/", webui.Admin(files, a.Config.Dev))
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusPermanentRedirect)
	})
	// 静态资源仅开放构建的 assets 目录，禁止读取 manifest 和后台 HTML。
	if assets, err := fs.Sub(files, "assets"); err == nil {
		static := http.StripPrefix("/static/assets/", http.FileServerFS(assets))
		mux.Handle("/static/assets/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			static.ServeHTTP(w, r)
		}))
	}
	mux.Handle("/assets/", a.Assets.Handler())
	mux.Handle("/", public.Handler())
	return httpx.Logging(a.Logger, httpx.Recover(a.Logger, httpx.Security(a.Config.Dev, httpx.RealIP(func() []string { return a.Runtime.Get().TrustedProxies }, mux)))), nil
}
