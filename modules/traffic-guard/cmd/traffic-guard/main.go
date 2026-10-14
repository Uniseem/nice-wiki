// traffic-guard 服务入口只装配本域参数；进程、信号、恢复重启与退出码
// 由 libs/platform/lifecycle.RunService 统一处理。
package main

import (
	"context"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/traffic-guard/internal/app"
)

func main() {
	os.Exit(lifecycle.RunService(platform.ServiceSpec{
		Module: "traffic-guard",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}))
}
