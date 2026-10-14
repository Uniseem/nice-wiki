// site-config 服务可执行入口：只装配本域参数，进程模板、信号与退出码
// 语义由 libs/platform/lifecycle.RunService 统一实现。
//
// 本域是运行状态权威，进程模板的运行监视经 app.NewRuntimeClient 读
// 自身权威（本地控制库），不发起自调用 HTTP。
package main

import (
	"context"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/site-config/internal/app"
)

func main() {
	os.Exit(lifecycle.RunService(platform.ServiceSpec{
		Module: "site-config",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}))
}
