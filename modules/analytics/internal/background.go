package analytics

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"sync"
	"time"
)

// RunPageBackground只装配本域回调；周期、worker、订阅机制分别复用公共库。
func RunPageBackground(ctx context.Context, rt *Runtime) error {
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	var group sync.WaitGroup
	errors := make(chan error, 5)
	launchPageLoop := func(run func(context.Context) error) {
		group.Add(1)
		go func() {
			defer group.Done()
			err := run(work)
			if err != nil {
				select {
				case errors <- err:
				default:
				}
				cancel()
			}
		}()
	}
	launchPageLoop(func(ctx context.Context) error {
		for ctx.Err() == nil {
			call, done, op := backgroundOperation(ctx, rt, "", "", 24*time.Hour)
			_, err := RunWorker(call, op, WorkerLoopInput{MaxConcurrency: 2})
			expired := call.Err() == context.DeadlineExceeded
			done()
			if err != nil {
				return err
			}
			if !expired && ctx.Err() == nil {
				return platform.NewError(platform.CodeDependencyUnavailable, false, "公共任务循环提前退出")
			}
		}
		return nil
	})
	launchPageLoop(func(ctx context.Context) error { return runAnalyticsPeriodic(ctx, rt, 60000, SchedulePageAggregates) })
	launchPageLoop(func(ctx context.Context) error {
		for ctx.Err() == nil {
			call, done, op := backgroundOperation(ctx, rt, "", "", 24*time.Hour)
			_, err := ConsumeEvents(call, op, ConsumeEventsInput{ConsumerID: "analytics", MaxInFlight: 4})
			expired := call.Err() == context.DeadlineExceeded
			done()
			if err != nil {
				return err
			}
			if !expired && ctx.Err() == nil {
				return platform.NewError(platform.CodeDependencyUnavailable, false, "公共订阅循环提前退出")
			}
		}
		return nil
	})
	launchPageLoop(func(ctx context.Context) error {
		return runAnalyticsPeriodic(ctx, rt, 1000, func(ctx context.Context, op OperationContext) error {
			_, err := PublishAggregateEvents(ctx, op, PublishAggregateEventsInput{BatchSize: 100, DataEpoch: rt.Epoch})
			return err
		})
	})
	launchPageLoop(func(ctx context.Context) error { return RunRetentionBackground(ctx, rt) })
	err := runAnalyticsPeriodic(work, rt, 5000, func(ctx context.Context, op OperationContext) error {
		_, err := FlushBeacons(ctx, op, FlushBeaconsInput{BatchSize: 500, DataEpoch: rt.Epoch})
		return err
	})
	cancel()
	group.Wait()
	if err != nil {
		return err
	}
	select {
	case err = <-errors:
		return err
	default:
		return nil
	}
}

// runAnalyticsPeriodic只装配本域可信操作，轮询/运行权威/安全窗口由平台统一执行。
func runAnalyticsPeriodic(ctx context.Context, rt *Runtime, millis int, run func(context.Context, OperationContext) error) error {
	return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: millis, Runtime: rt.Window.Runtime, WriteWindow: rt.Window.WriteWindow, Recover: func(ctx context.Context) error {
		call, cancel, op := backgroundOperation(ctx, rt, "", "", 30*time.Second)
		defer cancel()
		return run(call, op)
	}})
}

// DrainBeacons只供HTTP停止或公共屏障私有cleanup上下文排空，不能用普通新写通行证。
func DrainBeacons(ctx context.Context, rt *Runtime) error {
	for {
		rt.mu.Lock()
		pending := len(rt.queue) != 0 || rt.dropped != 0
		rt.mu.Unlock()
		if !pending {
			return nil
		}
		call, cancel, op := backgroundOperation(ctx, rt, "", "", 4*time.Second)
		_, err := FlushBeacons(call, op, FlushBeaconsInput{BatchSize: 500, DataEpoch: rt.Epoch})
		cancel()
		if err != nil {
			return err
		}
	}
}

// RunRetentionBackground有界续清，仅把本域保留策略回调交给平台周期机制。
func RunRetentionBackground(ctx context.Context, rt *Runtime) error {
	lastHour, lastAudit := "", ""
	return runAnalyticsPeriodic(ctx, rt, 60000, func(call context.Context, op OperationContext) error {
		now := time.Now().UTC()
		if err := PruneDaySalts(call, rt, now); err != nil {
			return err
		}
		for batch := 0; batch < 50; batch++ {
			result, err := PurgeBeacons(call, op, PurgeBeaconsInput{Now: now.Format(analyticsTimeLayout), BatchSize: 500})
			if err != nil {
				return err
			}
			if result.Deleted < 500 {
				break
			}
		}
		hour := now.Format("2006-01-02T15")
		if lastHour != hour {
			if _, err := PurgeExports(call, op, PurgeExportsInput{Now: now.Format(analyticsTimeLayout)}); err != nil {
				return err
			}
			lastHour = hour
		}
		day := now.Format("2006-01-02")
		if lastAudit != day {
			result, err := PurgeAuditHistory(call, op, PurgeAuditHistoryInput{Now: now.Format(analyticsTimeLayout), RetentionDays: 180})
			if err != nil {
				return err
			}
			if result.Deleted < 10000 {
				lastAudit = day
			}
		}
		return nil
	})
}
