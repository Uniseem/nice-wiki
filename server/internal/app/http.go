package app

import (
	"errors"
	"io/fs"
	"net/http"
	"nice-wiki/internal/api"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/site"
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
	apiServer := &api.Server{Preview: public.Preview, Assets: a.Assets, Auth: a.Auth, Content: a.Content, Settings: a.Settings, Runtime: a.Runtime, Channels: a.Channels, Setup: a.Setup, Logger: a.Logger, Dev: a.Config.Dev}
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
		mux.Handle("/static/assets/", http.StripPrefix("/static/assets/", http.FileServerFS(assets)))
	}
	mux.Handle("/assets/", a.Assets.Handler())
	mux.Handle("/", public.Handler())
	return httpx.Logging(a.Logger, httpx.Recover(a.Logger, httpx.Security(a.Config.Dev, httpx.RealIP(func() []string { return a.Runtime.Get().TrustedProxies }, mux)))), nil
}
