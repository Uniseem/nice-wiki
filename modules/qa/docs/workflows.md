# 问答、引用与回答缓存算法与工作流

状态机：Turn: admitted→queued→retrieving→generating→completed；任一步→cancelled|failed|invalidated。Cache: pending→approved→active→stale|disabled。Compaction: queued→running→committed|superseded|failed。Conversation: active→expired|deleted。

## LoadRuntime

启动问答服务。

1. 1 载入schema/policy
2. 2 检查SSE事件序号索引
3. 3 恢复取消与终态
4. 4 不恢复派发未知MG调用

## PutQaPolicy

管理公开问答策略。

1. 1 限制各token预算
2. 2 embedding space与MG/knowledge匹配
3. 3 semantic阈值≥0.94
4. 4 CAS更新并失效scope

## CreateConversation

创建伪名访客会话。

1. 1 验证guard签名action与subject
2. 2 不保存IP
3. 3 生成会话ID及30天过期
4. 4 同visitor/key返回同会话

## GetConversation

读取自有访客历史。

1. 1 校验ctx.actor.subjectId与visitorSubject
2. 2 过期返回410
3. 3 ordinal分页
4. 4 只返回本会话消息

## StartTurn

准入后开始问答轮次。

1. 1 owner及guard permit验证
2. 2 question≤500Unicode字符并固定可信qa上下文
3. 3 同会话最多1活跃轮次，其余请求持久排队最多5项按顺序执行
4. 4 写原问题/当前页hint/queued状态，前置仅收权威校验提示。

## ValidateAdmission

检查准入绑定。

1. 1 验签及expiresAt
2. 2 action/requestId/subjectId匹配
3. 3 guard消费一次凭证
4. 4 不接受客户端costClass

## NormalizeQuestion

稳定规范问题和cache锚点。

1. 1 NFKC/LF/全半角空白规范化
2. 2 自然语言大小写折叠但代码/产品ID/数字/否定词保留
3. 3 ≤500Unicode字符且≤8192bytes
4. 4 stable算法提取锚点并输出hash。

## ResolveTurnContext

固定回答上下文版本。

1. 1 读取当前policy/dataEpoch
2. 2 memory resolveContext taskId=qa.answer
3. 3 固定mapping/versionHash
4. 4 canonical hash形成本轮上下文基线

## LookupExactCache

查询精确精选缓存。

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1 complete=false绝不查询
3. 2 第一轮已有standalone exact条目可在前置前命中，追问须PlanQuestion补全后才查
4. 3 优先curated再未过期普通缓存
5. 4 精选豁免memory版本但全部来源仍需ValidateCachedAnswer。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## LookupSemanticCache

保守查询语义缓存。

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1a 默认AI同义判断分别memory:resolveContext taskId=qa.cache-match，不复用qa.answer；threshold模式不额外取模型task记忆。
3. 1 plan.complete/cacheEligible必须true
4. 2 同space/base canonical范围cosine≥0.8最多3项
5. 3 默认fast task qa.cache-match判断可直接同义回答，禁止rerank替代
6. 4 threshold模式用配置阈值及否定实体锚点保护
7. miss返回原embedding供knowledge复用。
8. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## ValidateCachedAnswer

验证缓存仍可公开交付。

1. 1 active且未过TTL，curated免TTL
2. 2 content校验current/visible且所有块hash相同
3. 3 普通cache校验qa.answer memoryVersionHash，curated豁免仅该项
4. 4 无引用答案在任一发布revision变化即stale，依赖不可用不得返回。

## RetrieveEvidence

取有源可见证据。

1. 1 knowledge searchKnowledge按固定space
2. 2 最多8块/4096token
3. 3 对IDs调用readSections
4. 4 映射可公开Citation且保留ref

## BuildAnswerPrompt

构造有界回答提示。

1. 1 任务记忆在固定system prefix最前，各条带description，任务指令随后
2. 2 summary/最近对话/当前页验证片段/检索材料分别标数据边界
3. 3 访客语言回答、优先对应块译文且引用当前语言href
4. 4 无证据明确文档未提及，固定最大1500输出tokens。

## GenerateAnswer

执行流式答案调用。

1. 1 校验lease/cancel/deadline
2. 2 ctx继承root/costClass=qa
3. 3 MG createInference
4. 4 消费原流并记录invocationId不重放未知请求

## ValidateCitation

