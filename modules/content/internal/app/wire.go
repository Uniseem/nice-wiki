// Package app 装配 content 域的启动依赖与运行权威客户端。
// 已实现的路由全部挂载，未实现的接口暂不注册。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/secretbox"
	"nice-wiki/libs/platform/serviceauth"
	content "nice-wiki/modules/content/internal"
	"nice-wiki/modules/content/internal/gen"
	"nice-wiki/modules/content/internal/transport"
)

// ModuleSlug 是本域固定的服务身份。
const ModuleSlug = "content"

// BuildApplication 按服务启动蓝图校验本域启动前置条件。
//
// 返回可运行的 Application：已实现的路由挂载，未实现的（回收站、回滚、预览、
// 屏障、快照）暂不注册；恢复 activation 与后台生命周期留待后续。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if err := ensureDataLayout(config.DataRoot); err != nil {
		return nil, err
	}
	anchors, err := loadTrustAnchors(config.TrustAnchorRef)
	if err != nil {
		return nil, err
	}

	runtimeClient, err := NewRuntimeClient(config)
	if err != nil {
		return nil, err
	}
	runtimeView, err := lifecycle.ReadRuntime(ctx, runtimeClient)
	if err != nil {
		return nil, err
	}

	store, err := content.OpenDatabase(ctx, config.DataRoot, runtimeView.DataEpoch)
	if err != nil {
		return nil, err
	}
	editLocks, err := content.NewEditLockManager(store)
	if err != nil {
		return nil, err
	}
	retain := false
	defer func() {
		if !retain {
			_ = editLocks.CloseEditLocks()
			_ = store.CloseDatabase()
		}
	}()
	if err := store.RecoverDatabaseJournal(ctx); err != nil {
		return nil, err
	}

	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	var identity *gen.IdentityClient
	if endpoint := config.Dependencies["identity"]; endpoint != "" {
		identity = gen.NewIdentityClient(endpoint, ModuleSlug, key)
	}
	referenceRuntime, err := content.NewReferenceRuntime(store, identity, key)
	if err != nil {
		return nil, err
	}
	window := platform.WriteWindowConfig{Runtime: func(work context.Context) (platform.RuntimeView, error) {
		return lifecycle.ReadRuntime(work, runtimeClient)
	}, WriteWindow: func(work context.Context) (time.Time, error) {
		current, err := lifecycle.ReadRuntime(work, runtimeClient)
		if err != nil {
			return time.Time{}, err
		}
		if current.Maintenance {
			return time.Time{}, platform.NewError(platform.CodeLocked, false, "本域维护写窗口关闭")
		}
		return time.Time{}, nil
	}}
	application := transport.NewApplication(config, runtimeClient, runtimeView.DataEpoch, true, true, true)
	if err = transport.RegisterReferenceRoutes(application, referenceRuntime, anchors); err != nil {
		return nil, err
	}
	treeRuntime, err := content.NewTreeRuntime(referenceRuntime, window)
	if err != nil {
		return nil, err
	}
	if endpoint := config.Dependencies["media"]; endpoint != "" {
		treeRuntime.Media = gen.NewMediaClient(endpoint, ModuleSlug, key)
	}
	if endpoint := config.Dependencies["model-gateway"]; endpoint != "" {
		if treeRuntime.Models, err = content.NewModelRuntime(gen.NewModelGatewayClient(endpoint, ModuleSlug, key), store); err != nil {
			return nil, err
		}
	}
	if err = transport.RegisterTreeRoutes(application, treeRuntime, anchors); err != nil {
		return nil, err
	}
	if err = transport.RegisterPublicationRoutes(application, treeRuntime, anchors); err != nil {
		return nil, err
	}
	var siteDataLoops []platform.BackgroundTask
	if identity != nil {
		drafts := content.DraftRuntime{Store: store, Manager: editLocks, Identity: identity, SiteConfig: gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key), Media: treeRuntime.Media, WriteWindow: window}
		locks := content.EditLockRuntime{Manager: editLocks, Identity: identity, WriteWindow: window}
		if err = transport.RegisterDraftRoutes(application, drafts, locks, treeRuntime, anchors); err != nil {
			return nil, err
		}
		siteData := content.SiteData{Drafts: drafts, Locks: locks, Tree: treeRuntime, Epoch: runtimeView.DataEpoch, AllowLocal: config.AllowLocalHTTP}
		if endpoint := config.Dependencies["media"]; endpoint != "" {
			siteData.Media = content.NewMediaGateway(endpoint, ModuleSlug, key)
		}
		if endpoint := config.Dependencies["identity"]; endpoint != "" {
			siteData.Drive = content.NewDriveGateway(endpoint, ModuleSlug, key)
		}
		if config.MasterKeyRef != "" {
			if box, boxErr := secretbox.Load(config.MasterKeyRef); boxErr == nil {
				siteData.Box = box
			}
		}
		if err = transport.RegisterSiteDataRoutes(application, siteData, identity, anchors); err != nil {
			return nil, err
		}
		siteDataLoops = []platform.BackgroundTask{
			{Name: "content.scheduled-backups", Run: func(ctx context.Context) error {
				return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: 60000, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(batch context.Context) error {
					siteData.RunScheduledBackups(batch, store.Sites(batch))
					return nil
				}})
			}},
			{Name: "content.git-auto-push", Run: func(ctx context.Context) error {
				return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: 30000, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(batch context.Context) error {
					siteData.RunGitAutoPush(batch, store.Sites(batch))
					return nil
				}})
			}},
		}
	}
	// 屏障与快照等路由尚未实现，请求落到 404；服务本身可启动。
	jobsClient := gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key)
	relay := content.RelayRuntime{Store: store, Jobs: jobsClient, WriteWindow: window}
	epoch := runtimeView.DataEpoch
	application.SetBackground(append(siteDataLoops, []platform.BackgroundTask{
		{Name: "content.relay-outbox", Run: func(ctx context.Context) error {
			return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: 1000, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(batch context.Context) error {
				// 发件箱不分站点：事件自己带着站点，转发用平台身份。
				_, err := content.RelayOutbox(content.WithRelayRuntime(batch, relay), content.ServiceOperation("system", epoch), content.RelayOutboxInput{BatchLimit: 100})
				return err
			}})
		}},
		{Name: "content.expire-idle-locks", Run: func(ctx context.Context) error {
			return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: 5000, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(batch context.Context) error {
				for _, site := range store.Sites(batch) {
					op := content.ServiceOperation(site, epoch)
					if _, err := content.ExpireIdleLocks(content.WithEditLockRuntime(platform.WithOperationContext(batch, op), content.EditLockRuntime{Manager: editLocks, Identity: identity, WriteWindow: window}), op, content.ExpireIdleLocksInput{Now: time.Now().UTC()}); err != nil {
						return err
					}
				}
				return nil
			}})
		}},
	}...))
	retain = true
	application.OnShutdown(store.CloseDatabase)
	application.OnShutdown(editLocks.CloseEditLocks)
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端；只保存外部引用，
// 不把密钥原文或服务端点写入日志。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

