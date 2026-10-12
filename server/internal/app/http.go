package app

import (
	"crypto/rand"
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
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	var turnstile *guard.Turnstile
	if a.Secrets.Has("turnstile") {
		_ = a.Secrets.Use("turnstile", func(value string) error {
			turnstile = &guard.Turnstile{Secret: value, Client: http.DefaultClient}
			return nil
		})
	}
	apiServer := &api.Server{Preview: public.Preview, Assets: a.Assets, Auth: a.Auth, Content: a.Content, History: a.History, Settings: a.Settings, Runtime: a.Runtime, Secrets: a.Secrets, Channels: a.Channels, Setup: a.Setup, Logger: a.Logger, Dev: a.Config.Dev, Videos: videoembed.NewRegistry(http.DefaultClient), QA: a.QA, Conversations: a.Conversations, Records: a.Records, Bans: a.Bans, GuardEvents: a.GuardEvents, Debug: a.Debug, AnswerCache: a.AnswerCache, Memory: a.Memory, MemoryMapping: a.MemoryMapping, MemoryAssistant: a.MemoryAssistant, Catalog: a.Catalog, Translations: a.Translations, Glossary: a.Glossary, Locale: a.Locale, Jobs: a.Jobs, Sessions: guard.NewSessions(secret), PoW: guard.NewPoW(secret), Turnstile: turnstile, CreateBackupFile: a.CreateBackup, ListBackupFiles: a.ListBackups, RecordVisit: a.RecordVisit, StatsSummary: a.StatsSummary, ListQARecordRows: a.ListQARecords, SaveQAFeedback: a.SaveQAFeedback, FeatureQAAnswer: a.FeatureQAAnswer, UnfeatureQAAnswer: a.UnfeatureQAAnswer}
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
