// agent 承载独立的 ZCode 原生运行时。
package main

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/agent/internal/app"
	"os"
)

func main() {
	os.Exit(lifecycle.RunService(platform.ServiceSpec{Module: app.ModuleSlug, BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
		return app.BuildApplication(ctx, config)
	}, RuntimeClient: app.NewRuntimeClient}))
}
