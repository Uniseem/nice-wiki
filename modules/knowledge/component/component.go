// Package component 把本模块暴露为可由宿主进程（services/）装配的组件；
// 领域代码仍在 internal 下，这里只转出装配说明。
package component

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/app"
	"nice-wiki/modules/knowledge/internal/extract"
)

// Spec 返回本组件的装配说明。
func Spec() platform.ServiceSpec {
	return platform.ServiceSpec{
		Module: "knowledge",
		BuildApplication: func(ctx context.Context, config platform.ServiceConfig) (platform.Application, error) {
			return app.BuildApplication(ctx, config)
		},
		RuntimeClient: app.NewRuntimeClient,
	}
}

// RunParserChild 识别隔离的 PDF 解析子进程：宿主进程的 main 必须在读取任何配置之前先调用它，
// 返回 handled=true 时子进程工作已做完，main 直接按 err 退出。
func RunParserChild() (bool, error) { return extract.RunPDFParserChild() }
