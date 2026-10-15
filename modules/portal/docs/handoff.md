# 主系统会话交接

## 持续实施第三轮浏览器认证边界（2026-10-15，源码已交付）

- 独占modules/portal/；分支codex/portal-auth-boundary-r1，基线9acad1ab56532521222f592af0d4b3a849792db5；公共原提交918dd6369ea7a33bf9296949c8b5441a08d1e88f已cherry-pick为9303d36，未改公共源码。没有新增实际文件。源码交付提交为df96a3d6b35a6a49d5d9b901559386c3074573b4，随后记录提交只回填此真实源码SHA和已完成函数的verified证据。
- 固定identity createSession/registerFirstSuperadmin/refreshSession/changePassword四操作：内部tokens/currentSessionTokens不进入浏览器JSON；nw_sid/nw_refresh均host-only Path=/ Secure HttpOnly SameSite=Strict，access期限使用identity expiresAt，refresh为浏览器session cookie。csrfToken仅安全session响应给浏览器内存，不另设可读cookie；内部SessionCookies.csrfCookie按既有可选双提交分支改为非required。
- refresh只接受JSON空对象，仅从唯一nw_refresh生成内部正文并重算真实bytes/hash；匿名刷新不要求有效access/proof，固定部署Origin仍必须。登录/首次注册调用公共PrivateNetworkScopes，从真实RemoteAddr及受信TrustedProxyCIDRs代理后缀派生当前+上一UTC日HMAC并签入受信op；不信任任意XFF，不记录原地址。
- 敏感响应64KiB总界，成功DTO/字段/期限/cookie先完成构造再写头；失败只保标准Problem决定和固定安全说明，不设/清旧cookie，敏感回执CONFLICT要求重新登录。当前session revoke只用ResolveAuthContext权威SessionID与路由参数相等且200 revoked=true后清cookie；撤其他设备和任何失败不清。
- 修正既存缺口：NewApplication改为返回(*Application,error)，静态九项required本域路由与实际非nil handler map未齐即DEPENDENCY_UNAVAILABLE；不注册默认503占位。Ready缺初始化/恢复权威继续明确失败，不固定返回Initialized/JournalRecovered=true。BuildApplication/Ready仍blocked，不宣称整站登录已可运行。
- 浏览器OpenAPI分层：x-browser-auth-adapters登记四操作的requestSchema/responseSchema，VPS生成器只替换浏览器spec副本；内部identity Go契约与真实routes归属不变。api.gen.ts不手改，根负责公共生成器。
- 本轮VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14模块readonly build/vet/gofmt与完整make check全通过（856文件/1047函数/374操作/2222类型），0并发冲突；生成/格式已回传，四操作浏览器TS安全shape已复读。源码交付为df96a3d6b35a6a49d5d9b901559386c3074573b4，已完成参考函数与生成物已按此SHA回填verified；文件仍有完整binding缺口时保留in_progress。没有本机Go/gofmt/gen/vet/类型检查/测试，也没有新测试/mock/fixture/临时业务程序、真实接口、模型、服务启动、部署或push；所有业务验收未执行。完整ProxyRequest尚缺准入/审计、Serve尚缺runtime watcher及恢复退出，和BuildApplication/Ready一起保持blocked。
- 后续下一项：已登记internal/audit_redaction.go的RedactRequest纯函数。先读本域requirements/data及implementation的RedactRequest合同，按静态路由生成最小访问审计字段并剥除cookie/authorization/敏感query，不调用外部服务；完成源码与实际合同后由根统一VPS检查。本轮只记录此接续项，不开展下一工作包。

## 持续实施第一轮最终集成事实（2026-10-15）

- 权威access/CSRF、专用委托、固定Origin和代理修复已通过；BuildApplication仍因本域端点、准入和恢复装配blocked。
- 主线实际源码提交：dad2dce；模块分支交付经cherry-pick后SHA已在progress.json映射，后文原分支SHA保留用于溯源。
- 三域逐个合入后各自VPS make docs/gen、14模块readonly build/vet/gofmt与完整make check通过；最终850文件/943函数/372操作/2186类型，生成回传零变化。现有设计检查补核managementPermission与jobManagementGrants集合一致。
- 全部编译/格式/vet/生成检查均在VPS；没有新增测试，也没有运行业务测试、SQL行为验收、服务/真实接口/模型、镜像、部署或push。部分文件与入口继续in_progress/blocked，不把检查当业务验收。
- 下一项以本节和根plan最新工作包为准，后文“等待VPS”等旧说明为过程历史。


## 持续实施第一轮公共集成（待合main）

