# 数据、权威与持久化

site-config私有settings/siteId/revisionId.json是站点设置权威文件，banner正文同样不可变；私有head/journal只有COMMITTED可见。业务数据库保存设置索引、历史水位、receipt/outbox/stages，以及不可重建的初始化决定与完整准备材料。独立control.db保留runtimeState与恢复切换回执，绝不被业务快照覆盖。设置文件（不含credentials/keys/可信代理）通过对象API由data-protection镜像纳入data git。requiredModules固定为ActivationReceipt枚举中的14个有状态域，portal无状态，不登记恢复回执。

严格保留old home.md/D17、D34、admin.md站点设置。hero仅一屏，无卡片精选。主标题/副标题/按钮文字、站点名、footer/banner会翻译，Welcome To默认不翻译可开。bannerId是访客localStorage关闭身份。baseLanguage在初始化选择默认zh-Hans；已初始化无变更基准语言的新产品入口。

类型、required/额外字段、枚举、最大长度、空值与格式均由 [types.json](../contracts/types.json) 定义。函数输入和结果各有独立 schema，内部句柄类型标 x-internal。

| 类型 | 精确含义 |
|---|---|
| Identifier | Identifier |
| Revision | Revision |
| Hash | Hash |
| Timestamp | Timestamp |
| AuthContext | AuthContext |
| Error | Error |
| Problem | Problem |
| PageRequest | PageRequest |
| PageInfo | PageInfo |
| PublishedRevisionRef | PublishedRevisionRef |
| PublicationValidation | PublicationValidation |
| AssetRef | AssetRef |
| ObjectRef | ObjectRef |
| BarrierRequest | BarrierRequest |
| BarrierStatus | BarrierStatus |
| SnapshotRef | SnapshotRef |
| SnapshotExportRequest | SnapshotExportRequest |
| SnapshotExportResult | SnapshotExportResult |
| SnapshotStageResult | SnapshotStageResult |
| EventEnvelope | EventEnvelope |
| HealthStatus | HealthStatus |
| LanguageTag | 规范 BCP47 语言 tag；大小写按语言/脚本/区域规范，不代表需要模型验证 |
| SafeURL | https 公共地址；本地开发 http 由部署能力显式允许；禁止 userinfo/query/fragment |
| HomeConfig | 仅全屏一屏hero；下方无卡片/精选/其他内容 |
| ResourceLimits | 可配置资源上限 |
| SecurityPolicy | 会话/预览非业务功能新增 |
| SiteConfig | 版本化站点设置权威文件；既有功能不增添多内容首页 |
| RuntimePhase | 独立控制库状态 |
| ActivationReceipt | 恢复切换/换回回执；只接受对应服务身份 |
| RuntimeState | 运行状态权威；独立 control.db，不进入业务快照恢复覆盖 |
| RuntimeTransition | 严格控制迁移；不能任意设置phase/maintenance/dataEpoch |
| InitializationState | 首超管注册后的站点/语言两步向导；没有AI和owner reservation |
| AnnouncementBody | 只有一条可关闭公告横幅 |
| Announcement | 单条站点公告横幅；无富文公告列表产品 |
| ConfigChanged | 只传配置引用 |
| ReadSiteConfigInput | 读取后台站点配置的输入 |
| ReadSiteConfigResult | 读取后台站点配置的结果 |
| UpdateSiteConfigInput | 一般站点设置，品牌/hero/footer/banner分别接口 |
| UpdateSiteConfigResult | 更新站点常规配置的结果 |
| ValidateSiteConfigInput | 校验配置所有字段和关联约束的输入 |
| ValidateSiteConfigResult | 校验配置所有字段和关联约束的结果 |
| NormalizeBaseURLInput | 规范公开站点地址的输入 |
| NormalizeBaseURLResult | 规范公开站点地址的结果 |
| ValidateLanguageTagInput | 规范基准语言 tag的输入 |
| ValidateLanguageTagResult | 规范基准语言 tag的结果 |
| UpdateHomeInput | 配置首页介绍、卡片和精选文档的输入 |
| UpdateHomeResult | 配置首页介绍、卡片和精选文档的结果 |
| ReadPublicHomeInput | 读取可公开的首页配置的输入 |
| ReadPublicHomeResult | 读取可公开的首页配置的结果 |
| ValidateHomeLinksInput | hero校验 |
| ValidateHomeLinksResult | hero引用 |
| ReadRuntimeStateInput | 读取独立控制库权威运行状态的输入 |
| ReadRuntimeStateResult | 读取独立控制库权威运行状态的结果 |
| TransitionRuntimeStateResult | 按CAS切换epoch与维护阶段的结果 |
| ValidateRuntimeTransitionInput | 验证维护与恢复状态迁移矩阵的输入 |
| ValidateRuntimeTransitionResult | 验证维护与恢复状态迁移矩阵的结果 |
| RegisterActivationInput | 登记域恢复切换/换回回执的输入 |
| RegisterActivationResult | 登记域恢复切换/换回回执的结果 |
| CheckWriteEpochInput | 所有业务写入执行epoch与屏障检查的输入 |
| CheckWriteEpochResult | 所有业务写入执行epoch与屏障检查的结果 |
| InitializeSiteInput | D55初始化仅站点和基准语言两步；默认zh-Hans |
| InitializeSiteResult | 无需AI完成首站点初始化的结果 |
| ReadInitializationStatusInput | 读取安全初始化阶段的输入 |
| ReadInitializationStatusResult | 读取安全初始化阶段的结果 |
| PrepareDefaultConfigInput | 确定性生成首次基础配置的输入 |
| PrepareDefaultConfigResult | 确定性生成首次基础配置的结果 |
| PersistInitializationDecisionInput | 持久化不可反悔的初始化决定的输入 |
| PersistInitializationDecisionResult | 持久化不可反悔的初始化决定的结果 |
| ResumeInitializationInput | 恢复崩溃或依赖断开的初始化阶段的输入 |
| ResumeInitializationResult | 恢复崩溃或依赖断开的初始化阶段的结果 |
| AbortInitializationInput | 取消尚未持久commit的初始化的输入 |
| AbortInitializationResult | 取消尚未持久commit的初始化的结果 |
| ValidateAnnouncementBodyInput | 校验公告纯文本正文并规范序列化的输入 |
| ValidateAnnouncementBodyResult | 校验公告纯文本正文并规范序列化的结果 |
| WriteAnnouncementRevisionInput | 持久写不可变公告正文文件的输入 |
| WriteAnnouncementRevisionResult | 持久写不可变公告正文文件的结果 |
| CommitAnnouncementHeadInput | 原子提交公告head与正文日志的输入 |
| CommitAnnouncementHeadResult | 原子提交公告head与正文日志的结果 |
| RecoverAnnouncementJournalInput | 恢复正文文件与提交日志的输入 |
| RecoverAnnouncementJournalResult | 恢复正文文件与提交日志的结果 |
| ResolveConfigOperationInput | 查询配置幂等回执的输入 |
| ResolveConfigOperationResult | 查询配置幂等回执的结果 |
| PublishConfigEventInput | 将配置事件与业务提交原子登记的输入 |
| PublishConfigEventResult | 将配置事件与业务提交原子登记的结果 |
| ReadConfigObjectInput | 经协议读取本域正文或快照对象的输入 |
| ReadConfigObjectResult | 经协议读取本域正文或快照对象的结果 |
| ReleaseBarrierInput | 释放已协调完成的屏障的输入 |
| ReleaseBarrierResult | 释放已协调完成的屏障的结果 |
| LinkTarget | 站点链接判别联合体 |
| HeroButton | 首页0–3按钮 |
| HeroBackground | 首页背景完整联合体 |
| FooterLink | 页脚链接 |
| FooterConfig | 页脚版权、链接、ICP、源代码 |
| UpdateBrandingInput | 更新站名/logo/图标/主题色/默认分享图的输入 |
| UpdateBrandingResult | 更新站名/logo/图标/主题色/默认分享图的结果 |
| UpdateFooterInput | 更新页脚版权/自定义链接/ICP备案/源码链接的输入 |
| UpdateFooterResult | 更新页脚版权/自定义链接/ICP备案/源码链接的结果 |
| UpdateBannerInput | 保存单条可关闭公告横幅的输入 |
| UpdateBannerResult | 保存单条可关闭公告横幅的结果 |
| WriteConfigRevisionInput | 写权威版本化站点设置文件的输入 |
| WriteConfigRevisionResult | 写权威版本化站点设置文件的结果 |
| CommitConfigHeadInput | 提交配置权威head与日志的输入 |
| CommitConfigHeadResult | 提交配置权威head与日志的结果 |
| ReadConfigHistoryInput | 读取可回滚的站点设置历史的输入 |
| ReadConfigHistoryResult | 读取可回滚的站点设置历史的结果 |
| RestoreConfigRevisionInput | 回滚首页/公告/品牌/页脚历史的输入 |
| RestoreConfigRevisionResult | 回滚首页/公告/品牌/页脚历史的结果 |
| ListResourceReferencesInput | 向媒体返回站点设置权威资源引用快照的输入 |
| ListResourceReferencesResult | 向媒体返回站点设置权威资源引用快照的结果 |
| releaseBarrierRequest | 释放屏障请求体 |
| restoreConfigRevisionRequest | 回滚站点设置请求体 |

