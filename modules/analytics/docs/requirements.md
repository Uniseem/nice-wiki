# analytics 需求

**M14-R001** 聚合匿名公开访问、管理操作审计和 model-gateway 费用引用，提供后台统计及导出；实时预算、账本余额仍由 model-gateway 决定。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R002** 允许字段匿名beacon、小时/日聚合与漏数计数

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R003** 管理审计引用、安全结构化元数据和权限查询

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R004** 按gateway唯一ledgerEntryId派生费用聚合及对账

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R005** 不记录IP/提问正文/模型输入输出/secret/token

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R006** 不追踪跨站访客或持久设备指纹

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R007** 不控制实时额度，不写费用账，不把派生汇总当结算真相

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R008** 服务间HTTP/JSON/SSE；事件使用公共EventEnvelope并标版本，至少一次按eventId和源修订去重

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R009** 所有写入验证当前dataEpoch，恢复后旧任务/租约/token/缓存不得修改或授权新数据

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R010** 私有存储；外域只通过拥有者HTTP合同读写，不共享数据库或直接读私有文件

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收


验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R012** 日志、事件与镜像不记录IP、提问正文或secret；结构化字段仅allowlist

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R013** 未来Go后端、Vue3模块界面按文件/函数合同手写；本阶段只登记设计，无业务源码和测试样例

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R014** analytics停机：匿名beacon丢弃；安全审计/费用事件由生产者outbox持久保留，不能把失联当已审计。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R015** gateway停机：展示已有聚合同时标最后watermark/对账时间；不声称实时余额，不创建模型请求补数。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R016** 日志/trace统一仅requestId/errorCode/模块/状态/耗时；导出和审计严格allowlist排除敏感字段。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M14-R017** 恢复后保留费用投影引用但标需对账；secret日salt/原始匿名token/IP不进入快照或git。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

首版默认参数：

```json
{
  "beaconMaxBodyBytes": 2048,
  "beaconRetentionHours": 720,
  "hourlyRetentionDays": 400,
  "dailyRetentionDays": 0,
  "auditRetentionDays": 180,
  "usageRetentionDays": 1095,
  "ingressQueueSize": 4096,
  "flushEverySeconds": 5,
  "flushBatchSize": 500,
  "queryRangeMaxDays": 366,
  "queryDefaultDays": 7,
  "reconcileEveryHours": 24,
  "redactionPolicy": "allowlist-only",
  "exportMaxRows": 100000,
  "visitorCounting": "HMAC(IP+规范User-Agent+每日随机盐)，日盐每日更换；IP/UA原文不存，Cookie不用于统计",
  "rawVisitRetentionDays": 30,
  "geoIPSource": "本地离线GeoIP数据库，不调用第三方",
  "referrerStorage": "仅来源域名，去掉路径/query/fragment",
  "knownCrawlerFiltering": true,
  "aiMarkdownAccessSeparate": true
}
```

精确参数约束以types.json为准；不从客户端接受成本分类、可信actor、IP或secret。
