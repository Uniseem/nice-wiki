# portal 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 27 个：手写 15、UI 1、生成 5、配置 6。函数 51 个：约束 15，参考 36。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/portal/internal/route_registry.go` | 手写 | 校验并编译公开与管理 API 的静态路由表 | `portal.CompileRouteRegistry`（参考）、`portal.ResolveRoute`（参考） |
| `modules/portal/internal/request_validation.go` | 手写 | 限制请求体、请求头与 operationId | `portal.ValidateEnvelope`（参考）、`portal.NormalizeForwardHeaders`（参考） |
| `modules/portal/internal/origin_guard.go` | 手写 | 对写入及凭据请求检查来源 | `portal.AssertSameOrigin`（参考）、`portal.VerifyCSRF`（参考） |
| `modules/portal/internal/auth_proof.go` | 手写 | 取得经 identity 校验的当前权限上下文 | `portal.ResolveAuthContext`（参考）、`portal.BuildInternalProof`（参考）、`portal.AuthorizeRoute`（参考）、`portal.Resolve`（参考） |
| `modules/portal/internal/proxy_stream.go` | 手写 | 发送一次有界请求到拥有服务 | `portal.ForwardRequest`（参考）、`portal.StreamResponse`（参考）、`portal.MapUpstreamFailure`（参考）、`portal.ProxyRequest`、`portal.authenticationOperation`（参考）、`portal.prepareAuthenticationRequest`（参考）、`portal.adaptAuthenticationResponse`（参考）、`portal.decodeAuthenticationJSON`（参考）、`portal.authenticationCookies`（参考）、`portal.authenticationProblem`（参考）、`portal.isCurrentSessionRevocation`（参考）、`portal.adaptSessionRevocation`（参考）、`portal.prepareTrustedBeacon`（参考） |
| `modules/portal/internal/bootstrap_service.go` | 手写 | 聚合当前站点与用户后台壳信息 | `portal.LoadBootstrap`、`portal.BuildCapabilities`（参考）、`portal.BuildNavigation`（参考） |
| `modules/portal/internal/preferences_service.go` | 手写 | 读取用户的显示偏好 | `portal.ReadPreferences`、`portal.UpdatePreferences`、`portal.ValidatePreferenceRoute`（参考） |
| `modules/portal/internal/event_bridge.go` | 手写 | 提供后台权限过滤 SSE | `portal.OpenEventStream`、`portal.FilterPortalEvent`（参考）、`portal.EncodeSSEFrame`（参考）、`portal.ResumeEventCursor`（参考） |
| `modules/portal/internal/health_service.go` | 手写 | 输出入口自身及必要依赖可用性 | `portal.CollectReadiness` |
| `modules/portal/internal/audit_redaction.go` | 手写 | 生成不会泄漏密码、token 或正文的访问日志字段 | `portal.RedactRequest`（参考）、`portal.EmitAccessAudit`（参考） |
| `modules/portal/internal/runtime_state_client.go` | 手写 | 读取唯一权威运行状态 | `portal.ReadRuntimeState`（参考） |
| `modules/portal/internal/maintenance_gate.go` | 手写 | 对维护和恢复 epoch 实施入口屏障 | `portal.CheckEntranceGate`（参考）、`portal.AcquireBarrier`、`portal.ReleaseBarrier` |
| `modules/portal/cmd/portal/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `portal.main` |
| `modules/portal/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `portal.BuildApplication`、`portal.readSiteIdentity`（参考） |
| `modules/portal/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `portal.Serve`、`portal.Shutdown`、`portal.Health`、`portal.Ready`、`portal.NewApplication`（参考）、`portal.registerLocalRoutes`（参考）、`portal.proxyHandler`（参考） |
| `modules/portal/go.mod` | 配置 | 独立module nice-wiki/modules/portal；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/portal/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/portal/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/portal；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/portal/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/portal与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/portal/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/portal/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/portal/config/routes.json` | 生成 | 由全部模块openapi.json生成的入口路由表：非service接口的方法/路径→owner、operationId、x-permission、x-admission、超时与流式标志；CompileRouteRegistry启动时加载，未登记路径一律404 | 无 |
| `modules/portal/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/portal/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改；公共后台页面权限与界面语言目录数据 | 无 |
| `modules/portal/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/portal/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript生成浏览器DTO；本域OpenAPI的x-browser-auth-adapters替换identity四认证操作的浏览器输入/输出，内部identity Go契约和真实路由归属保持生产方；生成目录不手改 | 无 |
| `modules/portal/web/src/App.tsx` | UI | 后台单页应用根：登录/初始化向导、后台外壳与按 permission-groups.adminRoutes 装配的各域页面 | `portal.App` |
