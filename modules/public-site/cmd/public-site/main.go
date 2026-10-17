// public-site 服务可执行入口：只装配本域参数，进程生命周期由平台统一管理。
package main

import (
	"context"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/public-site/internal/app"
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
