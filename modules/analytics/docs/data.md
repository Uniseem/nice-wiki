# analytics 数据与持久化

未来私有目录 `data/analytics/active/` 保存SQLite和领域对象；`data/analytics/restore/<restoreId>/staged|before/` 为恢复暂存与换出的旧数据；`data/analytics/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `beacon_inbox(beacon_id PK, document_id, locale, event_type, status_class, elapsed_bucket, daily_visitor_hash, source_domain, country_code, occurred_at, data_epoch；不存IP/UA原文，保留30天)`

- `hourly_page_counts(hour, document_id, locale, views, visits, load_bucket_counts, data_epoch; PK hour/document_id/locale)`

- `audit_records(audit_id PK, event_id UNIQUE, actor_id, actor_kind, action, resource_ref, result_code, occurred_at, hash, previous_hash, data_epoch)`

- `usage_projection(ledger_entry_id PK, ledger_revision, operation_id, cost_class, provider_id, model_id, amount_micros, currency, state, occurred_at, data_epoch)`

- `daily_usage(day, cost_class, provider_id, model_id, amount_micros, entry_count, data_epoch; PK day/class/provider/model)`

- V6 `usage_facts_v2` 保存每一条已核验V2账项/观察的不可变完整事实；`usage_projection_v2` 与 `usage_observations_v2` 只保存当前投影。账项保留原 owner/site/sourceEpoch/root/op/class/provider/model、独立 changeRevision/ledgerRevision、amountKnown/USD微额/state/time/captureKind；观察保留 invocation/attempt/reservation 的 held、maximum/held 金额与 unknownUsageCount。unknown 以缺失金额表达，禁止补0；每个完整固定上界由事件入口先ClaimInbox，再与投影同事务提交。

- `event_inbox(event_id PK, source_revision, data_epoch)`

- `aggregation_watermarks(source, revision, updated_at)`

- `export_runs(export_id PK, filter_hash, actor_id, object_ref, expires_at, state)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

V6迁移连续跟随V5。旧 `usage_projection`、`daily_usage` 或 usage 水位非空时拒绝静默升级，避免把缺owner/site/sourceEpoch的旧行宣称为完整V2来源。新事实按 `dataEpoch+changeRevision` 唯一并校验实体修订单调；同修订/同实体修订对应不同原始hash返回冲突，旧修订只保留历史，不覆盖新当前投影。投影事务不建立 `usage-v2-complete`，现有费用查询继续失败关闭。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP管理协议（barrier/export/stage/discard），在safe屏障固定revisionWatermark；恢复先全域暂存，写切换决定后各服务重启换入，全部域登记回执才解维护。control区不随恢复替换。换入后先schema迁移与完整性验证，失败拒绝启动并可整体换回。

R1收尾首切片：私有SQLite V1只在空未版本化库事务建表，未知旧库维护拒绝；siteId严格来自site-config:readSiteConfig，epoch来自runtime，不造身份。控制屏障由公共机制持有，不放active表；盐不进入快照。日报总PV/UV独立于source/country维度表，UV不可SUM维度distinct。

页面查询兼容粒度hour/day：完整UTC小时或唯一部署ReportingTimeZone的完整日历日，默认7完整日/168小时；显式边界必须对齐，day以日历366日和DST计算。返回真实粒度/fromUTC/toUTC，水位取已聚合pages，不拿新beacon水位冒充已聚合。日盐与日汇总共用可信报表时区，未配不假设UTC。审计游标签名复用平台，固定seq上界和保留锚；身份/过滤/锚变化拒混页。审计链V1以UTF8前hash十六进制串拼固定canonical payload，含内部安全metadata但不将原metadata作为HTTP输出。

V2审计事实：固定producer/action，HTTP历史三字段同现且纯服务提交原actorId；旧实时仍当前主体。异步只原业务事务outbox→jobs→ConsumeAuditEvent，origin显式取已验证Envelope.Producer，不伪op调用方；payload采用AuditRecordedPayload，原site/epoch/operation/time全部哈希。新成功OK，兼容HTTP SUCCESS别名；V1 hash只读保留，新增行schemaVersion=2。链内来源与安全metadata、inbox和唯一auditId同事务；重复信封不重新插入被保留期删除的旧事实。

