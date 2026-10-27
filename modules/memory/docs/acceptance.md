# 长期记忆、任务映射与记忆助手验收行为规格

设计状态：未实现、未运行测试、未调用真实接口或模型。以下为未来验收要求，未标记通过。

- [ ] M11-R01 原始人工记忆是本服务权威数据；其他服务仅请求MemoryContext，不读文件/表、不缓存无版本正文。

- [ ] M11-R02 每个context固定entry revisions、mapping revision、语言fallback、renderer version；versionHash为规范排序manifest与正文SHA256。

- [ ] M11-R03 单条默认1536UTF8bytes可单独调整，单任务注入总量默认5120UTF8bytes；人工写条目或映射前投影全部受影响任务，任何超预算拒绝保存并指出taskId，禁止静默丢条目。

- [ ] M11-R04 助手只生成proposal，不写active entry或glossary；批准与拒绝必须由人工memory.manage，助手服务身份不能自批准。

- [ ] M11-R05 术语提议批准产生发送localization批准命令的outbox；localization终态事件异步更新proposal，不同步回调resolveContext。

- [ ] M11-R06 人工版本变更后旧助手结果只能superseded；expectedEntryVersion/expectedMappingRevision CAS禁止覆盖新编辑。

- [ ] M11-R07 记忆不保存供应方密钥、费用余额、匿名IP或完整访客问答；导入条目必须人工确认正文来源。

- [ ] M11-R08 entry正文权威为本域Markdown文件含frontmatter，数据库仅索引；readonly条目助手只能建议人手动修改。

- [ ] 同幂等键同payload不产生重复副作用，不同payload返回CONFLICT。
- [ ] CAS冲突/旧租约/旧dataEpoch不修改新状态。
- [ ] 取消贯穿所有子调用；收费未知仍可追踪原MG invocation。
- [ ] 快照暂存与丢弃可重放；暂存失败不改active目录，切换后可整体换回。
- [ ] JSON额外字段/null、越界值、非法权限均返回公共Problem。