func validateConfig(config platform.ServiceConfig) error {
	if config.Module != ModuleSlug {
		return platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	if config.Listen == "" || config.DataRoot == "" || !filepath.IsAbs(config.DataRoot) {
		return platform.NewError(platform.CodeValidation, false, "本域监听或数据目录配置不合法")
	}
	if config.TrustAnchorRef == "" || config.SigningKeyRef == "" {
		return platform.NewError(platform.CodeValidation, false, "本域服务身份引用不完整")
	}
	if config.Dependencies == nil || config.Dependencies["site-config"] == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "content运行权威不可用")
	}
	return nil
}

func ensureDataLayout(dataRoot string) error {
	for _, name := range []string{"active", "control"} {
		path := filepath.Join(dataRoot, name)
		if err := lifecycle.ValidatePrivatePath(path, "directory", true); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
		if err := lifecycle.ValidatePrivatePath(path, "directory", false); err != nil {
			return err
		}
	}
	return nil
}

func loadTrustAnchors(ref string) (platform.TrustAnchors, error) {
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不可读")
	}
	var anchors platform.TrustAnchors
	if err := json.Unmarshal(raw, &anchors); err != nil || len(anchors.Services) == 0 {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不合法")
	}
	return anchors, nil
}

func loadSigningKey(ref string) (platform.SigningKey, error) {
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不可读")
	}
	var key platform.SigningKey
	if err := json.Unmarshal(raw, &key); err != nil || key.KeyID == "" || !filepath.IsAbs(key.PrivateKeyRef) {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不合法")
	}
	if _, err := serviceauth.LoadSigningKey(key); err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名私钥不可用")
	}
	return key, nil
}
