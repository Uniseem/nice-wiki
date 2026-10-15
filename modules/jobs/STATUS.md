# 任务与事件当前进度

最新集成结果：隔离VPS的14模块make build与完整make check通过；本轮已完成条目转verified，实际主分支SHA见progress.json。部分入口/用例仍未实现，业务验收和部署未开始。下列本机与待登记说明保留为交付时历史事实。

更新：2026-10-15。阶段：R1-03租约fencing与可选心跳进度已实现并通过编译门槛；完整HTTP/启动装配、重试回收、事件投递/死信、业务验收与部署尚未完成。

- 本轮独占 `modules/jobs/`，分支 `codex/jobs-lease-r1`，基线 `00dd199239aba487bceb5227aac52f27ce949d74`；实现提交 `e6e0d81226358a11a5764ee9ebb43a6706810e9e`，未push。
- 已实现 `ClaimJob`、`StartJob`、`HeartbeatJob`、`CheckLease`：owner/epoch/worker/fence隔离、到期拒绝、30秒租约、owner在途上限8、批次默认10及最多10、持久fence计数、领取/心跳幂等回执、状态outbox。
- `AppendProgress`随租约包实现为同事务参考函数；心跳可选报告安全阶段代码与计数，checkpointKey/inputHash去重，seq单增且同phase不能倒退。续租、进度、事件和回执一起提交或回滚。
- `LeaseRuntimeProvider`要求装配层提供可信runtime与control屏障安全点/截止；缺少钩子、维护缺有效截止时失败关闭。租约组不继续使用错误放在active库的旧barriers表；全量屏障布局留后续专包收口。
- 本机Go1.27.1根 `make build` 通过（14模块只读依赖build/vet/gofmt），`make docs`和`make privacy`通过。`make check`仅有新增3个实际文件待公共framework登记，其余合同、类型、引用、可达性、权限和隐私检查通过；台账暂记implemented，待集成完整检查后升级verified。
- OpenAPI补CheckLease必填workerId/fencingToken查询参数、claim可省略limit及heartbeat可选progress；生成目录未手改，由集成会话统一生成。平台JobsClient的workerId支持由集成会话公共提交提供。
- 未新增测试/样例/临时验证程序，未运行真实接口、业务进程、模型、联调或部署；完整BuildApplication仍按未装配依赖返回不可用，业务验收未登记完成。

## 既有工作记录


更新：2026-10-15。阶段：R1-05 `CreateJob` 任务提交持久化已实现；租约、事件投递/死信、完整装配、业务验收与部署未开始。

- 2026-10-15 集成复核修复：`ValidateJobSpec` 白名单改读 make gen 生成的 `internal/gen/job_types.gen.go`（与 config/job-types.json 同源）。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次会话按 handoff 下一项实现 `jobs.CreateJob`；实际分支为 `codex/analytics-r1-storage-contract-audit`，基线 `32afbc58bd24b8d03230c920a3b105374c783acb`。
- 实现提交为 `588de8fa991340df7dcf019f8c11849f85e1b796`（`jobs：实现任务提交持久化`），仅包含 `modules/jobs/` 内的代码、迁移和依赖锁定变更，未 push。
- `internal/queue.go` 已实现：复用 `ValidateJobSpec` 的 owner/type/ObjectRef/epoch 校验，补充 operationId 与 dedupeKey UTF-8/8192字符约束；按 `(owner, dataEpoch, dedupeKey)` 查询，同 inputHash 复用原任务，不同输入返回 `CONFLICT`。
- 同一 SQLite 写事务插入 queued 任务和 `jobs.job-state-changed.v1` 初始 outbox 事件；事件载荷只含任务引用、状态、修订和 owner，不含对象正文、提问、密钥或 IP。排空屏障拒绝新任务并返回 `LOCKED`。
- `migrations/0001_domain.sql` 已建立 jobs、leases、progress、job_results、events、deliveries、schedules、barriers 和 schema_migrations；`OpenDatabase` 使用 WAL/foreign_keys/busy_timeout，并调用 platform outbox 建表。数据库入口尚未接入完整 BuildApplication，避免在缺少全部 handler 时伪造 ready。
- `modernc.org/sqlite v1.59.0` 已登记到本域 go.mod/go.sum；Go 工具链不可用，未完成 Go 编译、gofmt、vet 或运行验收。已通过 `git diff --cached --check`、隐私检查，并用 Python sqlite3 内存数据库执行迁移插入校验。
- 完整 `python scripts/check_docs.py` 仍因公共 `docs/structure/framework.json` 未登记本域新增实际文件，以及范围外 site-config 脏改动而失败；公共文件按本工作包范围未修改。业务行为验收仍为0，未新增测试/样例、真实接口、真实模型或部署。

前置启动骨架、`ValidateJobSpec`、`Serve`、`Shutdown`、`Health`、`Ready` 的既有状态与未验证限制保持不变。下一项优先实现 `ClaimJob`/租约 fencing，之后再接入完整存储与 handler 装配。

公共契约、根状态、scripts、libs、deploy 拓扑和其他模块均未修改；并行工作只改本域，公共资料交集成会话。
