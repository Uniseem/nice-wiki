package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/audit"
	"nice-wiki/libs/platform/pgdb"
)

// 退出码语义：0正常、1失败、2配置错误、75恢复重启。
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitBadConfig   = 2
	ExitForRestore  = 75
	shutdownGrace   = 30 * time.Second
	configErrFormat = "配置错误：输出errorCode后退出\n"
)

// RunService 是各服务main的通用进程模板：
// 加载配置（错误返回2）→注册SIGINT/SIGTERM→BuildApplication
// （其中先ApplyRestoreSwap）→运行Serve与WatchRuntime；收到信号或
// 恢复切换时以30秒上限Shutdown；不能安全关闭返回1并保留journal。
func RunService(spec platform.ServiceSpec) int {
	if spec.BuildApplication == nil {
		fmt.Fprint(os.Stderr, "装配回调缺失\n")
		return ExitFailure
	}
	config, err := LoadServiceConfig(os.Args[1:], os.Environ())
	if err != nil {
		fmt.Fprint(os.Stderr, configErrFormat)
		return ExitBadConfig
	}
	ctx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	return runComponent(ctx, stopSignals, spec, config)
}

// RunServices 是宿主进程模板：在同一进程内运行多个组件（每个组件仍是一个独立装配的 Application，
// 各自监听、各自配置、各自服务身份）。参数只接受 --config-dir，组件配置取 <目录>/<模块>.json；
// 不读环境变量覆盖，避免各组件互相覆盖监听地址。任一组件失败则整体停止，由进程守护重启。
func RunServices(args []string, specs []platform.ServiceSpec) int {
	dir := ""
	for i := 0; i < len(args); i++ {
		name, value, hasValue := splitFlag(args[i])
		if name != "--config-dir" {
			fmt.Fprint(os.Stderr, configErrFormat)
			return ExitBadConfig
		}
		if !hasValue {
			i++
			if i >= len(args) {
				fmt.Fprint(os.Stderr, configErrFormat)
				return ExitBadConfig
			}
			value = args[i]
		}
		dir = value
	}
	if dir == "" || len(specs) == 0 {
		fmt.Fprint(os.Stderr, configErrFormat)
		return ExitBadConfig
	}
	configs := make([]platform.ServiceConfig, len(specs))
	for i, spec := range specs {
		if spec.BuildApplication == nil {
			fmt.Fprint(os.Stderr, "装配回调缺失\n")
			return ExitFailure
		}
		config, err := LoadServiceConfig([]string{"--config", filepath.Join(dir, spec.Module+".json")}, sharedEnv(os.Environ()))
		if err != nil || config.Module != spec.Module {
			fmt.Fprintf(os.Stderr, "组件配置错误：%s\n", spec.Module)
			return ExitBadConfig
		}
		configs[i] = config
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	codes := make(chan int, len(specs))
	for i := range specs {
		go func(spec platform.ServiceSpec, config platform.ServiceConfig) {
			codes <- runComponent(ctx, stop, spec, config)
		}(specs[i], configs[i])
	}
	worst := ExitOK
	for range specs {
		code := <-codes
		if code != ExitOK {
			stop()
			if worst == ExitOK || code == ExitFailure {
				worst = code
			}
		}
	}
	return worst
}

// sharedEnv 是进程里所有组件共用的环境变量：只放公开地址、本地 HTTP 开关、可信代理和依赖地址；
// 模块名、监听、数据目录、密钥引用这类每个组件各不相同的项只能写在各自的配置文件里。
func sharedEnv(env []string) []string {
	var out []string
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		switch {
		case name == "NW_PUBLIC_BASE_URL", name == "NW_ALLOW_LOCAL_HTTP", name == "NW_TRUSTED_PROXY_CIDRS", strings.HasPrefix(name, "NW_DEP_"):
			out = append(out, entry)
		}
	}
	return out
}

