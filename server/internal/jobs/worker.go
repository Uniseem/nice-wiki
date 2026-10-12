package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mikestefanello/backlite"
)

type queue struct {
	kind    Kind
	service *Service
}

func (q *queue) Config() *backlite.QueueConfig { v := Config(q.kind); return &v }
func (q *queue) Process(ctx context.Context, payload []byte) error {
	var task Task
	if err := json.Unmarshal(payload, &task); err != nil {
		return err
	}
	if task.Kind != q.kind {
		return errors.New("任务类型与队列不一致")
	}
	s := q.service
	s.mu.RLock()
	handler := s.handlers[q.kind]
	s.mu.RUnlock()
	if handler == nil {
		return errors.New("任务缺少处理函数")
	}
	key := string(task.Kind) + ":" + task.SubjectID
	update := func(percent int, state string) {
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		s.mu.Lock()
		s.progress[key] = Progress{Kind: task.Kind, SubjectID: task.SubjectID, Percent: percent, State: state}
		s.mu.Unlock()
	}
	update(0, "running")
	// 使用最新代次处理，在运行期间更新的任务会在完成后再投递一次。
	generation, err := s.merge.Current(ctx, task.Kind, task.SubjectID)
	if err != nil {
		return err
	}
	err = handler(ctx, task.SubjectID, func(percent int) { update(percent, "running") })
	if err != nil {
		update(0, "failed")
		return err
	}
	if err := s.merge.Complete(ctx, task.Kind, task.SubjectID, generation); err != nil {
		return err
	}
	update(100, "completed")
	latest, err := s.merge.Current(ctx, task.Kind, task.SubjectID)
	if err != nil {
		return err
	}
	if latest > generation {
		return s.Enqueue(ctx, task.Kind, task.SubjectID)
	}
	return nil
}
