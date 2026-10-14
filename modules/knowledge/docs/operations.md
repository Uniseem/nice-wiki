# 知识入库与混合检索运行与恢复

发布构建deadline30分钟，单资源本地提取60秒，单MG调用180秒；取消标记每块/每10秒检查。未配置embedding时入库skipped并管理页说明；检索只能显式lexicalOnly降级，不可隐式换空间。磁盘low watermark10%停止新build并保留active。

本域deadline、取消、错误和幂等写在逐函数合同；所有耗时步骤前后检查cancel。无副作用读取最多3次transport重试，等待500/1000/2000ms但不得超总deadline；模型按old有限备用/格式重试/翻译降级创建新attempt并重新预留费用；旧未知收费保留held，不把旧请求重放或误记0费。

本地屏障与control.db：屏障停止新build、active切换和staging写入，等待reader lease排空；导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建；派生索引可由管理员另发重建任务。 control区（屏障、恢复日志）不属于业务快照。屏障到期不自动释放，管理员确认恢复已完成或整体换回后释放。

诊断只输出request/root/object IDs、版本、耗时、状态和公共错误码；禁止secret/prompt/记忆正文/问题正文/IP。业务文本只在授权私有业务表保存。备份对象按ObjectRef SHA256和byteSize验证，schema1以外拒绝；恢复暂存仅在本域restore/<restoreId>目录创建，禁止任意路径导入。

可观测指标：请求量/错误率/延迟、活动lease、outbox积压、屏障状态、stale_result数量；费用显示调用MG queryUsage。健康ready需DB/schema/关键依赖可用，配置未设某AI能力为degraded且明确禁派发。
