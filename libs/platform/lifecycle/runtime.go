package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// 运行状态读取与监视参数。
const (
	readRuntimeBudget = 2 * time.Second
	runtimeCacheTTL   = time.Second
	watchInterval     = 5 * time.Second
)

// ErrRestartForRestore 运行中发现恢复切换；调用方以30秒上限Shutdown
// 并以退出码75退出，由进程守护重启后换入目录。
var ErrRestartForRestore = errors.New("恢复切换请求安全退出")

type cacheEntry struct {
	view   platform.RuntimeView
	readAt time.Time
}

var runtimeCache sync.Map // SiteConfigClient指针 -> cacheEntry

// ReadRuntime 读取site-config权威运行状态；结果最多缓存1秒，
// 读取截止≤2秒；site-config不可达返回DEPENDENCY_UNAVAILABLE。
func ReadRuntime(ctx context.Context, client platform.SiteConfigClient) (platform.RuntimeView, error) {
	if client == nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeValidation, false, "site-config客户端缺失")
	}
	if cached, ok := runtimeCache.Load(client); ok {
		entry := cached.(cacheEntry)
		if time.Since(entry.readAt) < runtimeCacheTTL {
			return entry.view, nil
		}
	}
	readCtx, cancel := context.WithTimeout(ctx, readRuntimeBudget)
	defer cancel()
	view, err := client.ReadRuntimeState(readCtx)
	if err != nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "site-config不可达")
	}
	if view.DataEpoch == "" || view.Phase == "" {
		return platform.RuntimeView{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态不完整")
	}
	runtimeCache.Store(client, cacheEntry{view: view, readAt: time.Now()})
	return view, nil
}

// WatchRuntime 每5秒读取运行状态；发现恢复切换或整体换回且本域
// 存在对应staged或已换入目录时返回ErrRestartForRestore。
// site-config短时不可达继续轮询，不触发退出。
func WatchRuntime(ctx context.Context, client platform.SiteConfigClient, dataRoot string) error {
	if dataRoot == "" {
		return platform.NewError(platform.CodeValidation, false, "数据根缺失")
	}
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		view, err := ReadRuntime(ctx, client)
		if err == nil && needsRestartForRestore(view, dataRoot) {
			return ErrRestartForRestore
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// needsRestartForRestore 判断当前阶段与本域目录是否要求重启换入。
func needsRestartForRestore(view platform.RuntimeView, dataRoot string) bool {
	if view.RestoreID == "" {
		return false
	}
	restoreDir := filepath.Join(dataRoot, "restore", view.RestoreID)
	switch view.Phase {
	case platform.PhaseRestoreSwitching:
		if dirExists(filepath.Join(restoreDir, "staged")) {
			return true
		}
		// 已换入（重启后再次轮询）：换入标记存在即视为完成，不再要求退出。
		return dirExists(filepath.Join(restoreDir, "switched"))
	case platform.PhaseRestoreRollingBack:
		return dirExists(filepath.Join(restoreDir, "before"))
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// runtimeStatePath 是 site-config:readRuntimeState 的接口路径。
const runtimeStatePath = "/v1/runtime-state"

// maxRuntimeStateBytes 限制运行状态响应体，RuntimeState 远小于此值。
const maxRuntimeStateBytes = 1 << 20

// NewSiteConfigClient 构造各域共用的 site-config 运行状态客户端：以本服务
// 断言调用 GET /v1/runtime-state，并解开响应 {"state": RuntimeState} 外层；
// RuntimeState 的 dataEpoch/phase/restoreId/targetDataEpoch/maintenance 与
// RuntimeView 同名。各域 NewRuntimeClient 只装配参数，不再自写适配。
func NewSiteConfigClient(cfg platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.Dependencies["site-config"]), "/")
	parsed, err := url.Parse(base)
	if base == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return &siteConfigClient{
		endpoint: base + runtimeStatePath,
		http:     &http.Client{Transport: serviceauth.SigningTransport(cfg.Module, "site-config", key, nil)},
	}, nil
}

type siteConfigClient struct {
	endpoint string
	http     *http.Client
}

func (c *siteConfigClient) ReadRuntimeState(ctx context.Context) (platform.RuntimeView, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeInternal, false, "运行状态请求构造失败")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "site-config不可达")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRuntimeStateBytes))
	if err != nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取运行状态失败")
	}
	if resp.StatusCode >= 300 {
		var problem platform.Problem
		_ = json.Unmarshal(raw, &problem)
		code := problem.Code
		if _, ok := platform.ProblemHTTPStatus[code]; !ok {
			code = platform.CodeDependencyUnavailable
		}
		return platform.RuntimeView{}, platform.NewError(code, problem.Retryable || code == platform.CodeDependencyUnavailable, "%s", problem.Detail)
	}
	var response struct {
		State platform.RuntimeView `json:"state"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行状态响应不合法")
	}
	return response.State, nil
}