逐条保护引用。

1. 1 citationId必须在白名单
2. 2 content当前visible/matchesCurrent
3. 3 href由public route规则生成
4. 4 缺失返回STALE_RESULT并停止该证据回答

## AppendAnswerEvent

持久流式输出事件。

1. 1 租约/cancel状态检查
2. 2 delta≤16KiB且UTF8完整
3. 3 事务递增序号和事件
4. 4 提交后广播

## ResumeAnswerStream

SSE断线重连。

1. 1 会话owner授权
2. 2 读cursor后事件
3. 3 每15秒heartbeat无持久序号
4. 4 超24小时保留期返回410

## CompleteTurn

原子结束可见答案。

1. 1 content终态前再validate全部引用
2. 2 校验cancel及lease
3. 3 事务assistant message/answerRevision/completed事件
4. 4 释放guard并发permit

## CancelTurn

取消问答及所有子调用。

1. 1 owner校验
2. 2 写cancel标记
3. 3 MG cancelInference与jobs取消
4. 4 发布cancelled并释放guard slot但MG费用依其证据

## RecordFeedback

保存访客单条反馈。

1. 1 owner及completed校验
2. 2 answerRevision准确匹配
3. 3 每visitor/turn唯一upsert
4. 4 审计只传评分无问题正文

## ProposeCacheEntry

人工创建精选候选。

1. 1 qa.manage授权
2. 2 completed且引用有效
3. 3 清理访客身份/私密上下文
4. 4 保存pending不会命中

## ApproveCacheEntry

批准缓存公开内容。

1. 1 qa.manage人工编辑后标curated
2. 2 核对source引用有效
3. 3 curated免TTL与未来memory变化但仍受引用失效
4. 4 active后exact优先，semantic embedding另建。

## BuildCacheEmbedding

为已批准缓存构建语义向量。

1. 1 只active且原version
2. 2 ctx costClass=background继承批准根
3. 3 MG生成向量
4. 4 返回时CAS版本/space/epoch再写

## InvalidateCache

失效受版本变化影响缓存。

1. 1 inbox按eventId去重并检查epoch
2. 2 引用变化/撤回/删除使全部受影响cache含精选stale；memory变化仅匹配curated=false和旧memoryVersionHash，curated=true豁免；回答生成模型变化不自动失效
3. 3 active→stale仅作用于上述筛选集；TTL仅普通cache；无引用答案在任一新发布时失效
4. 4 删除失效向量可命中索引，保留审核历史与账单引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## DisableCacheEntry

人工停用缓存。

1. 1 校验qa.manage
2. 2 CAS版本
3. 3 active/pending/stale均可disabled
4. 4 禁用向量命中

## ListCacheEntries

管理精选缓存。

1. 1 权限检查
2. 2 稳定cacheId分页
3. 3 返回人工审核字段
4. 4 不关联公开访客身份

## TranslateCacheEntry

请求精选答案翻译。

1. 1 命中cache缺目标语言才发localization translateCachedAnswer
2. 2 快速档task qa.cache-translate且可信qa costClass继承
3. 3 翻译返回核对基准answerRevision与引用token
4. 4 自动保存localizedAnswer无新增人工批准流程。

## CreateCompaction

申请有界历史摘要。

1. 1 取preflight与answer模型较小context窗口
2. 2 总输入达到90%阈值才自动压缩
3. 3 原样保留最近15000token，小窗口retain=min(15000,可用历史预算)
4. 4 固定前summary与未压缩message范围，唯一conversation/sourceGeneration排队。

## RunCompaction

执行历史压缩。

1. 1 输入previousSummary加newMessage范围增量更新，不从头重写全部历史
2. 2 fast task qa.compact且不注入长期记忆
3. 3 继承本轮qa根与费用额度
4. 4 固定recent retained范围，取消后不提交摘要。

## CommitCompaction

保护新历史不被旧摘要覆盖。

1. 1 比较historyGeneration
2. 2 不相等标superseded
3. 3 CAS summary generation
4. 4 保存hash和输入范围

## CreateDebugTurn

隔离管理诊断。

1. 1 qa.manage及model.manage委托验证
2. 2 专用namespace和costClass=admin-probe
3. 3 使用真实发布证据且禁cache
4. 4 7天TTL和审计

## ReadDebugTurn

读取管理诊断。

