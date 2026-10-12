package jobs

import (
	"context"
	"time"
)

// Schedule 只为已注册处理函数安排周期任务，取消 context 后停止。
func (s *Service) Schedule(ctx context.Context, interval time.Duration, kind Kind, subject string, onError func(error)) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Enqueue(ctx, kind, subject); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
}