恢复重建必须验证 manifest 哈希和私有事务水位。derived cache 可重建；权威记录缺失或 schema 未知应隔离并停止对外发布。

## 初始化准备与决定

`active/site-config.db` 的 `initialization_decisions` 保存每个初始化 operation 的不可反悔决定、原始结果回执、完整准备配置规范 JSON 与 SHA256；`initialization_state` 保存当前协调状态和 CAS revision。二者由 `0002_initialization.sql` 只追加迁移建立，同事务以 SQLite `synchronous=FULL` 提交。决定记录不是可重建索引，也不等同 `settings` 权威文件已提交。commit 决定持久化后仍为 preparing；ResumeInitialization 核对材料和原始请求回执后，先提交 settings 权威文件与 COMMITTED 决定，再把配置水位推进 config_committed；最后独立事务推进 completed 与 initialized=true。准备阶段不对公开站点发布。

尚未开始时 operationId/siteId 为 null，revision/configRevision 为 0；updatedAt 使用 active 状态的持久建库时间。公开状态固定为 `initialized` 与只含 phase/safeError 的 state。完整 coordination 只向同站点超管或 identity/site-config 可信服务返回，匿名完全省略。

初始化默认值在本轮集成裁决中明确：主题色 `#2563EB`，hero 背景 `#0F172A`、遮罩 0、浅色文字、无按钮、副标题空、背景字 `Welcome To` 且不翻译；标题模板 `{title} | {site}`、允许索引，版权/自定义链接空、备案 null、源码链接 `https://github.com/Uniseem/nice-wiki`。资源上限为文档 16 MiB、单次上传 128 MiB、站点 10 GiB、上传并发 4；预览 900 秒、会话最长 604800 秒。logo/favicon/defaultShareImage/banner 为 null。以上默认数值此前产品基线未固定，本轮据现有 schema 范围补齐；不新增示例文档、账号、AI 渠道或模型任务。


