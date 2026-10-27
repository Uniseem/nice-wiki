# 长期记忆、任务映射与记忆助手算法与工作流

状态机：Entry: active→archived；版本修订不可变。Mapping: enabled|disabled。Assistant: queued→running→review→completed|cancelled|failed|superseded。Proposal: pending→approved→applying→applied|rejected|conflicted；拒绝pending→rejected，批准后不自动回滚。

本文件只写状态机、跨流程规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `glossary.proposal.applied` | 1 | `memory.HandleGlossaryProposalResult` | 消费glossary.proposal.applied执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `glossary.proposal.rejected` | 1 | `memory.HandleGlossaryProposalResult` | 消费glossary.proposal.rejected执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `content.publication.changed` | 1 | `memory.CountPublication` | 统计新发布数，达到20篇触发项目概述自动更新 |
| `identity.site.changed` | 1 | `sitestate.RecordSite` | 记下站点状态：后台循环只处理正常站点 |
| `identity.site.purged` | 1 | `sitestate.PurgeSite` | 删掉本域里该站点的全部数据和文件 |

### 异步任务

#### `memory.assistant`

- 执行函数：`memory.RunAssistant`
- 触发：CreateAssistantRun与RecoverAssistantLeases创建

1. 必须受信job绑定runId/inputRef SHA256/rootOperationId/dataEpoch/jobId；模型前及提交前锁外CheckLease，租约未知失败关闭；同op+assistant步骤复用MG持久结果不重新收费。
2. lease/cancel检查
3. 根memory.assistant accurate档max1500output token，可信admin-probe根
4. 严格解析new/update/delete/noop候选和readonly规则
5. 不写active，只保存pending差异候选。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
7. 仅消费CurrentSnapshot.Output.Text非nil的真实完整winning结果，显式空字符串可用；不由历史delta拼接或把费用未知当作没有结果，原费用/paid refs继续保留。

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

## R1 上下文确定性规则（2026-10-18）

映射只读 deleted=0；移除软删除并禁用，同 task/entry/language 的唯一索引只覆盖未删除行。全量排序包含该任务全部未删除映射（含禁用项），无重复无缺项，order=索引×10；一事务递增各映射 revision 和全局 mappingRevision。

一次只读 SQLite 快照固定 current 条目、映射、全局修订和有效预算。目标语言匹配优先，und 回退；条目语言须等于映射语言或 und。相同 entry 去重后按 priority 降序、order 升序、entryId 升序。rendererVersion 为 memory-context-v1：每条依次输出 `<memory-entry `、canonical JSON 对象（entryId/version/title/description）、`>\n`、正文、`\n</memory-entry>\n`，空条目集合输出空字符串。全部完整字节及条目数不得超预算，不截断、不省略。

manifest 记录有序 IDs/versions/实际语言/bodyHash、任务与目标语言、mappingRevision、rendererVersion、有效预算值与 budgetRevision、完整渲染 bodyHash/byteSize。移除 snapshotId/versionHash 后作 canonical JSON SHA256 得到 versionHash，snapshotId=`ctx-`+versionHash；相同输入得到相同版本。预算/语言/bodyHash 新追溯字段在 HTTP 中为兼容可选字段，生产方始终写出。resolve 后独立持久保存只含引用的派生清单，不改变业务代数；管理读取不返回正文。

写入预检使用完全相同选择器和渲染器，投影受影响任务全部已出现语言及 und；候选条目、映射和预算在提交前整体替换，任何语言超预算拒绝保存，错误只列 taskId。

助手创建先免费读取根任务分配，再以可信 admin-probe 费用类读取同 tier 唯一真实 profileId；不把 tier 当 profileId。清原 portal proof，用 memory 纯服务身份保留 site/root/epoch 读取配置。人工请求与全部选中正文总计最多16384字节，最多32条。事务先保存 queued 及不可变输入，恢复循环只用原去重键投递 jobs，不发起模型。执行器必须验证 JobExecutionInput 的 job/root/ref hash/epoch 绑定并在模型前及提交前锁外 CheckLease；模型幂等步骤固定为原 operationId+assistant，失去租约不会另建收费尝试。取消意图先持久化，jobs/MG 取消失败保留待投递；批准项不回滚，pending 候选转 rejected。
