# 任务与事件当前进度

更新：2026-10-15。阶段：R1-04 `ValidateJobSpec` 参考级纯校验已实现；业务验收与部署未开始。

- 本次会话在独立 worktree 分支 `codex/jobs-r1-04-validation` 领取 `jobs.ValidateJobSpec` 单项任务，基线为 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`。
- 已在 `internal/validation.go` 实现纯校验：owner/type 精确白名单、可信调用服务归属、ObjectRef owner/ID/version/hash/size/mediaType、输入 epoch 与可信操作上下文 epoch 比对；不写任务状态、不读取对象正文、不调用外部服务。
- `ValidateJobSpec` 实现提交为 `03ac7eff1ed562be4fdc18794080ae50ecfd900c`；去重键不属于本函数 `ValidateJobSpecInput`，其规则缺口已在本模块交接中记录，待 `CreateJob` 入口按 `CreateJobInput` 合同校验。
- 启动骨架实现提交为 `dba434eb1e03b1a183d2f8b66adf0eff8f3e2bbd`；台账与交接记录随后单独同步，未 push。
- 已创建合同登记的 `cmd/jobs/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`go.sum`、`Makefile`、`Dockerfile`、`config/defaults.json` 和 `deploy/manifest.json`。
- 入口已接入 `libs/platform/lifecycle.RunService`；装配层已实现 module/目录/信任锚/签名引用/runtime/恢复换目录前置检查。缺少完整任务、事件、死信、定时、屏障、存储及全部 OpenAPI handler 时按合同返回 `DEPENDENCY_UNAVAILABLE`，不启动不完整服务。
- `Serve`、`Shutdown`、`Health`、`Ready` 的生命周期骨架已实现；Ready 在业务 handler 未完整绑定时明确返回 `DEPENDENCY_UNAVAILABLE`。这些代码尚未因 Go 工具链缺失而完成编译、vet、gofmt 或运行验证。
- 默认配置和部署清单仅包含脱敏端点、数据目录和外部 secret 引用；没有新增测试、样例、真实接口、真实模型调用或部署操作。业务验收仍为0。
- 当前环境无 `go`、`gofmt` 和 `make`，无法按 VPS 优先规则执行 Go 构建、格式检查或模块检查；已运行静态隐私检查，未运行行为验收。检查结果和剩余阻塞见 [docs/handoff.md](docs/handoff.md) 与 [progress.json](progress.json)。

公共契约、根状态、scripts、libs、deploy 拓扑和其他模块均未修改；并行工作只改本域，公共资料交集成会话。
