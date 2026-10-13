# portal 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=33；分类={"handwritten": 15, "ui": 7, "config": 9, "generated": 2}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/portal/internal/route_registry.go` | handwritten | 校验并编译公开与管理 API 的静态路由表 | `portal.CompileRouteRegistry`, `portal.ResolveRoute` |
| `modules/portal/internal/request_validation.go` | handwritten | 限制请求体、请求头与 operationId | `portal.ValidateEnvelope`, `portal.NormalizeForwardHeaders` |
| `modules/portal/internal/origin_guard.go` | handwritten | 对写入及凭据请求检查来源 | `portal.AssertSameOrigin`, `portal.VerifyCSRF` |
| `modules/portal/internal/auth_proof.go` | handwritten | 取得经 identity 校验的当前权限上下文 | `portal.ResolveAuthContext`, `portal.BuildInternalProof`, `portal.AuthorizeRoute` |
| `modules/portal/internal/proxy_stream.go` | handwritten | 发送一次有界请求到拥有服务 | `portal.ForwardRequest`, `portal.StreamResponse`, `portal.MapUpstreamFailure` |
| `modules/portal/internal/bootstrap_service.go` | handwritten | 聚合当前站点与用户后台壳信息 | `portal.LoadBootstrap`, `portal.BuildCapabilities`, `portal.BuildNavigation` |
| `modules/portal/internal/preferences_service.go` | handwritten | 读取用户的显示偏好 | `portal.ReadPreferences`, `portal.UpdatePreferences`, `portal.ValidatePreferenceRoute` |
| `modules/portal/internal/event_bridge.go` | handwritten | 提供后台权限过滤 SSE | `portal.OpenEventStream`, `portal.FilterPortalEvent`, `portal.EncodeSSEFrame`, `portal.ResumeEventCursor` |
| `modules/portal/internal/health_service.go` | handwritten | 输出入口自身及必要依赖可用性 | `portal.CollectReadiness` |
| `modules/portal/internal/audit_redaction.go` | handwritten | 生成不会泄漏密码、token 或正文的访问日志字段 | `portal.RedactRequest`, `portal.EmitAccessAudit` |
| `modules/portal/web/src/api/api-client.ts` | ui | 统一浏览器 API 请求与错误状态 | `portal.sendApi`, `portal.refreshSingleFlight`, `portal.classifyApiError` |
| `modules/portal/web/src/state/session-store.ts` | ui | 将服务端 session 和权限装入界面状态 | `portal.hydrateSession`, `portal.applyPermissionEvent`, `portal.clearSession` |
| `modules/portal/web/src/router/router.ts` | ui | 解析后台地址并检查能力 | `portal.resolveAdminRoute`, `portal.guardUnsavedNavigation`, `portal.restoreReturnLocation` |
| `modules/portal/web/src/events/event-client.ts` | ui | 连接并自动恢复后台 SSE | `portal.connectEventStream`, `portal.dispatchPortalEvent` |
| `modules/portal/web/src/preferences/preferences-store.ts` | ui | 加载并应用用户显示偏好 | `portal.loadPreferences`, `portal.savePreferences` |
| `modules/portal/web/src/layout/admin-shell.ts` | ui | 计算初始化、登录、后台及降级壳状态 | `portal.deriveShellState`, `portal.updateDocumentMetadata` |
| `modules/portal/internal/runtime_state_client.go` | handwritten | 读取唯一权威运行状态 | `portal.ReadRuntimeState` |
| `modules/portal/internal/maintenance_gate.go` | handwritten | 对维护和恢复 epoch 实施入口屏障 | `portal.CheckEntranceGate`, `portal.AcquireBarrier`, `portal.ReleaseBarrier` |
| `modules/portal/cmd/portal/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M01-F901` |
| `modules/portal/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M01-F902` |
| `modules/portal/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M01-F903`, `M01-F904`, `M01-F905`, `M01-F906` |
| `modules/portal/go.mod` | config | 独立module nice-wiki/modules/portal；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/portal/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/portal/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/portal；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/portal/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/portal与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/portal/config/service.defaults.json` | config | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/portal/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/portal/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/portal/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/portal/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/portal/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/portal/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/portal/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M01-F907` |
