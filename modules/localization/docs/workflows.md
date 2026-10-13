# 语言、翻译与术语算法与工作流

状态机：Language: disabled→enabled→disabled。Translation: queued→running→review→ready|superseded|failed|cancelled；ready→stale。Unit: untranslated→machine|human；human具有protected布尔值。GlossaryProposal: accepted→applied|rejected。

## LoadRuntime

启动本地化服务。

1. 1 检查语言字典schema
2. 2 校验默认语言唯一
3. 3 加载术语代数
4. 4 注册旧租约恢复

## NormalizeLanguageTag

规范语言标识。

1. 1 拒绝空/长度>35
2. 2 按BCP47解析大小写
3. 3 禁止私用未知UI语言
4. 4 已支持ar/he/fa/ur为rtl其余ltr

## PutLanguage

更新语言启用配置。

1. 1 NormalizeLanguageTag
2. 2 profile仅声明校验不探测模型
3. 3 默认语言不能disabled
4. 4 CAS保存
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## SetLanguageEnabled

启停公开语言。

1. 1 默认语言停用拒绝
2. 2 enabled保存状态
3. 3 disabled取消该语言未完成翻译
4. 4 保留人工译文历史
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## ListLanguages

读取语言列表。

1. 1 公共只启用语言
2. 2 includeDisabled需localization.manage
3. 3 默认语言先其余tag排序
4. 4 禁止返回模型密钥

## HandlePublication

仅发布触发已配置翻译。

1. 1 inbox去重/epoch校验
2. 2 按enabled且automatic且profile已设语言
3. 3 固定源revision和各generation
4. 4 与jobs任务同事务

## CreateTranslation

申请源修订翻译。

1. 1 验证language启用和profile
2. 2 content当前可见
3. 3 固定术语/memory/human代数
4. 4 source/target/generation唯一排队

## FetchTranslationSource

读取翻译源。

1. 1 校验租约
2. 2 content readPublishedRevision
3. 3 hash/schema匹配
4. 4 分块只读不复制草稿

## SplitTranslationUnits

确定性切分翻译单元。

1. 1 沿用content稳定blockId与完整块树
2. 2 inline粗体/链接/代码转编号标签，URL/code值保留在本服务不发模型
3. 3 code块/公式/嵌入不翻译但image alt/caption翻译
4. 4 单块超预算仅table按行拆并保留父blockId，最终还原树。

## PlanIncrementalTranslation

增量选择待翻译单元。

1. 1 源hash没变复用
2. 2 changed/new块先同源hash且≥20字translation memory匹配
3. 3 human/protected源变不覆盖只review
4. 4 未复用块按1000预计输出token均衡顺序分批。

## ResolveGlossary

取得确定性术语快照。

1. 1 固定generation读取
2. 2 document term优先global
3. 3 原词按长度降序避免短词抢占
4. 4 相同scope原词冲突拒绝

## BuildTranslationPrompt

构造有界翻译请求。

1. 1 同文档按顺序批默认1000预计输出tokens，各批尽量均衡且优先标题边界
2. 2 整篇原文固定prefix仅本批标签块翻译，模型窗口不足明确缩上下文
3. 3 前批译文+相邻译文+section title+仅本批出现terms
4. 4 输出blockId完整和格式标签不变。

## RunTranslationBatch

执行统一网关翻译。

1. 1 校验cancel/lease/source计划
2. 2 costClass/background可信继承
3. 3 MG createInference translation
4. 4 解析候选不直接写target

## ValidateTranslationOutput

校验模型输出格式保护。

1. 1 blockId齐全唯一且没有未知ID
2. 2 编号inline标签完整有序/数量正确还原原URL及code
3. 3 足够长原文≥20字译文完全相同且不同语言则失败
4. 4 失败明确触发对半→逐块降级，最终失败用原文并后台显示。

## CommitTranslationBatch

提交翻译防旧覆盖。

1. 1 当前sourceRevision/epoch仍匹配
2. 2 CAS术语/memory/target/human代数
3. 3 protected单元跳过
4. 4 一事务写machine文本/进度

## CompleteTranslation

完成派生翻译manifest。

1. 1 所有计划单元终态
2. 2 protected且源变需review
3. 3 content当前可见再次检查
4. 4 manifest ready并持久事件

## ReadTranslation

公开读取当前译文。

1. 1 enabled语言和content当前公开源验证
2. 2 按当前blockId匹配，新译文未就绪显示上一generation译文但标对应源hash
3. 3 新块无译文显示原文且partial顶栏目标语言提示
4. 4 人工源变保留译文review，撤回文档全部禁止交付。

## EditTranslationUnit

人工编辑并自动保护。

1. 1 权限和插值校验
2. 2 单元CAS
3. 3 origin=human protected=true humanGeneration++
4. 4 取消旧待写批次