// runComponent 运行一个已加载配置的组件直到退出；stopSignals 在需要整体停止时调用。
func runComponent(ctx context.Context, stopSignals context.CancelFunc, spec platform.ServiceSpec, config platform.ServiceConfig) int {
	if _, key, err := LoadServiceIdentity(config); err == nil {
		audit.Install(config, key)
	}
	if dsn, err := pgdb.ReadDSN(config.DatabaseRef); err != nil {
		fmt.Fprintf(os.Stderr, "数据库连接串不可读 %s\n", config.Module)
		return ExitBadConfig
	} else if dsn != "" {
		pgdb.Configure(config.Module, config.DataRoot, dsn)
	}
	application, err := spec.BuildApplication(ctx, config)
	// 依赖还没就绪（站点未初始化、依赖服务未启动）时每 3 秒重试，不必靠进程守护反复重启整个进程。
	for err != nil && retryableStartup(err) && ctx.Err() == nil {
		time.Sleep(3 * time.Second)
		application, err = spec.BuildApplication(ctx, config)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "装配失败 %s，errorCode=%s %s\n", config.Module, errorCodeOf(err), messageOf(err))
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "原始错误：%v\n", err)
		}
		return ExitFailure
	}
	hosted, hasBackground := application.(platform.BackgroundApplication)
	var tasks []platform.BackgroundTask
	if hasBackground {
		tasks = hosted.BackgroundTasks()
	}
	names := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if !platform.ValidIdentifier(task.Name) || names[task.Name] || task.Run == nil {
			fmt.Fprint(os.Stderr, "后台入口配置不合法\n")
			_ = shutdown(application)
			return ExitFailure
		}
		names[task.Name] = true
	}
	var background sync.WaitGroup
	backgroundDone := make(chan error, len(tasks))
	for _, task := range tasks {
		background.Add(1)
		go func(task platform.BackgroundTask) {
			defer background.Done()
			result := task.Run(ctx)
			if ctx.Err() == nil {
				backgroundDone <- result
			}
		}(task)
	}
	// 共享一个30秒关闭预算；后台仍可能写库时不先关闭业务句柄。
	finish := func(code int) int {
		stopSignals()
		closeCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if hasBackground {
			if err := hosted.Drain(closeCtx); err != nil {
				fmt.Fprintf(os.Stderr, "停止入口失败，errorCode=%s\n", errorCodeOf(err))
				return ExitFailure
			}
		}
		stopped := make(chan struct{})
		go func() { background.Wait(); close(stopped) }()
		select {
		case <-stopped:
		case <-closeCtx.Done():
			fmt.Fprint(os.Stderr, "后台任务未在关闭期限内停止，保留恢复决定\n")
			return ExitFailure
		}
		if err := application.Shutdown(closeCtx); err != nil {
			fmt.Fprintf(os.Stderr, "关闭失败，errorCode=%s\n", errorCodeOf(err))
			return ExitFailure
		}
		return code
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- application.Serve(ctx) }()
	restart := make(chan error, 1)
	if spec.RuntimeClient != nil {
		go func() {
			client, err := spec.RuntimeClient(config)
			if err != nil {
				restart <- err
				return
			}
			restart <- WatchRuntime(ctx, client, config.DataRoot)
		}()
	}
	select {
	case err := <-serveDone:
		if ctx.Err() != nil {
			return finish(ExitOK)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "运行失败，errorCode=%s %s\n", errorCodeOf(err), messageOf(err))
		}
		return finish(ExitFailure)
	case err := <-restart:
		if ctx.Err() != nil {
			return finish(ExitOK)
		}
		if errors.Is(err, ErrRestartForRestore) {
			fmt.Fprint(os.Stderr, "恢复切换，安全退出并由进程守护拉起\n")
			return finish(ExitForRestore)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "运行状态监视失败，errorCode=%s\n", errorCodeOf(err))
		}
		return finish(ExitFailure)
	case err := <-backgroundDone:
		if ctx.Err() != nil {
			return finish(ExitOK)
		}
		if errors.Is(err, ErrRestartForRestore) {
			return finish(ExitForRestore)
		}
		fmt.Fprintf(os.Stderr, "后台入口提前结束，errorCode=%s %s\n", errorCodeOf(err), messageOf(err))
		return finish(ExitFailure)
	case <-ctx.Done():
		fmt.Fprint(os.Stderr, "收到停止信号，开始安全关闭\n")
		return finish(ExitOK)
	}
}

// retryableStartup 判断装配失败是否只是依赖尚未就绪：站点未初始化，或依赖服务暂不可用。
func retryableStartup(err error) bool {
	switch errorCodeOf(err) {
	case platform.CodeDependencyUnavailable:
		return true
	case platform.CodeNotFound:
		return strings.Contains(messageOf(err), "尚未完成初始化")
	}
	return false
}

// shutdown 以30秒上限安全关闭；超时或失败由调用方决定退出码。
func shutdown(application platform.Application) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	return application.Shutdown(ctx)
}

// messageOf 只取登记错误里的固定中文说明，不含密钥或请求内容。
func messageOf(err error) string {
	var libErr *platform.Error
	if errors.As(err, &libErr) {
		return libErr.Message
	}
	return ""
}

func errorCodeOf(err error) string {
	var libErr *platform.Error
	if errors.As(err, &libErr) {
		return libErr.Code
	}
	return platform.CodeInternal
}