## settings 权威文件与提交恢复

`active/settings/<siteId>/<revisionId>.json` 使用 `ConfigDocument` v1：`schemaVersion=1` 与完整 `config`。规范 JSON 是权威正文，SQLite 内的准备材料不能代替该文件。新增格式从空白 settings 实现开始；未知版本或不符合生成 SiteConfig schema 的文件失败关闭。修订号、对象大小与 SHA256 必须和 ObjectRef 一致。不可变对象通过同文件系统临时文件写入、fsync、排他 Link 和父目录 fsync 发布；相同修订同字节复用，异字节拒绝。head 采用临时文件 fsync、Rename 和父目录 fsync。新建目录逐级 fsync 父目录。

`active/journal/config/<revisionId>.prepared.json` 和 `.committed.json` 是独立原子、不可变的规范记录，均为 schemaVersion=1。采用独立文件避免 JSONL 半条记录被误读为完整决定。PREPARED 只记录准备对象；COMMITTED 额外固定原始 actor/site/operation、业务 requestHash、dataEpoch、expectedRevision、初始化标记和 changedSections。配置事务先取得 SQLite 写锁，并检查目标修订不存在另一 COMMITTED 决定，之后持久 COMMITTED，再更新 head、最后提交 SQL 索引、水位、回执和平台 outbox。SQL 不宣称覆盖文件系统的原子性；COMMITTED 落盘后中断只能补完，不能回滚决定。

`0003_config_commit.sql` 追加 `config_operations`，保存不可重建的原始 actor/site/op/epoch/业务摘要、稳定 revisionId、阶段与原始完成结果。初始化业务摘要固定为操作种类、站点名和规范语言；准备材料 SHA256 另存 `initialization_decisions`，不把包含 preparedAt 的材料摘要用作原始请求摘要。初始化决定与原始回执同事务提交；revisionId 由 epoch/site/初始化 operation 的 SHA256 固定。旧 0002 决定若缺原始身份回执不推测主体，按 INTEGRITY_ERROR 阻止完成。

