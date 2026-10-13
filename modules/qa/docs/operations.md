# 问答、引用与回答缓存运行与恢复

默认问题500字，前置500token、核查800、回答1500；最多4子问题、3检索轮、5工具调用，每会话1活跃轮次其余排队。总deadline180秒；recordRetentionDays默认30可调；debug7天；SSE24小时。压缩取preflight/answer较小窗口、预留10%、最近15000token原样保留且增量summary；输入过长拒绝允许压缩后新attempt一次，费用由MG独立预留。cache普通默认7天，精选免TTL/记忆版本变化，引用有效性永不豁免。

本域deadline、取消、错误和幂等写在逐函数合同；所有耗时步骤前后检查cancel。无副作用读取最多3次transport重试，等待500/1000/2000ms但不得超总deadline；模型按old有限备用/格式重试/翻译降级创建新attempt并重新预留费用；旧未知收费保留held，不把旧请求重放或误记0费。

本地屏障与control.db：屏障停止新turn/cache审批/压缩提交；取消活动turn并持久终态和原MG调用引用；导入的visitor cookie不得恢复授权，恢复dataEpoch使旧流/permit失效，原消息只经重新授权可读。 control信任锚/许可/旧指针/屏障不属于业务快照。restore许可校验失败拒写，屏障到期不自动释放，管理员经完整rollback/epoch校验后释放。

诊断只输出request/root/object IDs、版本、耗时、状态和公共错误码；禁止secret/prompt/记忆正文/问题正文/IP。业务文本只在授权私有业务表保存。备份对象按ObjectRef SHA256和byteSize验证，schema1以外拒绝；临时stage仅在本域私有目录创建，禁止任意路径导入。

可观测指标：请求量/错误率/延迟、活动lease、outbox积压、屏障状态、stale_result数量；费用显示调用MG queryUsage。健康ready需DB/schema/关键依赖可用，配置未设某AI能力为degraded且明确禁派发。
