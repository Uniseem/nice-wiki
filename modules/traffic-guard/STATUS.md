# 流量风控当前进度

更新：2026-10-15。本轮：持续实施第三轮登录失败保护已交付；完整17函数及追加迁移/生成产物verified，完整服务启动仍blocked，业务验收未执行。

- 基线：`9acad1ab56532521222f592af0d4b3a849792db5`；分支：`codex/traffic-login-protection-r1`。独占本域，未更改根/平台/其他模块。
- 已写BeginLoginAttempt/FinishLoginAttempt、可信PolicyStore注入/打开代次绑定、两个真实HTTP映射和只追加0002迁移。规则见[data](docs/data.md)；并发和终局均短事务，不持锁核验口令。
- login/bootstrap共享滚动15分钟失败窗（账号5/网络20）、固定15分钟锁及并发（账号1/网络8）；30秒预留，过期仅禁首次authenticated，late rejected按Finish受信时间计失败；同终局重放不重复计数或续锁。
- 完整BuildApplication、GetPolicy/UpdatePolicy、控制屏障/恢复/后台与匿名保留清理仍blocked或planned；本组HTTP映射必须由统一ServiceHandler装配，不能裸挂或伪装完整服务。
- 源码提交：`4867a47d4a3765f6ff53568601763c2139a07942`。2026-10-15集成会话VPS合成快照：linux/amd64 Go1.25.5，GOMAXPROCS=8；make docs/gen、14个现有Go模块-mod=readonly build/vet/gofmt、完整make check全部通过（856文件/1047函数/374操作/2222类型），0并发冲突。产物已回传；仅格式/编译/结构证据，未运行业务验收、真实服务或SQL临时验证。
- 全部生成/格式/编译/vet/检查由根会话统一在VPS执行；本机仅阅读和差异检查。未新增测试、样例、fixture、mock或临时验证程序，未运行真实接口、模型、服务、部署或push。

## 前序交付历史

更新：2026-10-15。阶段：R1-13 私有策略库启动恢复切片已实现；`GetPolicy`/`UpdatePolicy` 因契约缺口阻塞，业务准入、挑战、频率、并发、封禁尚未实现、验收或部署。

- 2026-10-15 集成复核修复：迁移改为 go:embed 读取 `0001_domain.sql`，删除内联副本；决定日志读取按条目 ID 聚合，open 后跟 done 不再误判未决。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- R1-12 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`；实现提交：`d597401755cabc77cf31dadf5b9c12260b60b43b`。
- R1-13 实际基线：`588de8f`；实际分支：`codex/analytics-r1-storage-contract-audit`（当前共享工作区分支名与本域不一致）；源码提交：`7aa4456`，随后为并行提交竞争补回迁移辅助文件的提交：`8a628a1`。traffic-guard 目录与 `1a6725d` 的本域内容一致；未修改其他模块文件。
- `internal/policy_store.go` 已实现本域 SQLite 打开（WAL、foreign_keys、busy_timeout=3000ms、单连接）、outbox 公共表初始化、迁移执行、control journal 损坏/未完成条目隔离和资源关闭；`migrations/0001_domain.sql` 已建立 `guard_policy`、幂等回执摘要表、`schema_migrations`。
- `internal/app/wire.go` 现在在继续 fail-closed 前真实打开、迁移并恢复本域策略库；完整 handler、授权客户端、后台循环和恢复回执不足时仍返回 `DEPENDENCY_UNAVAILABLE`，不开放半成品服务。
- `GetPolicy` 阻塞：函数输入要求 `siteId`，但 GET `/v1/policies` OpenAPI 没有 siteId 参数，也未规定从可信 `OperationContext` 推导的规则；缺少策略行时的返回错误也未登记。
- `UpdatePolicy` 阻塞：契约要求调用 identity 授权、写 policy-changed outbox、operationId 幂等，但函数签名未提供客户端/存储注入边界；当前数据契约此前也未登记回执表，已在本域迁移和数据文档预留摘要表，尚未实现绑定函数。
- `internal/policy.go` 已实现 `traffic-guard.ValidatePolicy`：校验整数主体/站点速率、突发量、`burst≤主体速率` 和主体/站点并发 `1..128`，成功只返回规范化策略值，失败返回无敏感详情的 `VALIDATION`；无费用、余额或 secret 字段，也无持久化/外部调用副作用。
- `GuardPolicy` 当前合同未暴露挑战难度或 ban TTL 字段，本次未擅自扩展生成类型、合同或接口；未知字段仍由严格 schema 边界拒绝。
- 本次源码状态为 implemented；验证状态为 not_run，业务验收状态为未执行。未新增测试、样例、快照、E2E、真实接口/模型调用或部署。
- 已通过 `git diff --check`、traffic-guard 合同 JSON 解析；本机缺少 Go、gofmt、make，未运行 Go 编译、vet、格式化或 `make check`。新增 `internal/policy_store.go` 与 `migrations/0001_domain.sql` 仍需集成会话登记到根 `docs/structure/framework.json`；本会话未修改根文件。

逐文件、逐函数状态以 [progress.json](progress.json) 为准；并行会话只改本域，公共资料交集成会话。