- 接入identity权威access授权与CSRF，使用真实会话版本和静态action；专用委托密钥独立于服务断言，固定配置Origin，修复流取消时机、重定向与IPv6默认端口。
- 跨服务委托先验本跳服务断言，portal首跳核原正文，后续保留原proof并由服务断言绑定衍生正文；仍须即时核权和资源授权。
- 任务目录新增managementPermission；identity派生job.manage粗能力，jobs按持久owner/type核具体业务权限。
- 来源分支codex/continuous-r1-cycle01、基线325f56d7b2f6c5567c16d7e21f66d5a88a97e342。最后修订尚待VPS，台账新增函数保持in_progress；不新增测试、不运行业务接口/模型/部署，不push。
- 下一项：统一VPS复核与逐模块合并后，接入已实现用例的HTTP绑定；先核各域OpenAPI和平台请求校验，不增加影子协议。


## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- 委托证明期限改为 min(now+10s, 操作期限, 会话期限)，此前沿用 30 秒操作期限导致所有需登录请求签发失败。
- 同源判断：写请求必须带同源 Origin；带会话的读请求未带 Origin 时放行、带了必须同源。入口来源的协议取本地 TLS 或 Caddy 覆盖写入的 X-Forwarded-Proto，修复 TLS 终止后 scheme 恒为 http 导致写请求全拒。
- CSRF 不再拿请求头与自身比较；权威校验为 identity 核对 X-CSRF-Token 与会话绑定（合同只定义该请求头），仅在另有独立 CSRF cookie 时做双提交比对。CSRF 客户端缺少可信操作上下文时报错，不再伪造上下文。
- `NormalizeForwardHeaders` 改为业务头白名单，浏览器提交的身份、转发地址、hop-by-hop 与平台可信头（X-Cost-Class、X-Deadline、X-Cancellation-Id、X-Data-Epoch 等）不再透传。
- `config/service.defaults.json` 与 `deploy/manifest.json` 端口按 ADR0005 端口表更正（监听 8080，依赖 8081/8101–8113）；修正 `NormalizeForwardHeaders` 返回类型编译错误；`BuildInternalProof` 注释改为与设计一致（拥有方/方法由服务断言绑定，nonce 不签名）；补齐 go.sum。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。
- 遗留：合同要求注入 traceparent，平台尚无链路追踪机制，未实现；固定允许 Origin 列表需在平台 ServiceConfig 增加字段，当前以可信代理协议+Host 推导。

## 本次事实

2026-10-15 在分支 `codex/portal-route-gate`、基线 `dc803f1d92f3b9d528ec92e11e929f544bd6a9a1` 完成本域首片，交付提交为 `282051b7e92b39f601136a4972c3ed25732d5ce3`。独占范围始终为 `modules/portal/`，未改根文件、公共契约、生成物、其他模块，未新增测试或真实接口调用。

已实现文件：

- `internal/route_registry.go`：静态路由编译、路径穿越/双重编码拒绝、精确/最长匹配。
- `internal/request_validation.go`：请求体/头/媒体类型/operationId 预算与可信头清洗。
- `internal/origin_guard.go`：同源、常量时间 CSRF 比对和登记的 identity CSRF 客户端适配。
- `internal/auth_proof.go`、`internal/proxy_stream.go`：预授权、短期委托 proof、有界上游转发、背压流回传和失败映射。
- `internal/runtime_state_client.go`、`internal/maintenance_gate.go`：site-config runtime 读取、维护入口门禁和 data-protection 屏障状态。
- `internal/bootstrap_service.go`：权限能力与固定导航纯规则。
- `internal/app/wire.go`、`internal/transport/server.go`、`cmd/portal/main.go`：独立服务装配、健康/就绪、HTTP 生命周期和代理外壳。
- `go.mod`、`go.sum`、`config/service.defaults.json`、`deploy/manifest.json`：本域工程装配资料。

## 检查证据

- JSON 解析：portal contracts、routes、service defaults、manifest、progress 均通过。
- `git diff --check`：通过。
- 隐私/路径扫描：本次新增源码未写入密钥、正文、原始 IP 或私有地址凭据；配置仅使用示例/部署端点和外部 secret 引用。
- `python scripts/check_docs.py`：未通过，原因是并行会话新增文件及本次 portal 新文件尚未由集成会话登记根 `docs/structure/framework.json`；本域未越权修改根登记。
- Go/gofmt/make：本机未安装，未编译、格式化或运行测试。

## 未完成与下一项

`LoadBootstrap`、偏好、SSE、审计、完整 identity 授权上下文、traffic-guard admission、UI 和容器构建仍为 `planned`/依赖阻塞；当前 generic 本域业务入口只返回 `DEPENDENCY_UNAVAILABLE`，不伪造成功结果。

下一项：先由集成会话登记本次实际文件到根结构清单并在具备 Go/VPS 工具链后复核本地编译；之后再单独认领 `LoadBootstrap` 与 identity 授权客户端，完成条件是按 identity/site-config 合同接入并通过对应编译/静态检查。
