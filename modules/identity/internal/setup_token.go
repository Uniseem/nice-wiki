package identity

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nice-wiki/libs/platform"
)

// 首次注册站主要出示的安装令牌。它放在 control 区的私有文件里，只有能登上部署机的人拿得到：
// 公开部署的域名一签出证书就会被扫到，没有它，谁先打开后台谁就成了站主。
const setupTokenName = "setup-token"

// EnsureSetupToken 在还没有任何账户时准备好安装令牌（已有就沿用），并在日志里指出它的位置；已有账户时删掉它。
func (s *Store) EnsureSetupToken(ctx context.Context) error {
	path := filepath.Join(s.dataRoot, "control", setupTokenName)
	var accounts int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accounts); err != nil {
		return identityDBError()
	}
	if accounts > 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return platform.NewError(platform.CodeInternal, false, "删除安装令牌失败")
		}
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		token := string(platform.NewIdentifier()) + string(platform.NewIdentifier())
		if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
			return platform.NewError(platform.CodeInternal, false, "写入安装令牌失败")
		}
	}
	fmt.Fprintf(os.Stderr, "还没有站主：首次注册需要安装令牌，见 %s\n", path)
	return nil
}

func (s *Store) checkSetupToken(supplied string) error {
	raw, err := os.ReadFile(filepath.Join(s.dataRoot, "control", setupTokenName))
	if err != nil {
		return platform.NewError(platform.CodeForbidden, false, "安装令牌不可用，请重启身份服务后重试")
	}
	if supplied == "" || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(string(raw))), []byte(strings.TrimSpace(supplied))) != 1 {
		return platform.NewError(platform.CodeForbidden, false, "安装令牌不对")
	}
	return nil
}

func (s *Store) removeSetupToken() {
	_ = os.Remove(filepath.Join(s.dataRoot, "control", setupTokenName))
}
