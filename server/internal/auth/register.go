package auth

func (s *Service) Register(username, password string) (Account, error) {
	if s.Registered() {
		return Account{}, ErrConflict
	}
	if !usernamePattern.MatchString(username) {
		return Account{}, invalid("账号名无效")
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
	if len(s.accounts) != 0 {
		return Account{}, ErrConflict
	}
	account := Account{ID: id, Username: username, PasswordHash: hash, Superadmin: true, Permissions: append([]Permission{}, AllPermissions...)}
	if err := s.saveLocked([]Account{account}); err != nil {
		return Account{}, err
	}
	return copyAccount(account), nil
}