## SetUnitProtection

显式启停人工保护。

1. 1 只能人工授权
2. 2 CAS单元
3. 3 humanGeneration++
4. 4 不自动重翻或覆盖现有文本

## CancelTranslation

取消翻译子链路。

1. 1 写取消标志
2. 2 jobs CancelJob
3. 3 MG cancelInference
4. 4 拒绝所有旧lease写入

## PutGlossaryTerm

人工维护术语。

1. 1 BCP47和长度校验
2. 2 同scope原词唯一
3. 3 CAS并generation++
4. 4 事件使受影响机翻stale不改protected单元

## DeleteGlossaryTerm

删除术语规则。

1. 1 校验权限
2. 2 CAS软删除
3. 3 generation++
4. 4 发布受影响范围

## ListGlossary

管理术语列表。

1. 1 权限检查
2. 2 按termId稳定分页
3. 3 固定首屏generation到游标
4. 4 generation改变提示重新查询

## ApplyApprovedGlossaryProposal

应用memory已批准提议。

1. 1 验证callerService=memory且approval签署委托
2. 2 proposalId/hash去重
3. 3 expectedGlossaryGeneration匹配才CAS term
4. 4 发glossary.proposal.applied或rejected终态事件

## HandleGlossaryChange

使机翻版本按术语变化失效。

1. 1 inbox去重
2. 2 只更高generation
3. 3 受影响machine manifest stale
4. 4 protected文本保持并管理review提示

## ValidateInterfaceCatalog

校验界面字符串契约。

1. 1 key必须在固定源catalog
2. 2 placeholders与源逐字一致
3. 3 禁HTML与新增key
4. 4 规范hash

## PutInterfaceString

人工编辑界面文本。

1. 1 校验固定key/placeholder
2. 2 CAS
3. 3 origin=human protected=true
4. 4 更新catalog generation与事件

## TranslateInterfaceCatalog

显式翻译界面缺项。

1. 1 管理授权且profile配置
2. 2 排除protected/未变sourceHash
3. 3 固定catalog generation
4. 4 任务每批16key并输出严格schema

## ReadInterfaceCatalog

读取运行界面字典。

1. 1 language启用
2. 2 按key合并目标与默认builtin
3. 3 插值缺失视INTEGRITY
4. 4 版本相同返回空strings/notModified

## SuggestLanguage

给访客语言建议。

1. 1 explicitPreference或previouslyRejected返回none
2. 2 key=browserLanguages/timeZone/country+enabledLanguageGeneration缓存
3. 3 miss根task localization.suggest使用fast最多500output token且总5秒，timeout规则fallback
4. 4 confidence≥0.8且建议语言不同才提示，不存IP。

## SaveLanguagePreference

保存显式访客语言偏好。

1. 1 验证伪名owner
2. 2 language已启用
3. 3 保存30天TTL
4. 4 不保存IP或浏览轨迹

## TranslateCachedAnswer

接受QA精选翻译命令。

1. 1 callerService=qa且源为active普通或精选cache，不新增人工批准或模型配置权限门槛
2. 2 核对sourceHash和基准answerRevision，已配置根qa.cache-translate fast任务并分别取翻译任务记忆
3. 3 继承原qa rootOperationId/costClass及同一签名admissionPermit到每个MG调用，费用计相同session/network小时作用域
4. 4 异步翻译完成自动ready，发cache-answer.translated，由QA自动保存对应语言答案
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## CommitCacheAnswerTranslation

保存精选翻译供QA审核。

1. 1 源cache answerRevision/hash/epoch匹配
2. 2 引用token和语言格式核验
3. 3 保存ready自动译文不加人工批准
4. 4 发cache-answer.translated引用事件供QA自动存localized答案。

## InvalidateDocumentTranslations

处理源撤回。

1. 1 inbox去重
2. 2 更高可见性代数
3. 3 manifest stale
4. 4 取消子任务并禁止公开读取

## RecoverTranslationLeases

恢复翻译任务。

1. 1 到期lease拒绝写
2. 2 固定source/current和人工代数
3. 3 旧任务superseded
4. 4 MG未知调用不重复消费只读其状态

## PublishOutbox

持久发布翻译事件。

1. 1 租约领批
2. 2 只发布不可变引用和版本
3. 3 jobs ACK才标
4. 4 有限重试三次后留待下轮

## AcquireBarrier

取得本地维护屏障。

1. 1 校验服务身份及superadmin委托或限定RecoveryAuthorization
2. 2 将本地control.db屏障draining并拒绝新业务写
3. 3 屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本
4. 导入必须恢复保护位与源ref，sourceEpoch未激活前译文不能交付，不自动重翻。
5. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。

## ReleaseBarrier

释放已授权维护屏障。

