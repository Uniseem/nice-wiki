# analytics 运维与故障

analytics停机：匿名beacon丢弃；安全审计/费用事件由生产者outbox持久保留，不能把失联当已审计。

gateway停机：展示已有聚合同时标最后watermark/对账时间；不声称实时余额，不创建模型请求补数。

日志/trace统一仅requestId/errorCode/模块/状态/耗时；导出和审计严格allowlist排除敏感字段。

恢复后保留费用投影引用但标需对账；secret日salt/原始匿名token/IP不进入快照或git。

启动：先校验config/schema/外部secret引用→恢复本域journal→读取site-config runtime→核active epoch→恢复控制日志是否有未完成的切换决定→注册ready。任一步关键校验失败则unavailable，不能开放写入；maintenance中仅允许已登记控制操作。关闭：拒新写/claim→取消新外部调用→在途到持久安全点→flush journal→退出；30秒未安全保持durable恢复记录并返回不可安全停机状态。

监控仅模块、errorCode、requestId、状态、队列长度/水位/耗时；不记录IP/问题正文/模型输入输出/secret/token。运维告警来自安全引用事件。每次文件操作检查绝对路径位于本域允许根，拒symlink/traversal。

control区不随恢复替换。切换后故障可继续等待或整体换回，不允许部分回滚。免费派生索引清空可重建，已付费结果引用必须保留，不能自动发模型请求。

本阶段没有真实服务/模型/Git远程操作，没有运行业务验收。
