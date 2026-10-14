# 语言、翻译与术语运行与恢复

发布触发已配置翻译，基准正文之外标题/目录树/站点名称导航/UI字典同流水线；图片内文字和附件内容不翻译。默认1000output token均衡批按文档顺序，跨文档并行；20字以上同hash TM复用；自动术语首次直接入库manual优先可整批撤销；术语不符带规定译法重译1次仍不符review；反思默认off。校验对半→逐块，长度上限先缩context再重试，所有paid attempt经过MG新预留，旧unknown继续held。译文权威文件原子manifest激活；未完成旧译文继续显示，新块原文顶部提示。语言建议5秒fast模型+缓存+规则fallback不存IP。

本域deadline、取消、错误和幂等写在逐函数合同；所有耗时步骤前后检查cancel。无副作用读取最多3次transport重试，等待500/1000/2000ms但不得超总deadline；模型按old有限备用/格式重试/翻译降级创建新attempt并重新预留费用；旧未知收费保留held，不把旧请求重放或误记0费。

本地屏障与control.db：屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本；导入必须恢复保护位与源ref，恢复切换完成前译文不能交付，不自动重翻。 control区（屏障、恢复日志）不属于业务快照。屏障到期不自动释放，管理员确认恢复已完成或整体换回后释放。

诊断只输出request/root/object IDs、版本、耗时、状态和公共错误码；禁止secret/prompt/记忆正文/问题正文/IP。业务文本只在授权私有业务表保存。备份对象按ObjectRef SHA256和byteSize验证，schema1以外拒绝；恢复暂存仅在本域restore/<restoreId>目录创建，禁止任意路径导入。

可观测指标：请求量/错误率/延迟、活动lease、outbox积压、屏障状态、stale_result数量；费用显示调用MG queryUsage。健康ready需DB/schema/关键依赖可用，配置未设某AI能力为degraded且明确禁派发。
