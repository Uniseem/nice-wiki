package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"nice-wiki/internal/fsutil"
	"nice-wiki/internal/store/sqlc"
	"os"
	"regexp"
	"sync"
)

var ErrNotFound = errors.New("账号不存在")
var ErrConflict = errors.New("账号状态冲突")
var ErrCredentials = errors.New("账号或密码错误")
var ErrLocked = errors.New("登录暂时锁定，请稍后重试")
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_\-\p{Han}]{1,64}$`)

type Account struct {
	ID                 string       `json:"id"`
	Username           string       `json:"username"`
	PasswordHash       string       `json:"passwordHash"`
	Superadmin         bool         `json:"superadmin"`
	Disabled           bool         `json:"disabled"`
	MustChangePassword bool         `json:"mustChangePassword"`
	Permissions        []Permission `json:"permissions"`
}
type Service struct {
	mu        sync.RWMutex
	loginMu   sync.Mutex
	path      string
	accounts  []Account
	queries   *sqlc.Queries
	dummyHash string
}

func Open(path string, queries *sqlc.Queries) (*Service, error) {
	accounts := []Account{}
	if err := fsutil.ReadJSON(path, &accounts); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	names := map[string]bool{}
	ids := map[string]bool{}
	superadmins := 0
	for _, account := range accounts {
		if !usernamePattern.MatchString(account.Username) || names[account.Username] || ids[account.ID] || account.ID == "" {
			return nil, errors.New("账号文件包含无效或重复身份")
		}
		if err := ValidPermissions(account.Permissions); err != nil {
			return nil, err
		}
		names[account.Username] = true
		ids[account.ID] = true
		if account.Superadmin {
			superadmins++
			if account.Disabled {
				return nil, errors.New("超管不能停用")
			}
		}
	}
	if len(accounts) > 0 && superadmins != 1 {
		return nil, errors.New("账号文件必须有且仅有一个超管")
	}
	dummyHash, err := HashPassword("unavailable-account-password")
	if err != nil {
		return nil, err
	}
	return &Service{path: path, accounts: accounts, queries: queries, dummyHash: dummyHash}, nil
}
func NewID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
func copyAccount(value Account) Account {
	value.Permissions = append([]Permission{}, value.Permissions...)
	return value
}
func (s *Service) Registered() bool { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.accounts) > 0 }
func (s *Service) List() []Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Account, len(s.accounts))
	for i, a := range s.accounts {
		result[i] = copyAccount(a)
		result[i].PasswordHash = ""
	}
	return result
}
func (s *Service) ByID(id string) (Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.accounts {
		if a.ID == id {
			return copyAccount(a), nil
		}
	}
	return Account{}, ErrNotFound
}
func (s *Service) byName(name string) (Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.accounts {
		if a.Username == name {
			return copyAccount(a), nil
		}
	}
	return Account{}, ErrNotFound
}
func (s *Service) saveLocked(next []Account) error {
	if err := fsutil.WriteJSON(s.path, next); err != nil {
		return err
	}
	s.accounts = next
	return nil
}
func (s *Service) Create(username, password string, permissions []Permission) (Account, error) {
	if !usernamePattern.MatchString(username) {
		return Account{}, invalid("账号名只允许字母、数字、中文、下划线和连字符")
	}
	if err := ValidPermissions(permissions); err != nil {
		return Account{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	id, err := NewID()
	if err != nil {
		return Account{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, account := range s.accounts {
		if account.Username == username {
			return Account{}, ErrConflict
		}
	}
	account := Account{ID: id, Username: username, PasswordHash: hash, MustChangePassword: true, Permissions: append([]Permission{}, permissions...)}
	if err := s.saveLocked(append(append([]Account{}, s.accounts...), account)); err != nil {
		return Account{}, err
	}
	return copyAccount(account), nil
}
func (s *Service) Update(ctx context.Context, id string, disabled bool, permissions []Permission) (Account, error) {
	if err := ValidPermissions(permissions); err != nil {
		return Account{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Account{}, s.accounts...)
	for i, account := range next {
		if account.ID != id {
			continue
		}
		if account.Superadmin {
			return Account{}, invalid("不能修改超管的权限或停用状态")
		}
		account.Disabled = disabled
		account.Permissions = append([]Permission{}, permissions...)
		next[i] = account
		if err := s.queries.DeleteAccountSessions(ctx, id); err != nil {
			return Account{}, err
		}
		if err := s.saveLocked(next); err != nil {
			return Account{}, err
		}
		return copyAccount(account), nil
	}
	return Account{}, ErrNotFound
}
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := []Account{}
	found := false
	for _, account := range s.accounts {
		if account.ID == id {
			if account.Superadmin {
				return invalid("不能删除超管")
			}
			found = true
		} else {
			next = append(next, account)
		}
	}
	if !found {
		return ErrNotFound
	}
	if err := s.queries.DeleteAccountSessions(ctx, id); err != nil {
		return err
	}
	return s.saveLocked(next)
}
func (s *Service) ResetPassword(ctx context.Context, id, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Account{}, s.accounts...)
	for i, account := range next {
		if account.ID == id {
			if err := s.queries.DeleteAccountSessions(ctx, id); err != nil {
				return err
			}
			account.PasswordHash = hash
			account.MustChangePassword = true
			next[i] = account
			return s.saveLocked(next)
		}
	}
	return ErrNotFound
}

// ValidationError 的消息只包含可向调用者展示的静态规则，不包含凭据或路径。
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func invalid(message string) error       { return &ValidationError{Message: message} }
