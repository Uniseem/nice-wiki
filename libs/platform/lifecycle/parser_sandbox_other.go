//go:build !linux || !amd64

package lifecycle

import "nice-wiki/libs/platform"

// EnterParserSandbox 在尚未审定隔离规则的平台失败关闭。
func EnterParserSandbox() error {
	return platform.NewError(platform.CodeDependencyUnavailable, false, "当前平台不支持受控解析隔离")
}
