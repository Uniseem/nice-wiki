# 长期记忆、任务映射与记忆助手算法与工作流

状态机：Entry: active→archived；版本修订不可变。Mapping: enabled|disabled。Assistant: queued→running→review→completed|cancelled|failed|superseded。Proposal: pending→approved→applying→applied|rejected|conflicted；拒绝pending→rejected，批准后不自动回滚。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `glossary.proposal.applied` | 1 | `memory.HandleGlossaryProposalResult` | 消费glossary.proposal.applied执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `glossary.proposal.rejected` | 1 | `memory.HandleGlossaryProposalResult` | 消费glossary.proposal.rejected执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `content.publication.changed` | 1 | `memory.CountPublication` | 统计新发布数，达到20篇触发项目概述自动更新 |

### 异步任务

#### `memory.assistant`

- 执行函数：`memory.RunAssistant`
- 触发：CreateAssistantRun与RecoverAssistantLeases创建

1. lease/cancel检查
2. 根memory.assistant accurate档max1500output token，可信admin-probe根
3. 严格解析new/update/delete/noop候选和readonly规则
4. 不写active，只保存pending差异候选。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

#### `memory.overview`

- 执行函数：`memory.UpdateProjectOverview`
- 触发：每日定时；CountPublication累计20篇新发布时创建
- 系统定时：`0 4 * * *`

1. source=auto且autoUpdateEnabled才继续
2. daily或publishedCount≥20触发
3. content当前公开标题摘要固定快照→fast task memory.overview
4. CAS entryVersion/autoUpdateEnabled并预算preflight保存，否则旧结果superseded
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `memory.LoadRuntime` | 启动时 | 启动记忆服务 |
| `memory.RecoverAssistantLeases` | 启动时及每60秒 | 恢复助手执行与批准命令 |
| `memory.DispatchGlossaryCommand` | 启动后持续，空闲每1秒 | 异步发送批准术语命令 |
| `memory.PruneContextSnapshots` | 每日 | 回收无引用派生注入清单 |
| `memory.PublishOutbox` | 启动后持续，空闲每1秒 | 发布记忆引用式事件 |
| `memory.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `memory.RunWorker` | 启动后持续 | 主动领取本域持久任务 |
<!-- flows:end -->