1. 1 仅创建管理员或superadmin
2. 2 独立namespace查询
3. 3 过期410
4. 4 不查询公开cache/message表

## DeleteConversation

删除自有对话数据。

1. 1 owner校验
2. 2 取消活动turn
3. 3 删除消息/summary/反馈正文
4. 4 保留无正文审计与MG账单引用

## RecoverTurns

恢复崩溃问答。

1. 1 读取过期lease
2. 2 MG原invocation状态/流可续才续
3. 3 未派发阶段可继续
4. 4 已派发无法确认则failed不重调用

## PublishOutbox

发布问答脱敏事件。

1. 1 领取outbox租约
2. 2 只发IDs/评分/耗时/状态
3. 3 jobs持久ACK后标记
4. 4 三次有限重试后留待下一轮

## PruneExpiredRecords

清理到期记录。

1. 1 会话30天且无活动turn
2. 2 debug7天
3. 3 SSE24小时且终态
4. 4 跳过审核cache和费用引用

## AcquireBarrier

取得本地维护屏障。

1. 1 校验data-protection服务身份及superadmin委托
2. 2 将本地control.db屏障draining并拒绝新业务写
3. 3 屏障停止新turn/cache审批/压缩提交
4. 取消活动turn并持久终态和原MG调用引用
5. 导入的visitor cookie不得恢复授权，恢复dataEpoch使旧流/permit失效，原消息只经重新授权可读。
6. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。

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
3. 3 屏障停止新turn/cache审批/压缩提交
4. 取消活动turn并持久终态和原MG调用引用
5. 导入的visitor cookie不得恢复授权，恢复dataEpoch使旧流/permit失效，原消息只经重新授权可读。
6. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。

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

## ResolveCurrentPageHint

验证当前页面上下文。

1. 1 hint不是权限证据
2. 2 content current/visible验证
3. 3 读取标题及被指代小节≤2048tokens
4. 4 页面已撤回/旧revision丢弃hint不扩大可见范围

## PlanQuestion

补全问题、分类、风险与缓存资格。

1. 1 memory:resolveContext taskId=qa.preflight取项目概述，不复用qa.answer记忆
2. 2 fast qa.preflight最多500output tokens结合摘要/历史/经验证当前页补全基准语言问题
3. 3 严格分类/complete/risk，最多4子问题；仍缺指代needs_clarification且禁cache
4. 4 风险交guard，QA不自行封禁IP
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## EvaluateCacheEligibility

严格缓存完整问题门。

1. 1 必须plan.complete
2. 2 独立canonicalQuestion不残留这/它/第二步未命名来源
3. 3 private/dynamic上下文拒绝共享
4. 4 out_of_scope不写可回答cache

## AcquireQueryEmbedding

同查询只生成一次嵌入。

1. 1 本turn根queryHash/space singleflight
2. 2 knowledge createQueryEmbedding产一次向量
3. 3 校验queryHash/space/vectorHash/dataEpoch
4. 4 同结果供semantic cache和searchKnowledge，cache miss不重新发嵌入。

## InspectEvidence

快速检索员有限核查。

1. 1 读取本turn持久检索轮数/工具预算
2. 2 委托knowledge inspectEvidence而非直接调用其拥有的MG task
3. 3 校验返回selected IDs仍属于原证据引用
4. 4 原子更新轮数/tool计数，超3轮5次使用已收集材料停止。

## WriteAutomaticCache

按old语义自动保存合格答案。

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1 complete/cacheEligible且completed
3. 2 当前反馈unhelpful禁止写且后续负反馈使普通cache失效
4. 3 固定引用hash/memory/space及7天TTL
5. 4 canonical问题基准语言键，访客语言答案子记录，curated=false
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## StoreLocalizedCacheAnswer

保存缓存的多语言回答。

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1 caller localization持久结果引用验证
3. 2 CAS sourceAnswerRevision
4. 3 引用token不变且source仍有效
5. 4 localized answer自动保存并可下次命中
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## ReadCacheCalibration

缓存阈值校准。

1. 1 qa.manage
2. 2 同space分桶，记录只有similarity/feedback非IP
3. 3 选误命中率≤1%的最小阈值且样本<100保留配置
4. 4 输出建议供人工选择不自动改设置

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
| qa.answer | qa.ResolveTurnContext |
| qa.compact | qa.RunCompaction |
| qa.cache-embedding | qa.BuildCacheEmbedding |
