package auth

import (
	"context"
	"errors"
	"os"
	"strings"
)

func (s *Service) CheckReset(ctx context.Context, path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 2048 {
		return errors.New("超管重置文件类型或大小无效")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	var superadmin Account
	for _, account := range s.List() {
		if account.Superadmin {
			superadmin = account
			break
		}
	}
	if superadmin.ID == "" {
		return errors.New("尚未注册超管，不能重置")
	}
	if err := s.ResetPassword(ctx, superadmin.ID, password); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return s.Audit(ctx, superadmin, "reset_superadmin", superadmin.ID)
}
