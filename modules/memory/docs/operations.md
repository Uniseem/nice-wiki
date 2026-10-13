# 长期记忆、任务映射与记忆助手运行与恢复

默认单条1536bytes、单任务5120bytes；超投影预算拒保存而非静默截注入。resolveContext2秒不调模型。助手精确档，逐条new/update/delete/noop差异经人工批准，readonly只建议直接编辑。project overview每日自动任务及累计发布门槛默认20篇提前更新，人工修改关闭autoUpdate直到显式重开。助手消息保留默认30天可调、候选90天。记忆规则变更不自动重译，由后台显示quote后人工选择语言范围；普通QA缓存失效，精选记忆豁免。

本域deadline、取消、错误和幂等写在逐函数合同；所有耗时步骤前后检查cancel。无副作用读取最多3次transport重试，等待500/1000/2000ms但不得超总deadline；模型按old有限备用/格式重试/翻译降级创建新attempt并重新预留费用；旧未知收费保留held，不把旧请求重放或误记0费。

本地屏障与control.db：屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令；导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。 control信任锚/许可/旧指针/屏障不属于业务快照。restore许可校验失败拒写，屏障到期不自动释放，管理员经完整rollback/epoch校验后释放。

诊断只输出request/root/object IDs、版本、耗时、状态和公共错误码；禁止secret/prompt/记忆正文/问题正文/IP。业务文本只在授权私有业务表保存。备份对象按ObjectRef SHA256和byteSize验证，schema1以外拒绝；临时stage仅在本域私有目录创建，禁止任意路径导入。

可观测指标：请求量/错误率/延迟、活动lease、outbox积压、屏障状态、stale_result数量；费用显示调用MG queryUsage。健康ready需DB/schema/关键依赖可用，配置未设某AI能力为degraded且明确禁派发。
