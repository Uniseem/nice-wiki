# 主系统会话交接

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
