# ADR0002：事件统一拉取投递与两级函数合同

- 日期：2026-10-14。
- 状态：用户授权本轮修订并已写入合同；业务实现尚未开始。备份恢复简化另待确认（plan R0-07）。
- 来源：审阅 ADR0001 产出的十五域规格时发现事件链断开、部分发件箱没有转发者、签名与函数编号各域不一、依赖引用多数无法机器校验，以及逐个私有函数预登记带来的实现摩擦。

## 问题

1. jobs 同时设计了 HTTP 推送与 SSE 拉取两套投递方式。推送的目标接口从未登记；media、knowledge、qa、localization、memory、traffic-guard、data-protection 声明消费事件，却既没有接收接口，也没有订阅调用。public-site 只订阅不确认，analytics 用一个推送接口加确认的混合方式。
2. identity、site-config、content、media 在业务事务里写 outbox，却只在步骤里提到一个未登记的 dispatcher；public-site 登记了两个既无生产函数也无消费者的事件；portal 直接调用 jobs:publishEvent，却未登记任何生产事件。
3. 206 个函数用 RequestContext 别名，其余用 `(ctx context.Context, op OperationContext)`；部分浏览器函数接收服务端操作上下文，违反“浏览器不能自造上下文”的协议。函数 ID 有 `content.X` 与 `M06-F019` 两种写法。
4. 950 条依赖中约一半是函数名、点号写法的跨域操作或自由文本，检查脚本没有解析，调用图无法验证。
5. 要求所有私有辅助函数先登记，实现阶段每次拆分或重构都要先改 JSON 合同和人类文档。

## 选择

1. 事件只经 jobs 中转，统一拉取：
   - 生产方在业务事务内写 outbox；本域转发循环（新登记的 RelayOutbox，或已有的 PublishOutbox 等同类函数）在启动时和后台循环中把已提交行交给 jobs:publishEvent，确认后标 relayed，失败退避重试，异常行隔离不丢弃。
   - jobs 按订阅目录（config/event-subscriptions.json，由各域 events.json 的 consume 条目生成）为每个消费方建立投递。HTTP 推送投递取消，原函数改为 RequeueExpiredDeliveries，负责到期重投与死信。
   - 消费方的 ConsumeEvents 以 durable 模式调用 jobs:subscribeEvents，按 events.json 新增的 handler 字段调用本域处理函数；处理函数在本地事务内完成 inbox 去重与业务写入，提交后才调用 jobs:acknowledgeEvent。portal 浏览器转发使用 ephemeral 订阅，不建投递。
   - 删除 analytics 的 POST /v1/events 推送入口，以及 public-site 两个无人生产也无人消费的事件；portal 访问审计改为调用 analytics:appendAudit。
2. 签名：Go 操作函数统一为 `(ctx context.Context, op OperationContext, input …)`，删除 RequestContext 别名；浏览器函数统一为 `(input, signal: AbortSignal)`。
3. 函数 ID 统一为“模块slug.函数名”，取消编号写法。
4. dependencies 只允许本域函数 ID 或“模块:operationId”；非函数依赖移入新增的 resources 字段。
5. 函数合同分两级：HTTP 处理、事件转发/订阅/处理、任务执行与启动入口为 binding（约束），其余为 reference（参考）。参考函数实现时可增删改名，模块交付时同步合同与台账。
6. functions.md 与 files.md 由 scripts/render_docs.py 从合同生成；check_docs.py 新增依赖解析、签名约定、ID 格式、合同级别、事件 handler、订阅循环、outbox 转发，以及生成文档一致性校验。

## 代价与影响

- 合同、台账和生成文档大面积改写；所有条目仍是 planned，没有业务实现。
- 参考级放宽降低了登记摩擦，代价是检查只保证约束级函数与结构一致，不能证明参考拆分合理；交付时同步合同仍是义务。
- 每个消费域多一条到 jobs 的订阅长连接，jobs 成为事件的唯一中转：jobs 故障时事件延迟，但 outbox 保留，不会丢失。
- 备份恢复的简化已由 [ADR0003](0003-restore-by-restart.md) 完成。
