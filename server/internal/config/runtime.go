package config

import (
	"errors"
	"net/netip"
	"net/url"
	"nice-wiki/internal/fsutil"
	"os"
	"sync"
)

type Runtime struct {
	TrustedProxies          []string `json:"trustedProxies"`
	TurnstileSiteKey        string   `json:"turnstileSiteKey"`
	DailySpendLimit         float64  `json:"dailySpendLimit"`
	SessionHourlySpendLimit float64  `json:"sessionHourlySpendLimit"`
	IPHourlySpendLimit      float64  `json:"ipHourlySpendLimit"`
	GitRemote               string   `json:"gitRemote"`
	BackupHour              int      `json:"backupHour"`
	BackupKeep              int      `json:"backupKeep"`
}

func DefaultRuntime() Runtime {
	return Runtime{TrustedProxies: []string{}, BackupHour: 3, BackupKeep: 7}
}
func (r Runtime) Validate() error {
	if r.DailySpendLimit < 0 || r.SessionHourlySpendLimit < 0 || r.IPHourlySpendLimit < 0 || r.BackupHour < 0 || r.BackupHour > 23 || r.BackupKeep < 1 || r.BackupKeep > 365 {
		return invalid("系统设置范围无效")
	}
	for _, raw := range r.TrustedProxies {
		if _, err := netip.ParsePrefix(raw); err != nil {
			return invalid("可信代理必须使用 CIDR")
		}
	}
	if r.GitRemote != "" {
		u, err := url.Parse(r.GitRemote)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return invalid("远程仓库使用无凭据的 HTTPS 地址")
		}
	}
	return nil
}

type RuntimeStore struct {
	mu    sync.RWMutex
	path  string
	value Runtime
}

func OpenRuntime(path string) (*RuntimeStore, error) {
	value := DefaultRuntime()
	if err := fsutil.ReadJSON(path, &value); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := value.Validate(); err != nil {
		return nil, err
	}
	value.TrustedProxies = append([]string{}, value.TrustedProxies...)
	return &RuntimeStore{path: path, value: value}, nil
}
func (s *RuntimeStore) Get() Runtime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value := s.value
	value.TrustedProxies = append([]string{}, value.TrustedProxies...)
	return value
}
func (s *RuntimeStore) Save(value Runtime) error {
	if err := value.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fsutil.WriteJSON(s.path, value); err != nil {
		return err
	}
	value.TrustedProxies = append([]string{}, value.TrustedProxies...)
	s.value = value
	return nil
}

// ValidationError 的消息只包含可向调用者展示的静态规则，不包含凭据或路径。
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func invalid(message string) error       { return &ValidationError{Message: message} }
