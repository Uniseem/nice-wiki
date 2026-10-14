# 问答、引用与回答缓存算法与工作流

状态机：Turn: admitted→queued→retrieving→generating→completed；任一步→cancelled|failed|invalidated。Cache: pending→approved→active→stale|disabled。Compaction: queued→running→committed|superseded|failed。Conversation: active→expired|deleted。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `content.publication.changed` | 1 | `qa.InvalidateCache` | 消费content.publication.changed执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `content.publication.withdrawn` | 1 | `qa.InvalidateCache` | 消费content.publication.withdrawn执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `index.retired` | 1 | `qa.InvalidateCache` | 消费index.retired执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `memory.context.changed` | 1 | `qa.InvalidateCache` | 消费memory.context.changed执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `cache-answer.translated` | 1 | `qa.StoreLocalizedCacheAnswer` | 消费cache-answer.translated执行本域失效/任务/批准回执；不同步反向请求生产方。 |

### 异步任务

#### `qa.turn`

- 执行函数：`qa.ExecuteTurn`
- 触发：StartTurn与排队轮次出队时创建

1. 读取轮次与会话，校验租约/取消；每10秒traffic-guard:renewAdmission续并发许可；NormalizeQuestion；ResolveTurnContext固定上下文版本
2. 首轮且问题独立时先LookupExactCache；命中经ValidateCachedAnswer后直接输出（缺目标语言时TranslateCacheEntry）并跳到第5步
3. ResolveCurrentPageHint；PlanQuestion补全/分类/风险（风险交guard）；needs_clarification直接输出澄清；EvaluateCacheEligibility；合格问题LookupExactCache→AcquireQueryEmbedding→LookupSemanticCache，命中经ValidateCachedAnswer输出
4. 未命中：RetrieveEvidence（复用查询嵌入）→InspectEvidence（有限轮次）→BuildAnswerPrompt→GenerateAnswer流式输出→ValidateCitation
5. CompleteTurn释放并发许可；合格答案WriteAutomaticCache；上下文接近窗口时CreateCompaction；失败或取消写终态并释放许可
6. 本会话还有排队轮次时取最早一条，经traffic-guard:queueSessionAdmission换为活动许可后创建下一轮qa.turn任务

#### `qa.compact`

- 执行函数：`qa.RunCompaction`
- 触发：CreateCompaction创建

1. 输入previousSummary加newMessage范围增量更新，不从头重写全部历史
2. fast task qa.compact且不注入长期记忆
3. 继承本轮qa根与费用额度
4. 固定recent retained范围，取消后不提交摘要。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

#### `qa.cache-embedding`

- 执行函数：`qa.BuildCacheEmbedding`
- 触发：ApproveCacheEntry创建

1. 只active且原version
2. ctx costClass=background继承批准根
3. MG生成向量
4. 返回时CAS版本/space/epoch再写
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `qa.LoadRuntime` | 启动时 | 启动问答服务 |
| `qa.RecoverTurns` | 启动时及每30秒 | 恢复崩溃问答 |
| `qa.PruneExpiredRecords` | 每日 | 清理到期记录 |
| `qa.PublishOutbox` | 启动后持续，空闲每1秒 | 发布问答脱敏事件 |
| `qa.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `qa.RunWorker` | 启动后持续 | 主动领取本域持久任务 |
<!-- flows:end -->
