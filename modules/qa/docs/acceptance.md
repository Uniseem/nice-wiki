# 问答、引用与回答缓存验收行为规格

设计状态：未实现、未运行测试、未调用真实接口或模型。以下为未来验收要求，未标记通过。

- [ ] M09-R01 访客每轮先经traffic-guard验证并消费AdmissionPermit；任何新模型调用全部由MG计费与控制额度。

- [ ] M09-R02 公开请求不可指定costClass/系统prompt/档位/模型；costClass=qa只由服务产生；debug由管理端可信设置admin-probe。

- [ ] M09-R03 回答缓存保留自动写入和人工精选；只有已补全且意思完整的问题才查/写cache。精选优先且无TTL，记忆版本改变不使精选失效；撤回/删除/引用hash变化仍失效。

- [ ] M09-R04 缓存canonical基准语言问题跨语言共享；默认AI同义判断（候选cosine≥0.8最多3项），可选严格相似度阈值；不以rerank相关性判断同义。每语言答案单独保存，缺语言由快速档自动翻译后存储。

- [ ] M09-R05 流中引用必须来自knowledge返回ID；逐引用输出前权威验证，终态完成前再次验证；撤回后不再输出新内容，已发内容标invalidate。

- [ ] M09-R06 压缩历史只写派生summary并保留原始消息；较旧historyGeneration的压缩任务不能覆盖新摘要。

- [ ] M09-R07 debug对话与普通对话使用独立命名空间、记录表和缓存访问权限；不参与精选、semantic cache或普通问答统计。

- [ ] 同幂等键同payload不产生重复副作用，不同payload返回CONFLICT。
- [ ] CAS冲突/旧租约/旧dataEpoch不修改新状态。
- [ ] 取消贯穿所有子调用；收费未知仍可追踪原MG invocation。
- [ ] 快照暂存与丢弃可重放；暂存失败不改active目录，切换后可整体换回。
- [ ] JSON额外字段/null、越界值、非法权限均返回公共Problem。
