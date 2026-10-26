// Package component 把本模块暴露为可由宿主进程（services/）装配的组件；
// 领域代码仍在 internal 下，这里只转出装配说明。
package component

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/data-protection/internal/app"
)

// Spec 返回本组件的装配说明。
func Spec() platform.ServiceSpec {
	return platform.ServiceSpec{
		Module: "data-protection",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}
}
