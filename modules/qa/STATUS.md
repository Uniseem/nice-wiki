# 问答与缓存当前进度

更新：2026-10-18。R1 实施，业务验收与部署均未开始。

- 本分支 `codex/qa-20261018-bulk`，基线 `82179b907d25a546e9f470a1fb9cf1584ab45088`。首批源码 `fea4204`，第二批源码 `358fe68`，第三批源码 `6813f10`。逐函数台账统一 `blocked` 待根 GitLab 生成、格式、readonly build/vet 及完整 make check。
- 累计35个原有业务函数与53个实际助手，共88项代码就绪，合同119函数。第三批新增6个原有业务函数及5个必要助手；没有把未运行的检查记为完成。
- 已有真实会话/缓存/反馈/SSE持久事务；第二批补上独立任务记忆、可信 Planner 结构输出及风险报告、查询向量与完整证据读取、白名单引用与真实公开链接核对、自动缓存写入、管理记录与隔离诊断读取、压缩CAS提交、平台outbox转发。
- 原准入按稳定 RootOperationID 绑定；mint只含分配元数据，qa.ask/followup消费取得非空签名许可后在线验证并供付费链继承；免费qa.conversation消费前后token均为空，创建提交/重放后幂等释放原占位。不存在启动静态访客许可。挑战使用 CHALLENGE_REQUIRED，过期使用 GONE/410。
- 同步首批 CI 窄修：生成策略类型 TypesQaPolicy、Timestamp按time.Time消费、内部 QuestionPlan 共享类型；游标排除瞬时委托到期，SSE统一复用平台心跳与背压机制。implementation.phase保持design；progress.phase为implementation。
- 第三批补齐完整语义缓存、真实事件inbox失效、CompleteTurn、CancelTurn、DeleteConversation及PruneExpiredRecords；后两公开入口已映射。完整StartTurn/ExecuteTurn、模型回答与审查、精选批准、压缩生产与执行、恢复/后台任务和完整服务启动仍未完成。真实消费者函数完成不等于完整业务入口已可部署。
- 本机仅阅读、合同台账同步、Git隐私审阅及 git diff --check；未运行Go/gofmt/gen/build/vet/check、测试、真实接口、模型、服务或推送。根负责及时GitLab检查和双推，本域继续下一批。

免费会话窄修源码 `77c450c`：qa.conversation消费前后无模型token，仍校验已消费原root；创建提交/同输入回放后释放原slot。付费ask/followup继续要求非空消费签名。88项ready不增加，仍待GitLab，已再次冻结。
