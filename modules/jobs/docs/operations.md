# jobs 运维与故障

owner停机：lease到期标orphan等待owner确认结果，不自动执行不可证明未完成的付费阶段。

jobs停机：领域同步读取仍按本域权限；新异步工作明确503，领域outbox保留。

事件consumer停机：该consumer delivery退避/死信，其他consumer独立继续。

恢复旧epoch拒绝所有旧job/fence/event；定时器从新epoch未来tick继续，已有paid结果refs只读复用。

启动：先校验config/schema/外部secret引用→恢复本域journal→读取site-config runtime→核active epoch→恢复控制日志是否有未完成的切换决定→注册ready。任一步关键校验失败则unavailable，不能开放写入；maintenance中仅允许已登记控制操作。关闭：拒新写/claim→取消新外部调用→在途到持久安全点→flush journal→退出；30秒未安全保持durable恢复记录并返回不可安全停机状态。

监控仅模块、errorCode、requestId、状态、队列长度/水位/耗时；不记录IP/问题正文/模型输入输出/secret/token。运维告警来自安全引用事件。每次文件操作检查绝对路径位于本域允许根，拒symlink/traversal。

control区不随恢复替换。切换后故障可继续等待或整体换回，不允许部分回滚。免费派生索引清空可重建，已付费结果引用必须保留，不能自动发模型请求。

本阶段没有真实服务/模型/Git远程操作，没有运行业务验收。
