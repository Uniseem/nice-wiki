# 统一模型网关算法与工作流

状态机：Invocation: accepted→reserved→dispatching→streaming→succeeded|failed|cancel_requested→cancelled|charge_unknown；charge_unknown→reconciled。Reservation: held→settled|released；未知账单保持held。Provider: disabled|enabled。

## LoadRuntime

启动并验证本服务私有持久化与密钥引用。

1. 1 只读取本服务配置和迁移版本
2. 2 校验masterKeyRef可取32字节密钥
3. 3 注册恢复调度
4. 4 所有关键依赖可用才ready

## ValidateProvider

验证供应方配置。

1. 1 解析URL并按管理员可信network allowlist验证目标
2. 2 外部供应方仅HTTPS，显式本地部署allowlist可HTTP且不允许重定向到其他网段
3. 3 拒绝URL userinfo/fragment及危险附加Authorization重复头
4. 4 固定连接/总/流空闲超时。

## PutProvider

CAS保存供应方。

1. 1 校验配置
2. 2 锁provider行
3. 3 比较版本
4. 4 写配置和审计outbox后提交

## SetProviderEnabled

启停新调用入口。

1. 1 读取供应方
2. 2 CAS版本
3. 3 disabled禁止新路由
4. 4 保留正在执行调用记录

## PutCredential

加密存储密钥。

1. 1 禁止日志请求体
2. 2 读取部署主密钥
3. 3 用随机96bit nonce与providerId/version AAD加密
4. 4 事务写密文并擦除临时缓冲

## RotateCredential

轮转新请求使用的密钥。

1. 1 加密新secret
2. 2 CAS替换active版本
3. 3 已派发调用保留旧版本引用
4. 4 最后引用释放后销毁旧密文

## PutModel

保存不可变模型身份版本。

1. 1 核查provider存在
2. 2 校验能力与维度
3. 3 供应方模型版本/预处理变化创建新modelId
4. 4 CAS保存

## ValidateModel

纯声明验证模型能力。

1. 1 校验能力非空且在允许集合
2. 2 embedding维度1..65536
3. 3 chat/translation输出上限≥1
4. 4 计算嵌入身份

## PutProfile

管理AI档位与费用限制。

1. 1 五档按能力校验
2. 2 orderedModelIds保留管理员顺序且去重最多16个
3. 3 视觉未设可用fast中支持vision模型，rerank未设可用fast
4. 4 CAS保存，仅superadmin可分配。

## ResolveProfile

解析服务授权档位。

1. 1 读取profile快照
2. 2 与ctx.costClass逐字比较
3. 3 校验调用服务允许能力
4. 4 返回版本

## ResolveRoute

确定不可变调用路线。

1. 1 固定档位/任务覆盖版本
2. 2 按orderedModelIds剔除disabled并保留顺序
3. 3 embedding必须同一公共space，空间变化新索引而非混用
4. 4 返回最多16个候选，已派发attempt各自独立计费。

## QuoteCost

计算可保守预留的最大费用。

1. 1 读取manual优先目录价目
2. 2 有完整费率用整数向上取整
3. 3 缺费率固定price_unknown快照且保留usage，不伪造0成本或阻止调用
4. 4 warning说明未知价调用无法完整受正金额额度约束。

## CreateInvocation

接受统一AI调用。

1. 1 可信op固定root/costClass/task授权，qa费用类必须继承已消费guard签名admissionPermit
2. 2 MG验证permit签名/audience=model-gateway/dataEpoch/expiresAt/root request归属，从其构造内部FeeScopeBinding；不接受浏览器指定scope
3. 3 按能力检查payload与模型声明，vision资源经media授权；canonical hash含task/model/content及已验证admissionId
4. 4 同幂等键同hash返回原调用，不同CONFLICT；此时不派发供应方请求
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## ReserveCost

原子扣占额度。

1. 1 从invocation读取MG内部已验证FeeScopeBinding，调用者不能传或覆盖session/network scopes
2. 2 已知价锁site/day/costClass；qa且hourlyLimitsEnabled再锁session/hour和network/hour，network额度=sessionHourlyLimit×networkMultiplier默认5，额度0或总开关关闭表示不限
3. 3 background独立统计不占qa日/访客小时额度；未知价保存price_unknown且金额省略，显示不能完整纳入金额限额warning但允许调用
4. 4 同attempt唯一，新fallback独立预留；旧未知held继续计入同作用域，不释放或伪零
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

