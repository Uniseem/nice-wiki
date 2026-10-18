package lifecycle

import (
	"context"
	"nice-wiki/libs/platform"
	"sync"
	"time"
)

// TaskGroup只监督已由域持久接受的本进程任务；费用与终态仍由域负责。
type TaskGroup struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	tasks  map[string]context.CancelFunc
	slots  chan struct{}
	closed bool
	wg     sync.WaitGroup
	once   sync.Once
	done   chan struct{}
}

func NewTaskGroup(parent context.Context, maxConcurrent int) (*TaskGroup, error) {
	if parent == nil || maxConcurrent < 1 || maxConcurrent > 256 {
		return nil, platform.NewError(platform.CodeValidation, false, "进程任务集合配置不合法")
	}
	ctx, cancel := context.WithCancel(parent)
	g := &TaskGroup{ctx: ctx, cancel: cancel, tasks: map[string]context.CancelFunc{}, slots: make(chan struct{}, maxConcurrent), done: make(chan struct{})}
	context.AfterFunc(ctx, g.stop)
	return g, nil
}

// Start保留原截止及调用值，脱离HTTP断开；进程停止、Cancel仍取消下游IO。
func (g *TaskGroup) Start(ctx context.Context, key string, run func(context.Context) error) (bool, error) {
	deadline, ok := ctx.Deadline()
	if key == "" || run == nil || !ok || !deadline.After(time.Now()) {
		return false, platform.NewError(platform.CodeValidation, false, "任务缺少唯一key或有效原截止")
	}
	g.mu.Lock()
	if g.closed || g.ctx.Err() != nil {
		g.mu.Unlock()
		return false, platform.NewError(platform.CodeCancelled, false, "进程任务集合已关闭")
	}
	if _, exists := g.tasks[key]; exists {
		g.mu.Unlock()
		return false, nil
	}
	select {
	case g.slots <- struct{}{}:
	default:
		g.mu.Unlock()
		return false, platform.NewError(platform.CodeRateLimited, true, "进程任务并发已满")
	}
	work, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	stop := context.AfterFunc(g.ctx, cancel)
	g.tasks[key] = cancel
	g.wg.Add(1)
	g.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			stop()
			g.mu.Lock()
			delete(g.tasks, key)
			g.mu.Unlock()
			<-g.slots
			g.wg.Done()
		}()
		// 回调必须先持久记录本域成功/失败及费用；业务error不取消其它用户任务。
		_ = run(work)
	}()
	return true, nil
}
func (g *TaskGroup) Cancel(key string) bool {
	g.mu.Lock()
	cancel, ok := g.tasks[key]
	g.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}
func (g *TaskGroup) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
	case <-g.ctx.Done():
	}
	g.stop()
	<-g.done
	return nil
}
func (g *TaskGroup) Drain(ctx context.Context) error {
	g.stop()
	select {
	case <-g.done:
		return nil
	case <-ctx.Done():
		return platform.NormalizeIOError(ctx.Err(), platform.CodeDeadlineExceeded, false, "进程任务尚未全部退出")
	}
}
func (g *TaskGroup) stop() {
	g.once.Do(func() {
		g.mu.Lock()
		g.closed = true
		g.cancel()
		for _, cancel := range g.tasks {
			cancel()
		}
		g.mu.Unlock()
		go func() { g.wg.Wait(); close(g.done) }()
	})
}
