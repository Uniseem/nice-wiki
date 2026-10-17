# 公开交付启动装配交接（2026-10-17，pipeline 48 已编译）

## 范围与事实

- 分支 `codex/public-site-20261017-r5`，基线 `a3e6af980d5288e42bc8d023410faf8923ff21e8`，源码提交 `1a126fd06bdbebe7c02e0306ea3b99e635a19b3a`。只改 `modules/public-site/`。未 push。
- 入口 `cmd/public-site/main.go` 只装配本域参数，进程生命周期交给 `platform.RunService`。`go.mod` 固定 Go 1.25.0，以及 kin-openapi v0.133.0、oapi-codegen/runtime v1.1.2、`nice-wiki/libs/platform`。
- `ResolvePublishedLink` 已绑定真实函数，只接受 site-config 纯服务。它读取 site-config、启用语言和 content 公开树；块 ID 非空时核验当前公开正文的 schema、hash 和稳定块 ID。已知撤回返回 `unavailable`。依赖未知返回 503。水位变化返回公共错误表 `STALE_RESULT`/409。成功响应 `no-store`。不调用 `readPublicHome` 或 `readTranslation`。
- `BuildCanonical` 只使用调用方传入的 `publicBaseURL` 和规范路径。输入没有配置对象 ID，所以不调用 `readConfigObject`。
- `BuildApplication` 校验 module、监听、数据目录、信任锚和签名引用，并要求 content、site-config、localization、identity、media、jobs、data-protection 为 http(s) 端点。analytics 可缺。换目录结果不是 none，或任一登记操作没有处理函数时，启动失败。schema 迁移、journal、activation 回执字段和后台循环都还没有写清的实现，函数保持 blocked，不返回可运行应用。
- `Serve`、`Shutdown`、`Health`、`Ready` 已按已写清的生命周期步骤实现。`Ready` 在 journal、恢复控制或业务路由未就绪时返回 503。运行状态的五秒轮询仍由平台 `RunService` 负责。
- `RewriteInternalLinks` 以及其余页面、恢复、事件和 worker 函数仍为 planned。accepted 为 0。`main`、`ResolvePublishedLink`、`BuildCanonical`、`Serve`、`Shutdown`、`Health`、`Ready` 已由 pipeline 48 记 verified。

## 检查

pipeline 47 编译通过，生成补丁已回传。pipeline 48 生成补丁为 0，三个分片成功。下面的登记清单现已完成。

## 请集成会话登记

以下新文件只存在于本域，尚未写入 `docs/structure/framework.json`：

- `modules/public-site/go.mod`
- `modules/public-site/cmd/public-site/main.go`
- `modules/public-site/internal/app/wire.go`
- `modules/public-site/internal/transport/server.go`
- `modules/public-site/internal/links.go`
- `modules/public-site/internal/seo.go`

## 下一步（一项）

pipeline 48 已把公开链接和已写清的生命周期函数记 verified。`BuildApplication` 仍 blocked。下一步补齐 journal、schema 迁移和其余处理函数，不要在缺处理函数时返回可运行应用。
