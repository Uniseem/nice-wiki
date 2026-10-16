# 统一模型网关运行与恢复

调用总deadline180秒、连接10秒、首token30秒、空闲20秒；供应方超时、限流可按有序备用切换，但新attempt独立预留，原未知收费保守保留held。格式输出失败原模型最多一次重试再备用；每模型fallback最多一次，全链有限且不重置deadline。未知账单每5分钟查询最多12次后人工核对，额度仍held。models.dev目录离线内置且每日更新，手动覆盖优先；初始化无模型要求。

本域deadline、取消、错误和幂等写在逐函数合同；所有耗时步骤前后检查cancel。无副作用读取最多3次transport重试，等待500/1000/2000ms但不得超总deadline；模型按old有限备用/格式重试/翻译降级创建新attempt并重新预留费用；旧未知收费保留held，不把旧请求重放或误记0费。

本地屏障与control.db：屏障停止新预留/新派发；在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封；部署主密钥不导出。缺密钥恢复后所有供应方disabled。 control区（屏障、恢复日志）不属于业务快照。屏障到期不自动释放，管理员确认恢复已完成或整体换回后释放。

诊断只输出request/root/object IDs、版本、耗时、状态和公共错误码；禁止secret/prompt/记忆正文/问题正文/IP。业务文本只在授权私有业务表保存。备份对象按ObjectRef SHA256和byteSize验证，schema1以外拒绝；恢复暂存仅在本域restore/<restoreId>目录创建，禁止任意路径导入。

可观测指标：请求量/错误率/延迟、活动lease、outbox积压、屏障状态、stale_result数量；费用显示调用MG queryUsage。健康ready需DB/schema/关键依赖可用，配置未设某AI能力为degraded且明确禁派发。

## 主密钥引用与回执协议

本域ServiceConfig可选masterKeyRef（部署secret引用），环境覆盖为NW_MASTER_KEY_REF；示意引用只登记在本域manifest，实际secret由部署私有注入，不进入仓库或data git。缺引用不阻断免费读取和启动装配，凭据写入失败关闭。主密钥文件必须正规单硬链接、无符号链接，内容恰好32字节；只在写凭据时安全有界读取，文件替换/长度或mtime变化失败关闭。

本轮不支持部署主密钥轮换：AES密文与7天敏感输入回执都依赖同一部署主密钥。需要换主密钥时必须另行设计密文及回执迁移，禁止自动重签或覆盖旧结果。Put/Rotate的旧供应方密钥版本不是部署主密钥轮换，旧密文仍保留供后续派发引用；引用计数/释放与销毁尚未交付。
