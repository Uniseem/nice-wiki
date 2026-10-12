package jobs

import (
	"context"
	"database/sql"
	"errors"
	"github.com/mikestefanello/backlite"
	"sync"
	"time"
)

// MergeStore 由 app 注入，jobs 不导入 store 业务能力。
type MergeStore interface {
	Enqueue(context.Context, Kind, string, func(*sql.Tx, int64) error) (bool, error)
	Current(context.Context, Kind, string) (int64, error)
	Complete(context.Context, Kind, string, int64) error
}
type Handler func(context.Context, string, func(int)) error
type Progress struct {
	Kind      Kind   `json:"kind"`
	SubjectID string `json:"subjectId"`
	Percent   int    `json:"percent"`
	State     string `json:"state"`
}
type Service struct {
	client   *backlite.Client
	merge    MergeStore
	mu       sync.RWMutex
	handlers map[Kind]Handler
	progress map[string]Progress
}

func Open(db *sql.DB, merge MergeStore) (*Service, error) {
	client, err := backlite.NewClient(backlite.ClientConfig{DB: db, NumWorkers: 2, ReleaseAfter: 15 * time.Minute, CleanupInterval: time.Hour})
	if err != nil {
		return nil, err
	}
	if err := client.Install(); err != nil {
		return nil, err
	}
	return &Service{client: client, merge: merge, handlers: map[Kind]Handler{}, progress: map[string]Progress{}}, nil
}
func (s *Service) Register(kind Kind, handler Handler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if handler == nil {
		return errors.New("任务处理函数不能为空")
	}
	if _, exists := s.handlers[kind]; exists {
		return errors.New("任务已注册")
	}
	s.handlers[kind] = handler
	s.client.Register(&queue{kind: kind, service: s})
	return nil
}
func (s *Service) Enqueue(ctx context.Context, kind Kind, subject string) error {
	s.mu.RLock()
	_, ok := s.handlers[kind]
	s.mu.RUnlock()
	if !ok {
		return errors.New("任务尚未实现，不能投递")
	}
	queued, err := s.merge.Enqueue(ctx, kind, subject, func(tx *sql.Tx, generation int64) error {
		_, err := s.client.Add(Task{Kind: kind, SubjectID: subject, Generation: generation}).Ctx(ctx).Tx(tx).Save()
		return err
	})
	if err != nil {
		return err
	}
	if queued {
		s.client.Notify()
	}
	return nil
}
func (s *Service) Start(ctx context.Context) { s.client.Start(ctx) }
func (s *Service) Stop(ctx context.Context) error {
	if !s.client.Stop(ctx) {
		return errors.New("后台任务未在关闭期限内结束")
	}
	return nil
}
func (s *Service) Progress() []Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Progress{}
	for _, value := range s.progress {
		result = append(result, value)
	}
	return result
}
