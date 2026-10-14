# 长期记忆、任务映射与记忆助手算法与工作流

状态机：Entry: active→archived；版本修订不可变。Mapping: enabled|disabled。Assistant: queued→running→review→completed|cancelled|failed|superseded。Proposal: pending→approved→applying→applied|rejected|conflicted；拒绝pending→rejected，批准后不自动回滚。

## LoadRuntime

启动记忆服务。

1. 1 检查entry body/hash
2. 2 验证映射taskId允许集合
3. 3 载入固定预算
4. 4 恢复未确认批准命令

## ValidateEntry

验证记忆内容与预算。

1. 1 NFC/LF/frontmatter合法，正文≤entry.maximumBytes默认1536
2. 2 description/title/tag范围检查
3. 3 内容视作可公开，拒密钥/内部客户信息
4. 4 readonly只约束助手，人工直接编辑仍可。

## CreateEntry

创建人工记忆条目。

1. 1 ValidateEntry并人工授权
2. 2 投影所有受影响task注入量默认≤5120bytes否则VALIDATION并列taskId
3. 3 CAS版本后写私有temp/fsync/rename权威Markdown并记录commit manifest，失败恢复旧指针
4. 4 数据库revision索引及outbox提交，人工编辑project overview后autoUpdateEnabled=false。

## UpdateEntry

人工更新记忆。

1. 1 ValidateEntry并人工授权
2. 2 投影所有受影响task注入量默认≤5120bytes否则VALIDATION并列taskId
3. 3 CAS版本后写私有temp/fsync/rename权威Markdown并记录commit manifest，失败恢复旧指针
4. 4 数据库revision索引及outbox提交，人工编辑project overview后autoUpdateEnabled=false。

## ArchiveEntry

归档停止未来注入。

1. 1 权限CAS
2. 2 state archived
3. 3 revision记录
4. 4 context版本改变且已有快照可追溯

## RestoreEntry

恢复归档记忆。

1. 1 读取archived
2. 2 CAS
3. 3 active新version不复写旧revision
4. 4 更新memoryGeneration

## ReadEntry

读取人工记忆或历史。

1. 1 memory.manage
2. 2 缺version取current
3. 3 历史只读
4. 4 body不写访问日志

## ListEntries

管理条目列表。

1. 1 权限
2. 2 稳定entryId keyset
3. 3 limit默认20
4. 4 返回hash及正文仅管理会话

## ListEntryRevisions

读取不可变记忆历史。

1. 1 权限
2. 2 version降序
3. 3 各bodyHash校验
4. 4 不允许历史修改

## PutMapping

维护任务映射。

1. 1 taskId引用根唯一AI task catalog
2. 2 entry存在且语言范围合理
3. 3 投影该task完整注入量≤默认5120bytes否则拒写列taskId
4. 4 CAS映射及global mappingRevision。

## RemoveMapping

移除任务映射。

1. 1 权限CAS
2. 2 软删除mapping
3. 3 mappingRevision++
4. 4 只影响后续context

## ReorderMappings

稳定重排任务记忆。

1. 1 传入集合必须全量无重复
2. 2 CAS全局revision
3. 3 order=索引*10
4. 4 一事务写映射与版本

## ListMappings

读取任务映射。

1. 1 权限
2. 2 taskId允许
3. 3 order/mappingId稳定排序
4. 4 返回全局版本

## PutTaskBudget

配置任务注入上限。

1. 1 bytes1024..65536 entries1..64
2. 2 CAS
3. 3 预算版本进入context hash
4. 4 不扩大模型token额度

## ReadTaskBudget

读取注入预算。

1. 1 验证taskId
2. 2 未自定义使用5120bytes/64entries revision0
3. 3 返回有效预算
4. 4 无外部调用

## SelectContextEntries

按范围确定注入候选。

1. 1 固定mappingRevision读取enabled
2. 2 active条目且language目标优先und
3. 3 priority降序/order/entryId稳定排序
4. 4 相同entry去重

## ApplyContextBudget

确定性截选完整条目。

1. 1 精确计算全部选择条目的标题/description/正文边界UTF8bytes
2. 2 预算≤5120默认且每entry≤其limit
3. 3 超预算INTEGRITY_ERROR拒发不静默截条（写时已preflight）
4. 4 保持mapping顺序所有条目不截断。

## RenderContext

生成防混淆记忆上下文。

1. 1 固定renderer先description再正文带entry/version边界
2. 2 作为system前部任务记忆但不是工具授权
3. 3 精确UTF8bytes≤task budget
4. 4 canonical manifest/text SHA256形成versionHash。

