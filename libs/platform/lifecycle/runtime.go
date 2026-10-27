package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

type cacheEntry struct {
	view   platform.RuntimeView
	readAt time.Time
}

var runtimeCache sync.Map // SiteConfigClient指针 -> cacheEntry

// NewWriteContext 为已校验的后台配置建立一个最多30秒的当前安全写窗口。
// 每次外部IO后的本域确认仍由调用方重新取窗口，不能复用旧权威快照。
func NewWriteContext(ctx context.Context, config platform.WriteWindowConfig) (context.Context, platform.RuntimeView, context.CancelFunc, error) {
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	if op, ok := platform.OperationContextFrom(work); ok {
		deadline, _ := work.Deadline()
		op.Deadline = deadline.UTC().Format(time.RFC3339Nano)
		work = platform.WithOperationContext(work, op)
	}
	view, err := config.Runtime(work)
	if err != nil {
		cancel()
		return nil, view, nil, err
	}
	deadline, err := config.WriteWindow(work)
	if err != nil {
		cancel()
		return nil, view, nil, err
	}
	if view.DataEpoch == "" {
		cancel()
		return nil, view, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "运行权威数据代次未知")
	}
	if view.Phase != platform.PhaseRunning {
		cancel()
		return nil, view, nil, platform.NewError(platform.CodeLocked, true, "当前运行阶段不接受后台写入")
	}
	if (view.Maintenance && deadline.IsZero()) || (!deadline.IsZero() && !deadline.After(time.Now())) {
		cancel()
		return nil, view, nil, platform.NewError(platform.CodeLocked, true, "后台维护安全窗口已关闭")
	}
	if deadline.IsZero() {
		return work, view, cancel, nil
	}
	bounded, release := context.WithDeadline(work, deadline)
	if op, ok := platform.OperationContextFrom(bounded); ok {
		effective, _ := bounded.Deadline()
		op.Deadline = effective.UTC().Format(time.RFC3339Nano)
		bounded = platform.WithOperationContext(bounded, op)
	}
	return bounded, view, func() { release(); cancel() }, nil
}

// RunRecoveryLoop 串行执行本域恢复批次；未知权威暂停，永久错误交还进程装配。
func RunRecoveryLoop(ctx context.Context, config platform.RecoveryLoopConfig) error {
	if config.AdmitBatch == nil {
		config.AdmitBatch = func(ctx context.Context) (context.Context, func(), error) { return ctx, func() {}, nil }
	}
	if config.PollMillis < 100 || config.PollMillis > 60000 || config.Runtime == nil || config.WriteWindow == nil || config.Recover == nil {
		return platform.NewError(platform.CodeValidation, false, "恢复循环参数未装配")
	}
	window := platform.WriteWindowConfig{Runtime: config.Runtime, WriteWindow: config.WriteWindow}
	for {
		if ctx.Err() != nil {
			return nil
		}
		scoped, release, admitErr := config.AdmitBatch(ctx)
		var err error
		if admitErr != nil {
			err = admitErr
		} else {
			work, _, cancel, windowErr := NewWriteContext(scoped, window)
			err = windowErr
			if err == nil {
				err = config.Recover(work)
				cancel()
			}
			release()
		}
		if ctx.Err() != nil {
			return nil
		}
		// 数据代次变化要重启进程；其他错误只影响这一批，记下来下一轮再试，
		// 不让一条处理不了的记录（或一次被拒的调用）拖垮整个服务。
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeVersionMismatch {
			return err
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "恢复循环批次失败，稍后重试：%v\n", err)
		}
		timer := time.NewTimer(time.Duration(config.PollMillis) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

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
	if view.DataEpoch == "" || view.Phase == "" || view.Revision < 1 {
		return platform.RuntimeView{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态不完整")
	}
	runtimeCache.Store(client, cacheEntry{view: view, readAt: time.Now()})
	return view, nil
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
	// 调用链上的操作上下文可能带着早已过期的期限（长期循环沿用旧上下文）；运行状态读取自己给 10 秒。
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if op, ok := platform.OperationContextFrom(ctx); ok {
		deadline, _ := ctx.Deadline()
		op.Deadline = deadline.UTC().Format(time.RFC3339Nano)
		ctx = platform.WithOperationContext(ctx, op)
	}
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
