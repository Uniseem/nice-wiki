# data-protection 运维与故障

任一必需域停机：备份/恢复不进入ready/completed；maintenance保持失败状态等待明确continue或commit前abort。

密钥provider停机：不创建不可解密ready包，不泄漏密钥；已有密文保存。解密校验失败阻止prepare。

site-config停机：不能打开/切代/退出maintenance；各服务readiness fail closed，不假设缓存epoch可继续写。

data-protection停机：已开始maintenance由runtime持久维持；各域不自己解除。重启根据control journal续接。

远程Git停机或分叉：backup ready保持，mirror标failed/conflict；无force push、无真实push在本阶段。

启动：先校验config/schema/外部secret引用→恢复本域journal→读取site-config runtime→核active epoch→恢复控制日志是否有未完成commit→注册ready。任一步关键校验失败则unavailable，不能开放写入；maintenance中仅允许已登记控制操作。关闭：拒新写/claim→取消新外部调用→在途到持久安全点→flush journal→退出；30秒未安全保持durable恢复记录并返回不可安全停机状态。

监控仅模块、errorCode、requestId、状态、队列长度/水位/耗时；不记录IP/问题正文/模型输入输出/secret/token。运维告警来自安全引用事件。每次文件操作检查绝对路径位于本域允许根，拒symlink/traversal。

恢复：全域snapshot完整→validate全部→prepare全部→持久commit→runtime epoch CAS→activate全部→readiness全部→解除maintenance。恢复许可在进入维护前签发，本次control/barrier/trust-anchor不被导入替换。commit后故障只允许继续，不允许部分回滚。免费派生索引清空可重建，已付费结果引用必须保留，不能自动发模型请求。

本阶段没有真实服务/模型/Git远程操作，没有运行业务验收。
