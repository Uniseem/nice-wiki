// identity 服务可执行入口：只装配本域参数，进程模板、信号与退出码
// 语义由 libs/platform/lifecycle.RunService 统一实现。
//
// 合同中的 `accounts` 运维子命令依赖 identity.ReadServerAccountIndex
// （server_recovery.go，后续业务工作包），随该函数一并接线。
package main

import (
	"context"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/identity/internal/app"
)

func main() {
	os.Exit(lifecycle.RunService(platform.ServiceSpec{
		Module: "identity",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}))
}
