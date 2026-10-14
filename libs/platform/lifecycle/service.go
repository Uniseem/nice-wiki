package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nice-wiki/libs/platform"
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
	application, err := spec.BuildApplication(ctx, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "装配失败，errorCode=%s\n", errorCodeOf(err))
		return ExitFailure
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- application.Serve(ctx) }()
	restart := make(chan error, 1)
	go func() {
		if spec.RuntimeClient == nil {
			restart <- nil
			return
		}
		client, err := spec.RuntimeClient(config)
		if err != nil {
			restart <- err
			return
		}
		restart <- WatchRuntime(ctx, client, config.DataRoot)
	}()
	select {
	case err := <-serveDone:
		stopSignals()
		if err != nil {
			fmt.Fprintf(os.Stderr, "运行失败，errorCode=%s\n", errorCodeOf(err))
			_ = shutdown(application)
			return ExitFailure
		}
		// Serve提前返回视为异常退出，保留journal供排查。
		return ExitFailure
	case err := <-restart:
		if errors.Is(err, ErrRestartForRestore) {
			fmt.Fprint(os.Stderr, "恢复切换，安全退出并由进程守护拉起\n")
			if shutdownErr := shutdown(application); shutdownErr != nil {
				fmt.Fprintf(os.Stderr, "关闭失败，errorCode=%s\n", errorCodeOf(shutdownErr))
				return ExitFailure
			}
			return ExitForRestore
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "运行状态监视失败，errorCode=%s\n", errorCodeOf(err))
			_ = shutdown(application)
			return ExitFailure
		}
		// 无RuntimeClient时阻塞等待信号或Serve结束。
		if serveErr := <-serveDone; serveErr != nil {
			_ = shutdown(application)
			return ExitFailure
		}
		return ExitOK
	case <-ctx.Done():
		fmt.Fprint(os.Stderr, "收到停止信号，开始安全关闭\n")
		if err := shutdown(application); err != nil {
			fmt.Fprintf(os.Stderr, "关闭失败，errorCode=%s\n", errorCodeOf(err))
			return ExitFailure
		}
		return ExitOK
	}
}

// shutdown 以30秒上限安全关闭；超时或失败由调用方决定退出码。
func shutdown(application platform.Application) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	return application.Shutdown(ctx)
}

func errorCodeOf(err error) string {
	var libErr *platform.Error
	if errors.As(err, &libErr) {
		return libErr.Code
	}
	return platform.CodeInternal
}
