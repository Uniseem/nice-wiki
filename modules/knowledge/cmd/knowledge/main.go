// knowledge入口先识别隔离PDF子进程，再由平台读取配置/密钥并管理进程生命周期。
package main

import (
 "context"
 "os"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
 "nice-wiki/modules/knowledge/internal/app"
 "nice-wiki/modules/knowledge/internal/extract"
)

func main() {
 handled,err:=extract.RunPDFParserChild()
 if handled {if err!=nil {os.Exit(1)};os.Exit(0)}
 os.Exit(lifecycle.RunService(platform.ServiceSpec{Module:app.ModuleSlug,BuildApplication:func(ctx context.Context,config platform.ServiceConfig)(platform.Application,error){return app.BuildApplication(ctx,config)},RuntimeClient:app.NewRuntimeClient}))
}
