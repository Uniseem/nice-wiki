# traffic-guard 运维与故障

风控不可达：新 QA/高成本导出/模型探测 503，不启用客户端自判；静态公开阅读只受入口基本限流，禁止调用模型。

pepper/签名密钥不可用：不签 permit，恢复调用方得到503；日志只有errorCode/requestId。

jobs不可达：outbox持久重试；不把事件派发失败当原事务失败或重扣令牌。

恢复后旧permit/挑战因epoch失配拒绝，保留未到期匿名ban；缺pepper时维护不退出。

启动：先校验config/schema/外部secret引用→恢复本域journal→读取site-config runtime→核active epoch→恢复控制日志是否有未完成的切换决定→注册ready。任一步关键校验失败则unavailable，不能开放写入；maintenance中仅允许已登记控制操作。关闭：拒新写/claim→取消新外部调用→在途到持久安全点→flush journal→退出；30秒未安全保持durable恢复记录并返回不可安全停机状态。

监控仅模块、errorCode、requestId、状态、队列长度/水位/耗时；不记录IP/问题正文/模型输入输出/secret/token。运维告警来自安全引用事件。每次文件操作检查绝对路径位于本域允许根，拒symlink/traversal。

control区不随恢复替换。切换后故障可继续等待或整体换回，不允许部分回滚。免费派生索引清空可重建，已付费结果引用必须保留，不能自动发模型请求。

本阶段没有真实服务/模型/Git远程操作，没有运行业务验收。
