# 统一模型网关运行与恢复

调用总deadline180秒、连接10秒、首token30秒、空闲20秒；供应方超时、限流可按有序备用切换，但新attempt独立预留，原未知收费保守保留held。格式输出失败原模型最多一次重试再备用；每模型fallback最多一次，全链有限且不重置deadline。未知账单每5分钟查询最多12次后人工核对，额度仍held。models.dev目录离线内置且每日更新，手动覆盖优先；初始化无模型要求。

本域deadline、取消、错误和幂等写在逐函数合同；所有耗时步骤前后检查cancel。无副作用读取最多3次transport重试，等待500/1000/2000ms但不得超总deadline；模型按old有限备用/格式重试/翻译降级创建新attempt并重新预留费用；旧未知收费保留held，不把旧请求重放或误记0费。

本地屏障与control.db：屏障停止新预留/新派发；在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封；部署主密钥不导出。缺密钥恢复后所有供应方disabled。 control信任锚/许可/旧指针/屏障不属于业务快照。restore许可校验失败拒写，屏障到期不自动释放，管理员经完整rollback/epoch校验后释放。

诊断只输出request/root/object IDs、版本、耗时、状态和公共错误码；禁止secret/prompt/记忆正文/问题正文/IP。业务文本只在授权私有业务表保存。备份对象按ObjectRef SHA256和byteSize验证，schema1以外拒绝；临时stage仅在本域私有目录创建，禁止任意路径导入。

可观测指标：请求量/错误率/延迟、活动lease、outbox积压、屏障状态、stale_result数量；费用显示调用MG queryUsage。健康ready需DB/schema/关键依赖可用，配置未设某AI能力为degraded且明确禁派发。