启动 `RecoverConfigJournal` 只回放明确 COMMITTED 的文件决定，逐条核对连续修订、epoch、PREPARED、权威对象 schema/hash 与原始回执，补全 SQL 索引、水位、回执和 outbox，再发布最新 head。没有 COMMITTED 的对象不公开；有初始化 commit 决定的 PREPARED 留给 Resume，其他 PREPARED 写入 `active/quarantine/config` 隔离标记。已提交对象缺失或损坏会阻止启动。Ready 须核对已恢复状态、COMMITTED、head、SQL 水位和权威对象。

配置提交与初始化完成分开持久：config_committed 或 blocked 时允许存在配置水位 1、initialized=false 的恢复中间态；只有最后完成事务才置 initialized=true。初始化 completed 结果保留首次配置水位，不随后续设置修订改写；读取协调状态允许已完成初始化水位小于当前配置水位。abort 不生成配置 stage，不删除超管；同取消 operation 始终返回原始取消结果。


配置持久 IO 自身承担文件权威边界：从文件系统根目录使用 `os.Root` 能力句柄逐段 `Lstat` 并打开下一目录，拒绝父路径中的符号链接、特殊文件和非目录。后续文件创建、Link、Rename 和读取都相对已核对的目录句柄进行，不重新跟随绝对父路径。读取前后核对同一正规文件，末级 `O_NOFOLLOW` 拒绝链接、`O_NONBLOCK` 避免文件被替成 FIFO 后阻塞；既有不可变文件复用也经过该边界。正文、head、PREPARED、COMMITTED 和隔离标记的规范 JSON 均以入口既有总界 16 MiB 为上限，读取使用有界 reader 防止损坏或并发增长无限占用内存；类型、目录身份或大小异常失败关闭。


## 一般设置操作与已提交读取

一般设置只改变公开地址、SEO、资源上限和安全参数；品牌、首页、页脚、公告、基准语言和 initialized 不在该输入中。`ReadSiteConfig` 要求 site.read/superadmin，或已登记内部服务主体；携用户委托的服务调用仍按该用户的站点和权限判断，不能借服务名绕过授权。读取在本域文件提交锁和只读事务内，把 initialized/epoch/配置水位、正式索引、COMMITTED、原始回执、head 与权威文件视为同一边界。任何未补齐决定或水位不一致失败关闭。

`0004_config_operations.sql` 追加 `config_operation_materials`，保存一般设置 operation 的种类、完整候选 ConfigDocument 规范 JSON、SHA256 与修订号；同事务建立原始 actor/site/epoch/requestHash 回执和稳定 revisionId。业务摘要包含 kind=settings 与原始严格输入的规范 JSON（含 expectedRevision），先读取已成功的原始结果，当前 URL/HTTP 能力策略只应用于新操作和未完成操作，材料保留首次 updatedAt。候选 JSON 仅供未完成操作继续写文件，不能作为正式配置读取。只在明确 COMMITTED 后由文件决定恢复 SQL 水位和 outbox；未 COMMITTED 的普通候选须同 operation 重试，不在启动时从数据库推定发布。

配置对象 ID 与站点内 revision 在追加迁移中建立唯一索引，拒绝歧义历史。已完成 operation 的重试根据原始回执读取原修订文件；即使后来已有更高配置水位，也不改写最新 head。未完成操作复用原候选材料与既有 PREPARED/COMMITTED；跨进程仍由 SQLite 写锁和文件决定占有目标水位。

URL/IDNA 规范化只调用平台 `lifecycle.NormalizePublicURL`；本域保存由进程配置注入的 allowLocalHTTP 能力，HTTP 请求不能自造该字段。ValidateSiteConfig 只做模板占位符、规范语言、链接安全、ID 唯一和公告作用域/时间窗等纯语义，入口或持久 IO 已完成的结构校验不重复执行。普通设置不重查未改变的 media/content 引用；改变引用的用例另行承担实时授权和 pin 恢复。

`ReadConfigObject` 本轮仅支持正式索引中已提交的 settings 对象；公告/快照生产方未完成时不制造通用成功。按 objectId/version/expectedHash 精确读取并验证，同样复用既有正规文件、目录能力与 16 MiB 上限。返回的 io.ReadCloser 只在服务进程内流转，HTTP 输出原始对象字节，ETag（带引号 SHA256）、Content-Length/Type、对象 ID/版本头与同一 ObjectRef 绑定；不存在跨进程 bodyHandle 协议或全局句柄表。HTTP 适配负责消费并关闭该流。