首批readSiteIdentity误将“免费控制RPC”写成非法costClass=free，现清为空；合法非空类别仅qa/background/admin-probe。只做静态修正，未以编译当接口验收。

匿名统计实施格式：ReportingTimeZone在当前代次持久固定，首次仅running且无未知旧匿名明细绑定，不默认UTC；HMAC载荷canonical[siteId,dataEpoch,day,规范IP,小写折叠空白UA]，每日本机control独立32字节盐，盐与原IP/UA不进快照。portal私有签名正文POST /v1/internal/beacons为唯一真实访客地址来源，本域不解析代理头；公开原正文2048、签名内部正文4096，队列4096、每分钟3000。flush真实新增计数，空队列漏数亦持久，小时/本地日分别记来源水位；历史聚合不能因全局新访问和原明细过期被重算清零。Crawler只固定public-site纯服务成功读取按可信本地日计，独立幂等回执。

小时/日报任务V3：aggregate_jobs仅不可变业务输入/原提交意图与真实执行镜像，不是领取队列。canonical AggregateJobBody schemaVersion1绑定kind/period/timezone/sourceWatermark/siteId/dataEpoch，真实字节/长度hash构造本域ObjectRef；固定原operation/root/dedupeKey，未知create只原key续接，禁止修改旧输入。每分钟有界扫描闭合周期（小时5分钟、日10分钟）与迟到真实水位，jobs唯一调度。worker用平台真实续租/取消钩子，事务前锁外CheckLease，提交前核本域同job/worker/fence/epoch/attempt/expiry镜像；恢复不得复活旧镜像执行权。总UV与各source/country维度分开distinct。超过原明细保留且缺原聚合不可重建0；历史水位不受新全局访问影响。费用任务仍缺完整协议，RunWorker整体与快照/Ready继续blocked。

问答来源投影V4：qa_turn_projection仅真实轮次/来源修订/分类/hash/时间，inbox同事务。重复和低修订不增；更高修订撤旧再新计，同修订异完整事实冲突。旧非空汇总无来源修订拒绝静默迁移。failed事件生产schema无language，内部locale空表示未提供，不造语言。内容缺口只完整可信日历日，同快照返page/qa水位、QA分子分母/最近提交时间，0样本显示无样本；访次累加非跨日独立访客，问题正文仍qa权限入口。费用事件缺完整安全统计页仍不确认，审计producer事件尚未装配；整体订阅/Ready仍blocked。

私有CSV与保留期V5：统计pages/usage和审计audits使用固定不同权限创建/下载路由；边界即时授权，同原actor/site/epoch、24h与完整hash。固定安全列≤100000行/32MiB，公式首字符=+@-、tab/CR/LF加单引号，CSV转义。preparing原操作/hash/期限→part完整文件及目录fsync→prepared真实ref/原结果→rename目录fsync→ready；prepared只续原字节不重查。启动续journal/未知旧记录对象拒绝；到期保留幂等墓碑不重授24h。原明细30天有界续清，未完整派生page-view另记真实漏数而不改变period水位；原salt新日及每分钟擦旧。审计只删逐条核链的最老连续过期前缀，默认180最少30，原锚/删除/retention审计同事务，晚到原时间旧事实可能更久保留以维持链，绝不删中间或重写原hash。费用页面仅已有投影/真实对账时间，不冒全站实时余额。

费用V2消费候选：OpenAPI `x-runtime-payload-schemas` 登记 `AnalyticsUsageStatisticsPageResult`，wire用 `apivalidate.NewPayloadValidator`生成严格校验。`FetchUsagePage`按固定watermark续签约2.5秒单页、60秒总时限，单页2MiB、全范围原始JSON累计32MiB；每页先schema校验，再仅删pageHash算canonical SHA256，所有页完成才返回。`ConsumeAnalyticsEvent`只在完整页找到事件原账项或not_charged原attempt后，才在同事务写inbox与V2事实。该候选因GitLab HTTP 522和待统一gen保持blocked。