## ResolveContext

跨服务取得版本化记忆上下文。

1. 1 callerService允许taskId
2. 2 一只读快照固定entry/mapping/budget
3. 3 Select→Budget→Render
4. 4 回传公共MemoryContext无额外字段

## ReadContextManifest

查询版本证据清单。

1. 1 memory.manage
2. 2 versionHash命中不可变manifest
3. 3 返回IDs/versions不含正文
4. 4 可追溯预算及renderer

## CreateAssistantRun

显式运行记忆助手。

1. 1 仅memory.manage即可使用已配置助手，不要求model.manage/superadmin且不修改渠道/档位/任务模型分配
2. 2 根memory.assistant映射读取已配置accurate模型，未配置明确禁调用；固定entry versions/memoryGeneration
3. 3 输入≤16384bytes，readonly只建议人工直接编辑
4. 4 可信admin-probe根任务排队，统一MG执行及计费
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## BuildAssistantPrompt

构造只允许提议助手输入。

1. 1 固定指令只输出proposal schema
2. 2 明示无写权限
3. 3 输入记忆作为数据隔离
4. 4 禁生成密钥/IP/会话记录

## RunAssistant

通过MG产生候选提议。

1. 1 lease/cancel检查
2. 2 根memory.assistant accurate档max1500output token，可信admin-probe根
3. 3 严格解析new/update/delete/noop候选和readonly规则
4. 4 不写active，只保存pending差异候选。

## ValidateProposal

验证候选提议闭包。

1. 1 kind对应字段required且其他字段禁止
2. 2 entry_update/archive必须目标及旧version
3. 3 glossary必须目标语言/术语/旧glossaryGeneration
4. 4 无批准人允许pending

## CommitAssistantProposals

提交候选防旧覆盖。

1. 1 当前memoryGeneration一致
2. 2 每提议目标expectedVersion仍一致
3. 3 不一致run superseded
4. 4 pending proposals与run review同事务

## ReadAssistantRun

管理助手提议。

1. 1 memory.manage
2. 2 原创建人或有权限管理员可读
3. 3 返回候选旧版本证据
4. 4 不含密钥/原模型敏感日志

## ApproveProposal

人工批准具体提议。

1. 1 必须actor.kind=admin且memory.manage
2. 2 展示payload hash与版本匹配
3. 3 CAS pending→approved记录approvedBy
4. 4 entry类调用应用事务，glossary写批准command outbox

## RejectProposal

人工拒绝未应用候选。

1. 1 只pending可拒绝
2. 2 CAS revision
3. 3 rejected记录人/理由
4. 4 不影响active记忆

## ApplyEntryProposal

应用批准的记忆内容。

1. 1 必须已人工批准且hash一致
2. 2 readonly=true的助手候选禁止应用仅建议直接人工编辑
3. 3 CAS旧entry version并投影全部映射任务预算
4. 4 原子权威Markdown新对象+manifest与revision索引提交，proposal applied及context.changed同事务。

## EnqueueGlossaryCommand

产生已批准术语命令。

1. 1 校验kind=glossary及admin approval
2. 2 规范ApprovedGlossaryProposal含批准修订/hash
3. 3 唯一proposalId command
4. 4 applying与outbox同事务

## DispatchGlossaryCommand

异步发送批准术语命令。

1. 1 领取独立command租约
2. 2 HTTP请求localization applyApprovedGlossaryProposal
3. 3 同proposal/hash重试最多3次500/1000/2000ms
4. 4 ACK仅accepted，终态等事件

## HandleGlossaryProposalResult

消费术语批准结果。

1. 1 inbox去重
2. 2 proposal/hash/approvalRevision匹配
3. 3 applying→applied或conflicted
4. 4 不请求localization/memory resolve再循环

## CancelAssistantRun

取消未完成助手调用。

1. 1 人工权限
2. 2 写cancel
3. 3 MG cancelInference/jobs取消
4. 4 pending未批准候选标rejected，已批准不回滚

## RecoverAssistantLeases

恢复助手执行与批准命令。

1. 1 到期lease禁止旧写
2. 2 原MG流可续才续
3. 3 sourceGeneration已变superseded
4. 4 command ACK未终态重读事件状态不重复批准

## PublishOutbox

发布记忆引用式事件。

1. 1 领取租约
2. 2 payload只含entry/proposal ID与versionHash
3. 3 jobs持久ACK标published
4. 4 有限重试后保留下一轮

## PruneContextSnapshots

回收无引用派生注入清单。

1. 1 不删除entry历史
2. 2 无活跃任务/审计保留引用才回收
3. 3 验证私有文件范围
4. 4 删除manifest记录

