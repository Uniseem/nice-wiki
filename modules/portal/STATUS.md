# 主系统当前进度

最新任务（2026-10-15）：持续实施第三轮portal浏览器认证边界。分支codex/portal-auth-boundary-r1，基线9acad1ab56532521222f592af0d4b3a849792db5，接续公共918dd6369ea7a33bf9296949c8b5441a08d1e88f。仅拥有modules/portal/。

本轮已通过统一VPS与人工复读的源码：固定四个identity认证操作的敏感DTO到Secure HttpOnly SameSite=Strict cookie、安全浏览器session JSON、refresh空输入转内部cookie请求并重算hash、真实peer当前及上一UTC日HMAC签入LoginNetworkScopes、当前设备权威撤销成功才清cookie。补正既存装配假成功：缺真实本域handler时NewApplication失败关闭，Ready不返回固定初始化/恢复成功。

本轮统一VPS linux/amd64 Go1.25.5、GOMAXPROCS=8的make docs/gen、14模块readonly build/vet/gofmt和完整make check均通过（856文件/1047函数/374操作/2222类型），0并发冲突，产物已回传。源码尚待本域提交与真实SHA回填；没有本机格式、生成、编译、vet、类型检查或测试。BuildApplication/Ready及完整ProxyRequest/Serve的剩余绑定继续blocked，不能把本轮认证适配片段当整站可运行。未新增实际文件、测试、fixture或验证程序，未运行服务/真实接口/模型、部署或push。后文为历史记录，逐项以progress.json为准。

最新统一集成事实（2026-10-15）：权威access/CSRF、专用委托、固定Origin和代理修复已通过；BuildApplication仍因本域端点、准入和恢复装配blocked。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：dad2dce，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


本轮持续实施集成分支（未合main）：接入identity权威access授权与CSRF，使用真实会话版本和静态action；专用委托密钥独立于服务断言，固定配置Origin，修复流取消时机、重定向与IPv6默认端口。
VPS上一快照14模块build/vet/gofmt与make check通过；委托二跳与代理错误映射的最后修订等待最终复跑。业务验收和部署未执行。

更新：2026-10-15。阶段：R1 首片已实现，尚未编译或行为验收。

- 2026-10-15 集成复核修复：委托证明期限改为 min(now+10s, 操作期限, 会话期限)，此前沿用 30 秒操作期限导致所有需登录请求签发失败。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次本地分支：`codex/portal-route-gate`；基线：`dc803f1d92f3b9d528ec92e11e929f544bd6a9a1`。
- 交付提交：`282051b7e92b39f601136a4972c3ed25732d5ce3`（本地，未 push）。
- 已实现：静态路由索引/最长匹配、请求预算与可信头、Origin/CSRF 边界、委托 proof 签发、runtime 读取与维护门禁、入口屏障纯状态、HTTP 生命周期、portal 独立 Go module/配置/manifest。
- 后台壳和代理装配已建立：匿名健康/就绪可注册；路由表外返回 404；上游地址只来自配置；请求体、响应头和流式回传有界；未装配的身份授权上下文、偏好、bootstrap、SSE、审计接口 fail-closed。
- 未完成：`LoadBootstrap`、偏好/SSE/审计、完整 identity 授权上下文和 owning-service handler 装配、traffic-guard admission、UI、Dockerfile/Makefile；对应函数仍为 `planned`，跨域依赖未猜测实现。
- 业务编译、gofmt、VPS 检查、普通测试、E2E、真实接口、部署和行为验收均未执行。本机没有 Go/gofmt/make 工具链。
- 本域未改根文件、公共 contracts、生成物或其他模块；未 push。

逐文件/逐函数状态以 [progress.json](progress.json) 为准；实现状态不等于验证或业务验收通过。
