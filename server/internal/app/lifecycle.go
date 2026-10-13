package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"nice-wiki/internal/backup"
	"nice-wiki/internal/jobs"
	"time"
)

func (a *App) Run(ctx context.Context) (err error) {
	defer func() { err = errors.Join(err, a.DB.Close(), a.Lock.Close()) }()
	handler, err := a.Handler()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", a.Config.Address)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	// 工作线程在 HTTP 请求排空后才停止，避免关闭数据库时仍有处理函数运行。
	workerContext, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	a.Jobs.Start(workerContext)
	a.Jobs.Schedule(workerContext, time.Hour, jobs.Cleanup, "sessions", func(err error) { a.Logger.Error("会话清理任务投递失败") })
	a.Jobs.Schedule(workerContext, 24*time.Hour, jobs.Rollup, "daily", func(err error) { a.Logger.Error("访问统计汇总任务投递失败") })
	backup.Schedule(workerContext, func() int { return a.Runtime.Get().BackupHour }, func(ctx context.Context) error {
		return a.Jobs.Enqueue(ctx, jobs.Backup, "scheduled")
	}, func(err error) { a.Logger.Error("定时备份任务投递失败") })
	resetContext, stopReset := context.WithCancel(ctx)
	resetDone := make(chan struct{})
	go a.watchReset(resetContext, resetDone)
	defer stopReset()
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	a.Logger.Info("服务已启动", "dev", a.Config.Dev)
	select {
	case <-ctx.Done():
	case serveErr := <-serveErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			err = serveErr
		}
	}
	stopReset()
	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if shutdownErr := server.Shutdown(shutdown); shutdownErr != nil {
		err = errors.Join(err, shutdownErr, server.Close())
	}
	if stopErr := a.Jobs.Stop(shutdown); stopErr != nil {
		stopWorkers()
		err = errors.Join(err, stopErr)
	}
	select {
	case <-resetDone:
	case <-shutdown.Done():
		err = errors.Join(err, errors.New("超管重置任务未在关闭期限内结束"))
	}
	return err
}

func (a *App) watchReset(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.Auth.CheckReset(ctx, a.Layout.ResetSuperadmin()); err != nil {
				if ctx.Err() != nil {
					return
				}
				// 重置文件可能含密码，不记录文件内容和底层错误正文。
				a.Logger.Error("超管重置失败，请检查重置文件")
			}
		}
	}
}
