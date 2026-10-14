# 知识入库与混合检索验收行为规格

设计状态：未实现、未运行测试、未调用真实接口或模型。以下为未来验收要求，未标记通过。

- [ ] M08-R01 草稿事件不得创建AI任务；只有已启用且已配置嵌入档位的发布修订创建build。

- [ ] M08-R02 索引不是正文权威来源；每次检索在返回前向content验证PublishedRevisionRef。权威不可用必须失败关闭。

- [ ] M08-R03 staging构建独立于active，只有当前发布修订、空间和dataEpoch都匹配才CAS激活；旧任务标superseded。

- [ ] M08-R04 向量空间按公共EmbeddingSpace全字段及spaceId校验；模型同名、维度相同仍不得混搜。

- [ ] M08-R05 媒体提取必须经media资源授权且保留asset hash；提取文本不得覆盖content人工正文。

- [ ] M08-R06 每个子模型请求继承可信rootOperationId、costClass、deadline、cancellationId；服务不保存余额或费用账本。

- [ ] M08-R07 只给基准语言建索引；前置层把查询改写为基准语言，访客回答语言不建立第二份向量空间。

- [ ] 同幂等键同payload不产生重复副作用，不同payload返回CONFLICT。
- [ ] CAS冲突/旧租约/旧dataEpoch不修改新状态。
- [ ] 取消贯穿所有子调用；收费未知仍可追踪原MG invocation。
- [ ] 非released屏障拒绝新业务写；备份恢复控制操作只接受data-protection服务身份与superadmin委托。
- [ ] 快照暂存与丢弃可重放；暂存失败不改active目录，切换后可整体换回。
- [ ] JSON额外字段/null、越界值、非法权限均返回公共Problem。
