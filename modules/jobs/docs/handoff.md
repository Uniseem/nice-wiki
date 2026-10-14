# 任务与事件会话交接

## 本次事实（R1-05 CreateJob，2026-10-15）

- 本次工作包按上一会话 handoff 的下一项执行；实际共享工作区分支为 `codex/analytics-r1-storage-contract-audit`，基线：`32afbc58bd24b8d03230c920a3b105374c783acb`。
- 实现提交：`588de8fa991340df7dcf019f8c11849f85e1b796`（`jobs：实现任务提交持久化`）；未 push。
- 独占范围：`modules/jobs/`。本次代码提交只包含 `internal/queue.go`、`migrations/0001_domain.sql`、`go.mod` 和 `go.sum`；本交接及台账随后单独同步。未修改根文件、公共合同、生成物、libs、deploy 或其他模块。

## 本次完成与边界

- `jobs.CreateJob` 已按 binding 合同实现：先调用既有 `ValidateJobSpec`，再验证可信 `op.OperationID` 与 `CreateJobInput.dedupeKey` 的 UTF-8/8192字符上限。
- 本域私有 SQLite 已实现 WAL、foreign_keys、busy_timeout=3000ms、单写连接和迁移；事务按 `(owner, dataEpoch, dedupeKey)` 查重，同 inputHash 返回原 `jobId/operationId`，不同 inputHash 返回 `CONFLICT`。
- 新任务以 `queued`、attempt=0、revision=1 持久化，并在相同事务用 `libs/platform/outbox.EnqueueEvent` 登记 `jobs.job-state-changed.v1`。事件只含安全任务引用和状态，不含输入正文；检测到 `barriers.state=draining` 时返回 `LOCKED`。
- `OpenDatabase` 与 `InstallStore` 已提供给后续 BuildApplication 装配；当前 BuildApplication 仍因全部 x-handler、租约、事件/死信、定时、屏障、后台循环和激活回执未完成而按合同返回 `DEPENDENCY_UNAVAILABLE`，没有伪造 ready。
- 迁移文件建立 jobs、leases、progress、job_results、events、deliveries、schedules、barriers、schema_migrations；`nw_outbox` 由 platform 机制幂等建表。没有读取或复制所属服务对象正文。

## 检查与验收

- 已核对初始工作区与本域文档/四份合同；代码提交暂存范围仅为 `modules/jobs/`。共享工作区后来出现的 `modules/site-config/`、`modules/traffic-guard/` 脏改动未触碰、未暂存、未提交。
- 已运行 `git diff --cached --check`，返回码 0；已运行 `python scripts/check_docs.py --privacy-only`，返回码 0。
- 已用 Python `sqlite3` 内存数据库执行 `migrations/0001_domain.sql`，插入一条 queued 任务成功；这不是新增测试样例，也未写入仓库运行数据。
- 完整 `python scripts/check_docs.py` 返回码 1：公共 `docs/structure/framework.json` 尚未登记本域新增实际文件，且存在范围外 site-config 台账脏改动；按本工作包不能修改公共文件或其他模块。
- 当前环境无 `go`、`gofmt` 和 `make`，未执行 Go 编译、vet、gofmt、模块检查或业务行为验收；函数源码状态为 `implemented`，`verified` 和 `accepted` 均未登记。

## 下一步

下一工作包优先实现 `ClaimJob`、`StartJob`、`HeartbeatJob`、`CheckLease` 的租约 fencing，并复用本次 jobs 表、dataEpoch 和迁移边界；之后再接完整 OpenAPI handler 和 BuildApplication。不要修改公共 `framework.json`，由集成会话统一登记本次实际文件。