## ExtendReservation

预留额不足时扩展。

1. 1 验证held状态
2. 2 锁额度及预留
3. 3 新额仍在档位/日额度内
4. 4 写版本

## DispatchInvocation

派发供应方请求。

1. 1 验证reserved及lease
2. 2 获取供应方slot
3. 3 将dispatching与attempt持久化后发送
4. 4 记录响应或charge_unknown

## BuildProviderRequest

映射统一调用到供应方协议。

1. 1 读取固定模型/价目/密钥版本
2. 2 规范编码能力payload
3. 3 加供应方支持的幂等键
4. 4 返回只供executor内存消费

## ClassifyFailure

决定有限重试与收费未知。

1. 1 responseStarted后不把新模型delta拼进同一已交付结果
2. 2 timeout/限流/格式错误依有序策略可新attempt
3. 3 acceptance未知标旧attempt charge_unknown保留held
4. 4 新attempt先预留后派发，最多每模型一次及格式重试一次且deadline不重置。

## AppendStreamEvent

持久保存流序列。

1. 1 验证租约和非终态
2. 2 事务递增sequence
3. 3 持久事件
4. 4 提交后向订阅者广播

## ResumeStream

重连读取原流。

1. 1 验证调用所有者服务
2. 2 查询sequence>cursor
3. 3 返回最多256项
4. 4 游标超出保留区返回410

## RecordUsage

接受供应方用量证据。

1. 1 校验token与金额范围
2. 2 按provider/request/hash唯一
3. 3 冲突证据进入人工对账
4. 4 保存原始已脱敏证据hash

## SettleInvocation

结算已知费用。

1. 1 供应方实际金额优先
2. 2 否则已知固定价目按usage整数算费
3. 3 两者未知保存price_unknown ledger实际金额字段省略不伪0
4. 4 独立attempt结算，晚到证据通过reconciliation补不可变更正entry并更新额度。

## MarkUnknownCharge

保留不确定收费责任。

1. 1 检查已派发
2. 2 设置charge_unknown与证据
3. 3 维持全部held
4. 4 发布reconciliation.requested

## ReconcileCharge

证据驱动完成未知费用对账。

1. 1 校验可验证供应方证据/授权人工证明
2. 2 charged走唯一结算
3. 3 not_charged才释放held
4. 4 终态reconciled

## CancelInvocation

贯穿取消调用。

1. 1 未派发则cancelled释放held
2. 2 已派发设cancel_requested
3. 3 发送供应方取消若协议支持
4. 4 无未收费证明则charge_unknown

## GetInvocation

读取调用状态。

1. 1 验证服务租户和拥有者
2. 2 读取状态快照
3. 3 不返回prompt/secret
4. 4 返回最近序号

## EstimateTokens

保守估算请求预算。

1. 1 使用声明tokenizer版本
2. 2 未内置tokenizer则ceil(UTF8 bytes/2)+消息开销32
3. 3 乘1.2向上取整
4. 4 不调用供应方

## ValidateEmbeddingSpace

阻止错空间向量入库。

1. 1 重算空间SHA256
2. 2 每向量维度严格等于dimension
3. 3 拒绝NaN/Infinity
4. 4 l2模式范数误差≤1e-5

## AcquireProviderSlot

限制供应方并发。

1. 1 锁供应方槽位计数
2. 2 清除过期租约
3. 3 数量小于配置才取得
4. 4 设置30秒租约每10秒续约

## ReleaseProviderSlot

释放供应方槽位。

1. 1 验证租约绑定调用
2. 2 删除租约
3. 3 对不存在返回released=true
4. 4 不修改账单

## EnqueueOutbox

原子记录业务事件。

1. 1 必须在业务事务内
2. 2 验证事件schema
3. 3 eventId唯一
4. 4 与聚合变更一并提交

## PublishOutbox

持久发布事件。

1. 1 SKIP LOCKED领取100项
2. 2 发jobs并携eventId
3. 3 只有持久ACK后标published
4. 4 失败500/1000/2000ms三次后留待后轮

## RecoverLeases

恢复崩溃中的调用。

1. 1 锁到期租约调用
2. 2 reserved未派发重新排队
3. 3 dispatching/streaming证据不足标charge_unknown
4. 4 不自动重新消费模型

## GetLedger

管理端查询费用账本。

1. 1 核验model.billing.read
2. 2 限制时间窗≤90天
3. 3 按createdAt/entryId keyset分页
4. 4 返回价目可追溯记录

