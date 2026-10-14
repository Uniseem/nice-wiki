package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nice-wiki/libs/platform"
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
