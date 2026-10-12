package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"nice-wiki/internal/app"
	"nice-wiki/internal/config"
	"nice-wiki/internal/datadir"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := checkHealth(); err != nil {
			fmt.Fprintln(os.Stderr, "健康检查失败:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动或运行失败:", err)
		os.Exit(1)
	}
}
func checkHealth() error {
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("状态码 %d", response.StatusCode)
	}
	return nil
}
func run() error {
	flags := flag.NewFlagSet("nice-wiki", flag.ContinueOnError)
	data := flags.String("data", "", "数据目录")
	address := flags.String("addr", "", "监听地址")
	dev := flags.Bool("dev", false, "开发模式")
	level := flags.String("log-level", "", "日志级别")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不接受位置参数")
	}
	overrides := config.Overrides{}
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "data":
			overrides.Data = data
		case "addr":
			overrides.Address = address
		case "dev":
			overrides.Dev = dev
		case "log-level":
			overrides.LogLevel = level
		}
	})
	cfg, err := config.Load(overrides, func(data string) (string, string, error) {
		layout, err := datadir.New(data)
		return layout.Root, layout.Bootstrap(), err
	})
	if err != nil {
		return err
	}
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	return application.Run(ctx)
}
