// model-gateway 服务入口只装配本域参数；进程生命周期、信号、恢复重启
// 与退出码由 libs/platform/lifecycle.RunService 统一处理。
package main

import (
	"context"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/model-gateway/internal/app"
)

func main() {
	os.Exit(lifecycle.RunService(platform.ServiceSpec{
		Module: "model-gateway",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}))
}
