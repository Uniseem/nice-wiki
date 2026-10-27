# traffic-guard 需求

**M12-R001** 在请求进入高成本或高并发路径前执行频率、并发、挑战及封禁策略，只发准入凭证；model-gateway 仍是唯一实时额度和钱账权威。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R002** 按站点/动作/主体 token bucket 和持久并发租约

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R003** 基于匿名信号的挑战、临时禁令及后台策略

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R004** 签名短期 AdmissionPermit 与事件审计引用

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R005** 不存余额、货币扣款、费用预占或第二份钱账

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R006** 不使用 User-Agent 作为可靠身份

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R007** 不持久化原始 IP、问题正文、captcha secret 或凭证 token

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R008** 服务间HTTP/JSON/SSE；事件使用公共EventEnvelope并标版本，至少一次按eventId和源修订去重

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R009** 所有写入验证当前dataEpoch，恢复后旧任务/租约/token/缓存不得修改或授权新数据

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R010** 私有存储；外域只通过拥有者HTTP合同读写，不共享数据库或直接读私有文件

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收


验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R012** 日志、事件与镜像不记录IP、提问正文或secret；结构化字段仅allowlist

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R013** 未来Go后端、Vue3模块界面按文件/函数合同手写；本阶段只登记设计，无业务源码和测试样例

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R014** 风控不可达：新 QA/高成本导出/模型探测 503，不启用客户端自判；静态公开阅读只受入口基本限流，禁止调用模型。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R015** pepper/签名密钥不可用：不签 permit，恢复调用方得到503；日志只有errorCode/requestId。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R016** jobs不可达：outbox持久重试；不把事件派发失败当原事务失败或重扣令牌。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M12-R017** 恢复后旧permit/挑战因epoch失配拒绝，保留未到期匿名ban；缺pepper时维护不退出。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

首版默认参数：

```json
{
  "qaPerSubjectPerMinute": 30,
  "qaBurst": 5,
  "qaPerSitePerMinute": 600,
  "qaSubjectConcurrency": 1,
  "qaSiteConcurrency": 32,
  "permitLeaseSeconds": 30,
  "renewEverySeconds": 10,
  "maxPermitLifetimeSeconds": 300,
  "challengeTTLSeconds": 120,
  "powDifficultyBits": 18,
  "maxPowDifficultyBits": 22,
  "banDefaultSeconds": 600,
  "banMaxSeconds": 2592000,
  "externalChallengeTimeoutMs": 2500,
  "subjectKeyRotationDays": 30,
  "expiredLeaseSweepSeconds": 5,
  "policyRevisionCAS": true,
  "verificationPrimary": "Cloudflare Turnstile 无感验证，每位访客第一次问前",
  "verificationFallback": "组件加载5秒失败→自托管PoW，正常约1-2秒，可疑提高difficulty",
  "externalChallengeEnabled": true,
  "banScope": "仅AI功能；IPv4单地址，IPv6 /64；公开阅读不受此ban",
  "suspiciousReverifyEveryQuestions": 5,
  "questionMaxCharacters": 500,
  "unrelatedQuestionPolicy": "前置模型直接拒答，不检索不调用精确模型",
  "costLimitsOwner": "model-gateway：session+IP每小时、IP默认5倍session；全站每日仅QA，0无限；后台单计，可整体关闭小时花费限流"
}
```

精确参数约束以types.json为准；不从客户端接受成本分类、可信actor、IP或secret。