## AcquireBarrier

取得本地维护屏障。

1. 1 校验data-protection服务身份及superadmin委托
2. 2 将本地control.db屏障draining并拒绝新业务写
3. 3 屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令
4. 导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
5. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。

## ReleaseBarrier

释放已授权维护屏障。

1. 1 校验相同barrierId/operationId与data-protection服务身份
2. 2 暂存目录不影响释放；切换只在进程重启时发生
3. 3 runtime处于restore_switching或restore_rolling_back时不恢复业务worker，等待重启换目录
4. 4 released后开放该epoch业务写。

## ExportSnapshot

导出安全watermark快照。

1. 1 验证safe屏障及dataEpoch
2. 2 checkpoint本域私有数据并固定revisionWatermark
3. 3 屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令
4. 导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
5. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。

## StageSnapshot

把本域备份归档暂存到restore/<restoreId>/staged，不触碰active数据。

1. 校验data-protection服务身份与superadmin委托；site-config runtime为restore_staging且restoreId一致；本域restore屏障为safe
2. 经data-protection:serveRestoreArchive按restoreId拉取本域归档，边读边算SHA-256，写入<dataRoot>/restore/<restoreId>/staged.tmp
3. 校验checksum、owner、manifest内文件hash，以及dataSchemaVersion不高于本版本可迁移范围；任一不符删除临时目录并返回INTEGRITY_ERROR或UNSUPPORTED_SCHEMA
4. fsync后改名为staged，在control区记录暂存回执（restoreId、snapshotId、checksum）；active目录与数据库连接保持不变

## DiscardSnapshot

删除某次恢复的暂存目录或恢复完成后换出的旧数据目录。

1. 校验data-protection服务身份与superadmin委托
2. scope=staged：仅当runtime不在restore_switching/restore_rolling_back、且control日志显示本域尚未换入该restoreId时删除restore/<restoreId>/staged
3. scope=before：仅当该恢复已completed或已整体换回、且control日志确认active不是该before时删除restore/<restoreId>/before
4. 删除后在control区记录discarded；目录不存在返回removed=false

## ValidateProjectedBudgets

保存前完整投影注入预算。

1. 1 读固定entry/mapping snapshot
2. 2 应用候选变更到内存投影不持久
3. 3 按每任务/语言完整renderer bytes计算
4. 4 任一超budget返回VALIDATION field issues带taskId不含正文

## UpdateProjectOverview

自动维护项目概述并保护人工修改。

1. 1 source=auto且autoUpdateEnabled才继续
2. 2 daily或publishedCount≥20触发
3. 3 content当前公开标题摘要固定快照→fast task memory.overview
4. 4 CAS entryVersion/autoUpdateEnabled并预算preflight保存，否则旧结果superseded

## AppendAssistantMessage

保存可连续的记忆助手对话。

1. 1 actor admin且memory.manage
2. 2 owner及CAS版本
3. 3 body≤8192bytes且不接受内部密钥
4. 4 追加ordinal消息和30天可调TTL，需整理时另createAssistantRun

## ReadAssistantConversation

读取记忆助手连续历史。

1. 1 owner/admin权限
2. 2 到期NOT_FOUND
3. 3 ordinal keyset分页
4. 4 展示每run对应差异提议不自动应用

## DeleteAssistantConversation

清空助手对话。

1. 1 owner及memory.manage
2. 2 取消未完成关联run
3. 3 删除聊天正文保留批准审计与entry历史
4. 4 同key返回同结果

## RunWorker

主动领取本域持久任务。

1. 1 向jobs claimJob仅领取本域JOB_HANDLERS登记jobType
2. 2 每10秒heartbeatJob续30秒租约并检查cancel/deadline
3. 3 依jobType执行同域已登记函数，所有中间/提交校验lease与epoch
4. 4 completeJob/failJob持久报告，lease过期停止写，未知MG attempt留held不重放。

共通恢复顺序（ADR0003）：AcquireBarrier全部域safe→StageSnapshot全部域暂存→data-protection写切换决定→各服务重启时换入active并登记switched→全部就绪后complete_restore解除维护；暂存失败DiscardSnapshot丢弃后退出维护，切换后失败可整体换回。

任务执行由本域RunWorker主动claim/heartbeat/complete，jobs不调用影子execute-task接口。

| jobType | 本域入口函数ID |
|---|---|
| memory.assistant | memory.RunAssistant |
| memory.overview | memory.UpdateProjectOverview |
| memory.glossary-dispatch | memory.DispatchGlossaryCommand |