1. 1 校验相同barrierId/operationId恢复许可
2. 2 不允许未决prepared stage直接释放
3. 3 runtime dataEpoch与最终commit/rollback结果一致
4. 4 released后开放该epoch业务写。

## ExportSnapshot

导出安全watermark快照。

1. 1 验证safe屏障及dataEpoch
2. 2 checkpoint本域私有数据并固定revisionWatermark
3. 3 屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本
4. 导入必须恢复保护位与源ref，sourceEpoch未激活前译文不能交付，不自动重翻。
5. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。

## ValidateSnapshot

只读检查候选快照。

1. 1 校验owner/对象SHA256/byteSize且不接受本地任意路径
2. 2 schemaVersion仅支持1否则UNSUPPORTED_SCHEMA
3. 3 检查本域字段/唯一约束/引用/密钥兼容性
4. 4 返回field issues无业务写，任何integrity失败valid=false。

## PrepareSnapshot

隔离准备恢复数据。

1. 1 验证限定restoreId/barrierId/旧新epoch/audience/期限许可及safe屏障
2. 2 重复validate并读可信ObjectRef
3. 3 写独立staging DB/文件且不覆盖control.db、屏障、信任锚
4. 4 staging checksum一致才ready并记录恢复前指针。

## CommitSnapshot

原子激活隔离恢复数据。

1. 1 校验stage属于许可restore且目标epoch匹配
2. 2 safe屏障下禁止业务worker
3. 3 原子切换本域数据指针并持久旧指针
4. 4 committed幂等回执，屏障仍safe等待全域runtime epoch激活。

## RollbackSnapshot

回退未完成全域恢复。

1. 1 验证同一许可restore/barrier
2. 2 读取control保存旧指针
3. 3 原子恢复旧数据且stage rolled-back
4. 4 回执旧dataEpoch并保持safe，重复rollback返回同回执。

## ReuseTranslationMemory

精确翻译记忆复用。

1. 1 字符数≥20配置阈值
2. 2 sourceHash及语言一致且译文正常/当前术语兼容
3. 3 只复用target text保留新blockId来源
4. 4 短块因上下文变化不复用

## ExtractAutomaticGlossary

首次翻译直接抽取入术语表。

1. 1 document/target firstTranslation唯一
2. 2 根task localization.terms使用accurate档max1000output token抽取
3. 3 manual词条优先绝不覆盖，auto冲突保留高version
4. 4 新auto术语直接入库generation++且batchId可撤销。

## RollbackAutomaticGlossaryBatch

整批撤销自动术语。

1. 1 localization.manage
2. 2 CAS generation
3. 3 仅删除source=auto且仍该batch，已转manual绝不删
4. 4 generation++并失效受影响机翻

## PlanTranslationFallback

逐级翻译失败降级。

1. 1 format/unchanged批对半
2. 2 halves仍失败逐块
3. 3 single仍失败保留原文并状态fallback
4. 4 length先减参考context仅1次，terminology提醒重译仅1次，所有新调用独立MG预留。

## CheckTerminologyCompliance

检查规定译法符合度。

1. 1 只检查源出现terms
2. 2 目标包含固定译法或允许语言词形变体
3. 3 第一次不符带提醒重译一次
4. 4 仍不符review不阻止发布已译文

## ReflectTranslation

可选反思润色。

1. 1 enabled=false直接原样
2. 2 精确档对照source提供修改意见再改写两次bounded调用
3. 3 输出仍固定blockIds/inline标签
4. 4 CAS人工保护/source/代数，费用约初译3倍以MG实际账本为准

## ReadCacheAnswerTranslation

读取已完成的自动缓存翻译。

1. 1 callerService=qa
2. 2 源answerRevision精确匹配
3. 3 ready且dataEpoch当前才返回
4. 4 不改变审核策略、不隐式调模型

## RunWorker

主动领取本域持久任务。

1. 1 向jobs claimJob仅领取本域JOB_HANDLERS登记jobType
2. 2 每10秒heartbeatJob续30秒租约并检查cancel/deadline
3. 3 依jobType执行同域已登记函数，所有中间/提交校验lease与epoch
4. 4 completeJob/failJob持久报告，lease过期停止写，未知MG attempt留held不重放。

共通恢复顺序：AcquireBarrier全部域safe→Export或Validate→Prepare全部stage→Commit全部域指针→site-config transitionRuntimeState激活目标dataEpoch→ReleaseBarrier；任一prepare/commit失败在屏障内Rollback已提交域，回旧epoch后才释放。

任务执行由本域RunWorker主动claim/heartbeat/complete，jobs不调用影子execute-task接口。

| jobType | 本域入口函数ID |
|---|---|
| localization.document | localization.FetchTranslationSource |
| localization.ui | localization.TranslateInterfaceCatalog |
| localization.cache-answer | localization.TranslateCachedAnswer |
