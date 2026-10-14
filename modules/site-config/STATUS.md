# 站点配置与初始化当前进度

更新：2026-10-14。阶段：R1-02 实施中（首个工作包已交付）；业务验收与部署未开始。

- 启动装配组已实现并提交（分支 codex/site-config-r1-02，基线 ffe1ae4ccbb，实现提交 5ea6affabea）：cmd 入口、internal/app 装配、internal/transport HTTP 生命周期、go.mod/go.sum、config/service.defaults.json、deploy/manifest.json、Dockerfile、Makefile；进程模板复用 lifecycle.RunService。
- 与 identity 的关键差异：本域即运行状态权威，进程模板与 Ready 经本地 localRuntime 读自身控制库，不做 HTTP 自调用；控制库（独立 control.db）随 runtime_state_service 工作包落地，落地前读取如实返回 DEPENDENCY_UNAVAILABLE，进程按合同退出，不用占位数据冒充权威。
- main、Serve、Shutdown、Health、Ready 按合同实现，本地与隔离 VPS go build/vet/gofmt 及 make check 通过（台账 verified；Ready 的 schema/journal/epoch 深度校验随业务存储落地）。
- BuildApplication 仍为 blocked：运行控制库读取、业务存储打开与迁移、x-handler 绑定、换向后 RegisterActivation 回执、RecoverAnnouncementJournal/RelayOutbox 后台循环依赖后续业务函数，缺口已逐条登记在 [progress.json](progress.json)。
- 现网行为事实：本域运行控制库未实现前进程装配按 DEPENDENCY_UNAVAILABLE 退出（合同语义），readyz 同理返回 503；本域尚无业务存储与业务路由。
- 其余 25 个业务文件、40 个业务函数仍为 planned；新会话先核对 Git 状态与交接，不重复实现。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
