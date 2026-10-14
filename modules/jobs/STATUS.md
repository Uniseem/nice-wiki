# 任务与事件当前进度

更新：2026-10-15。阶段：R1-03 启动与契约生成准备实施中；业务验收与部署未开始。

- 本次会话在独立 worktree 分支 `codex/jobs-r1-03` 领取 jobs 启动装配工作包，基线为 `ac593c57861d2242f27d07562a2acae535d18c38`。
- 已创建合同登记的 `cmd/jobs/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`go.sum`、`Makefile`、`Dockerfile`、`config/defaults.json` 和 `deploy/manifest.json`。
- 入口已接入 `libs/platform/lifecycle.RunService`；装配层已实现 module/目录/信任锚/签名引用/runtime/恢复换目录前置检查。缺少完整任务、事件、死信、定时、屏障、存储及全部 OpenAPI handler 时按合同返回 `DEPENDENCY_UNAVAILABLE`，不启动不完整服务。
- `Serve`、`Shutdown`、`Health`、`Ready` 的生命周期骨架已实现；Ready 在业务 handler 未完整绑定时明确返回 `DEPENDENCY_UNAVAILABLE`。这些代码尚未因 Go 工具链缺失而完成编译、vet、gofmt 或运行验证。
- 默认配置和部署清单仅包含脱敏端点、数据目录和外部 secret 引用；没有新增测试、样例、真实接口、真实模型调用或部署操作。业务验收仍为0。
- 当前环境无 `go`、`gofmt` 和 `make`，无法按 VPS 优先规则执行 Go 构建检查；静态 JSON/合同/隐私检查结果和剩余阻塞见 [docs/handoff.md](docs/handoff.md) 与 [progress.json](progress.json)。

公共契约、根状态、scripts、libs、deploy 拓扑和其他模块均未修改；并行工作只改本域，公共资料交集成会话。
