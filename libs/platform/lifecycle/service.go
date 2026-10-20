package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/audit"
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
	if _, key, err := LoadServiceIdentity(config); err == nil {
		audit.Install(config, key)
	}
	ctx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	application, err := spec.BuildApplication(ctx, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "装配失败，errorCode=%s %s\n", errorCodeOf(err), messageOf(err))
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
