// portal 服务入口只装配本域参数；信号、退出码和安全关闭由平台模板统一处理。
package main

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/portal/internal/app"
)

func main() {
	lifecycle.RunService(platform.ServiceSpec{
		Module: app.ModuleSlug,
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	})
}
