# public-site 运维与故障

content 停机：所有正文/导航/sitemap/llms/Markdown 返回 503，不能把旧 cache 当安全授权；静态 CSS/图标可继续读。

media 不可达：可公开正文使用固定尺寸占位且不输出未知资源 URL；受保护预览资源失败仍不落公开缓存。

analytics 停机：匿名 beacon 丢弃；HTML 不因此失败。

public-site 重启：扫描 cache 索引文件哈希与遗留 tmp，损坏命中转 miss；authority 检查始终启用。

启动：先校验config/schema/外部secret引用→恢复本域journal→读取site-config runtime→核active epoch→恢复控制日志是否有未完成的切换决定→注册ready。任一步关键校验失败则unavailable，不能开放写入；maintenance中仅允许已登记控制操作。关闭：拒新写/claim→取消新外部调用→在途到持久安全点→flush journal→退出；30秒未安全保持durable恢复记录并返回不可安全停机状态。

监控仅模块、errorCode、requestId、状态、队列长度/水位/耗时；不记录IP/问题正文/模型输入输出/secret/token。运维告警来自安全引用事件。每次文件操作检查绝对路径位于本域允许根，拒symlink/traversal。

control区不随恢复替换。切换后故障可继续等待或整体换回，不允许部分回滚。免费派生索引清空可重建，已付费结果引用必须保留，不能自动发模型请求。

本阶段没有真实服务/模型/Git远程操作，没有运行业务验收。

resolvePublishedLink只允许已认证site-config纯服务主体；拒其他服务、访客及用户委托。任何content/配置/语言权威未知返回503，current ref或规范路径水位改变为STALE_RESULT/409，不返回visible=false掩盖依赖故障。响应no-store，不开放草稿/撤回历史，不触发翻译、模型、任务或付费。函数已编写，尚未编译，也未运行真实接口。
