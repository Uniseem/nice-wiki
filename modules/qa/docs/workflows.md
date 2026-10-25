# 问答、引用与回答缓存算法与工作流

状态机：Turn: admitted→queued→retrieving→generating→completed；任一步→cancelled|failed|invalidated。Cache: pending→approved→active→stale|disabled。Compaction: queued→running→committed|superseded|failed。Conversation: active→expired|deleted。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

#### `qa.turn`

- 执行函数：`qa.ExecuteTurn`
- 触发：StartTurn与排队轮次出队时创建

1. 校验原准入和轮次归属，续租并贯穿取消。
2. 加载历史时复核公开来源版本，读取有效原生检查点及其后的消息；预读当前页和站点导航。
3. 交由独立 ZCode AgentRuntime 使用模型默认窗口，自主以英文关键词进行嵌入混合检索并继续查阅相关正文；接近窗口上限时由原生机制压缩。
4. 模型调用仍由问答服务经模型网关执行，工具继续在本域核对公开权限与发布版本；不应用旧固定上下文、轮数或工具次数限制。
5. 保存本轮原始工具消息及最新原生检查点，校验最终引用并完成问答、释放准入；失败不重新派发原模型调用。

#### `qa.compact`

- 执行函数：`qa.RunCompaction`
- 触发：CreateCompaction创建

1. 输入previousSummary加newMessage范围增量更新，不从头重写全部历史
2. fast task qa.compact且不注入长期记忆
3. 继承本轮qa根与费用额度
4. 固定recent retained范围，取消后不提交摘要。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `qa.LoadRuntime` | 启动时 | 启动问答服务 |
| `qa.RecoverTurns` | 启动时及每30秒 | 恢复崩溃问答 |
| `qa.PruneExpiredRecords` | 每日 | 清理到期记录 |
| `qa.PublishOutbox` | 启动后持续，空闲每1秒 | 发布问答脱敏事件 |
| `qa.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `qa.RunWorker` | 启动后持续 | 主动领取本域持久任务 |
<!-- flows:end -->

## 2026-10-18 Planner 完整性与首批真实入口

可信前置提示须明确要求：把代词、省略主体、未命名的第二步来源落实为完整独立 canonicalQuestion；仍缺任一来源则 needs_clarification、complete=false、cacheEligible=false、resolvedReferences=false。只有从已校验的历史/当前页实际解析完成、且规范问题能独立理解时才输出 resolvedReferences=true；私人化或动态页面依赖不能因改写而变为共享资格。缺该字段按 false。本域将对 Planner 文本结果严格 schema 解析并补入可信原问题、语言和上下文指纹；浏览器公共入口不接受这些字段。当前 PlanQuestion 模型流程尚未实现，完整 ExecuteTurn 仍未就绪；确定性 EvaluateCacheEligibility 已按此规则编写，不做关键词扫描。

已编写的 HTTP 映射交平台 NewServiceHandler 统一身份、委托、权限、恢复代次、维护和请求结构校验。第一批只有实际已完成处理函数映射，完整 BuildApplication 仍须因缺其他操作而启动失败，不能运行一个缺路由的服务冒充交付。所有编译、生成与检查由根及时送 GitLab；本域不执行本机 Go、测试、模型或服务。

第二批SSE映射改用现成 `platform/apivalidate.ServeJSONEventStream`，本域只提供持久事件Next/Close；15秒心跳、30秒有界背压及关闭交平台。HTTP连接还受五分钟上限和更早上游ctx约束。没有在本域复制通用SSE机制。

完整ExecuteTurn/RunWorker/恢复仍未完成；调用链必须在首次执行固定每步记忆/输入及步骤幂等身份，并保存原MG调用引用，恢复费用未知步骤只查询原调用、不重新解析新记忆后发新请求。本批参考级前置/提示/检索消费者可以使用真实已装配依赖执行，不代表该完整调度链已经交付。

第三批补齐语义缓存、事件失效及轮次终态/取消/会话删除入口。完整StartTurn/ExecuteTurn/worker仍未装配，不把上述消费者就绪宣称成可部署端到端。取消/删除下游故障需按原请求幂等重放清理，不能跳过失败或把未知模型状态归为免费完成。译缓存仍等待localization提供真实规则版本来源，未增加协议或伪造translationVersionHash。

qa.conversation为免费准入：消费前后token为空，仍需人机验证及持久消费，创建提交/重放后释放原slot。qa.ask/followup只有消费后的非空签名回执才可进入付费链。保留原root/owner/site/epoch，不能把免费元数据当MG许可。