## GetPriceBook

读取历史固定价目。

1. 1 查不晚于effectiveAt最新不可变价目
2. 2 缺少不猜测价格
3. 3 返回快照ID
4. 4 读取不触发报价

## RedactLog

生成最小运行日志。

1. 1 只允许invocationId/rootOperationId/state/code/durationMs/providerId/modelId
2. 2 secret/header/prompt/content全部删除
3. 3 ID合法性检查
4. 4 输出安全字段

## QueryUsage

跨服务读取权威费用状态。

1. 1 仅调用服务自有rootOperationId或model.manage
2. 2 固定ledger读快照
3. 3 将settled与held分别聚合
4. 4 unknown不能折算零费

## ReadModels

读取模型能力声明。

1. 1 service或model.manage
2. 2 仅声明无探测
3. 3 返回不可变模型版本/space
4. 4 disabled仍可读但标不可调用

## ReadProfiles

读取档位声明。

1. 1 验caller capability权限
2. 2 过滤allowed costClass
3. 3 返回配置版本
4. 4 不暴露密钥和全站余额

## AcquireBarrier

取得本地维护屏障。

1. 1 校验服务身份及superadmin委托或限定RecoveryAuthorization
2. 2 将本地control.db屏障draining并拒绝新业务写
3. 3 屏障停止新预留/新派发
4. 在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封
5. 部署主密钥不导出。缺密钥恢复后所有供应方disabled。
6. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。

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
3. 3 屏障停止新预留/新派发
4. 在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封
5. 部署主密钥不导出。缺密钥恢复后所有供应方disabled。
6. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。

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

## PutTaskAssignment

超管维护单任务模型覆盖。

1. 1 taskId在根唯一AI task catalog
2. 2 default tier按任务登记
3. 3 override检查能力
4. 4 CAS保存并发assignment.changed

## ReadTaskAssignments

读取任务模型映射。

1. 1 service能力范围过滤
2. 2 与根目录taskId校验
3. 3 输出固定version
4. 4 禁止模型调用

## RefreshModelCatalog

刷新公开模型目录。

1. 1 host/path固定allowlist下载≤10MiB
2. 2 验证目录封闭schema/价目范围
3. 3 bundled fallback不被删
4. 4 更新remote记录且manualOverride优先

## CreateConnectionProbe

显式测试连接。

1. 1 仅superadmin点击
2. 2 构造固定不含站点内容的1token请求
3. 3 costClass=admin-probe并费用预留
4. 4 显示成功/耗时/账单状态

## CreateFallbackAttempt

费用安全保留备用与格式重试。

1. 1 确认nextModel属于固定路线且未耗尽
2. 2 旧尝试未知费用标charge_unknown保留held
3. 3 预算不足停止而非释放旧金额
4. 4 创建新attempt/预留后才dispatch

## MapAnthropicRequest

映射Anthropic消息协议。

1. 1 system合并独立字段
2. 2 user/assistant保持原序
3. 3 max_tokens固定
4. 4 不将密钥写body

## MapGeminiRequest

映射Gemini协议。

1. 1 role assistant→model
2. 2 systemInstruction独立
3. 3 模型能力选端点
4. 4 embedding维度/模型版本固定

## ParseRerankResponse

解析rerank协议。

1. 1 响应index必须在输入范围
2. 2 无重复且仅既有候选
3. 3 scores有限且排序稳定
4. 4 缺失候选排末不生成正文

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
| model.dispatch | model-gateway.DispatchInvocation |
| model.reconcile | model-gateway.ReconcileCharge |

## ReadFeeBudgetPolicy

读取费用限额配置。

1. 1 校验guard.manage/siteId/active epoch
2. 2 固定policy快照读取，dailyQaLimit=0不限且background独立
3. 3 返回policy，不处理IP或guard频率
4. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。


## PutFeeBudgetPolicy

维护old费用开关与会话小时、网络5倍配置。

1. 1 校验guard.manage、Money/开关/倍率1..100；network默认5倍，不伪造会话额度美元默认
2. 2 锁policy并CAS expectedVersion；同键同hash返回原配置，不同CONFLICT
3. 3 保存新version和审计，仅影响新预留，既有held/ledger不清零
4. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

费用策略授权：若dailyQaLimit与当前值不同，另校验actor具有superadmin；普通manage_ai只能调整小时策略，不能修改old超管专属全站日费用额度。策略全量输入中的未改日额度不扩大权限。
