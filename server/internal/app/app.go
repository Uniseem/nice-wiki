package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"nice-wiki/internal/assets"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/config"
	"nice-wiki/internal/content"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/history"
	"nice-wiki/internal/i18n"
	"nice-wiki/internal/jobs"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/setup"
	"nice-wiki/internal/store"
	"nice-wiki/internal/store/sqlc"
	"time"
)

type App struct {
	Assets   *assets.Store
	Config   config.Bootstrap
	Logger   *slog.Logger
	Layout   datadir.Layout
	Lock     *datadir.Lock
	DB       *sql.DB
	Auth     *auth.Service
	Settings *settings.Store
	Runtime  *config.RuntimeStore
	Content  *content.Repo
	Channels *llm.Channels
	Setup    *setup.Service
	Jobs     *jobs.Service
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
			err = errors.Join(err, lock.Close())
		}
	}()
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
	app.Settings, err = settings.Open(layout, nil, i18n.Known)
	if err != nil {
		return nil, err
	}
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
	app.Content, err = content.Open(ctx, layout, app.DB, versions, nil)
	if err != nil {
		return nil, err
	}
	app.Assets = assets.Open(layout, app.DB, assetSources{content: app.Content, settings: app.Settings}, logger)
	app.Setup = &setup.Service{Auth: app.Auth, Settings: app.Settings, Channels: app.Channels}
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
	if err = app.Auth.CheckReset(ctx, layout.ResetSuperadmin()); err != nil {
		return nil, err
	}
	return app, nil
}

// 数据库能力由组装层注入，jobs 保持既定依赖方向。
type mergeStore struct{ db *sql.DB }

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
	return sqlc.New(s.db).ClearQueuedGeneration(ctx, sqlc.ClearQueuedGenerationParams{Kind: string(kind), SubjectID: subject})
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
