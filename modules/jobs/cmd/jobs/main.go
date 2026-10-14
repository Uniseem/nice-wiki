// jobs 服务可执行入口：只装配本域参数，进程模板、信号与退出码
// 语义由 libs/platform/lifecycle.RunService 统一实现。
package main

import (
	"context"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/jobs/internal/app"
)

func main() {
	os.Exit(lifecycle.RunService(platform.ServiceSpec{
		Module: app.ModuleSlug,
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}))
}
