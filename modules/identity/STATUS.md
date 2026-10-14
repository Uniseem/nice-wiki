# 身份权限当前进度

更新：2026-10-14。阶段：R1-01 实施中（首个工作包已交付）；业务验收与部署未开始。

- 启动装配组已实现并提交（分支 codex/identity-r1-01，基线 9c667f3580ee，实现提交 533a1239fa8c）：cmd 入口、internal/app 装配、internal/transport HTTP 生命周期、go.mod/go.sum、config/service.defaults.json、deploy/manifest.json、Dockerfile、Makefile；进程模板复用 lifecycle.RunService。
- Serve、Shutdown、Health、Ready 按合同实现，本地与隔离 VPS go build/vet/gofmt 及 make check 通过（台账 verified；Ready 的 schema/journal/epoch 深度校验随业务存储落地）。
- main 与 BuildApplication 仍为 blocked：accounts 子命令、业务存储打开、x-handler 绑定、换向后会话失效与回执、后台循环依赖后续业务函数，缺口已逐条登记在 [progress.json](progress.json)。
- 现网行为事实：无 site-config 依赖地址或其不可达时进程按 DEPENDENCY_UNAVAILABLE 退出（合同语义）；本域尚无业务存储与业务路由，就绪响应 initialized=false、activeDataEpoch=null。
- 其余 19 个业务文件、40 个业务函数仍为 planned；新会话先核对 Git 状态与交接，不重复实现。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
