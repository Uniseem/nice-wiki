package internal

import (
	"nice-wiki/libs/platform"
)

// ReadLeaseRuntime 复用platform公开运行权威：当前不存在屏障或安全点，视图就是运行状态本身。
func (s *Store) ReadLeaseRuntime(runtime platform.RuntimeView) (LeaseRuntimeView, error) {
	return LeaseRuntimeView{Runtime: runtime}, nil
}

// ReadReadinessEvidence 只依赖本域数据库journal是否已恢复；初始化状态由调用方从站点配置读出。
func (s *Store) ReadReadinessEvidence(initialized bool) (journalRecovered, publicRequestsAllowed bool, err error) {
	if s == nil {
		return false, false, platform.NewError(platform.CodeDependencyUnavailable, true, "本域journal尚未完成恢复")
	}
	s.mu.RLock()
	closed, recovered := s.closed, s.journalRecovered
	s.mu.RUnlock()
	if closed || !recovered {
		return false, false, platform.NewError(platform.CodeDependencyUnavailable, true, "本域journal尚未完成恢复")
	}
	return true, initialized, nil
}
