# 统一模型网关验收行为规格

设计状态：未实现、未运行测试、未调用真实接口或模型。以下为未来验收要求，未标记通过。

- [ ] M07-R01 费用金额使用非负整数microUSD；汇率/价目快照固定在invocation，显示货币不参与结算。

- [ ] M07-R02 rootOperationId、costClass、tenantId只能来自认证的服务上下文；浏览器字段不得覆盖。

- [ ] M07-R03 供应方已接收后超时/取消可能已收费；进入charge_unknown并保留最大费用预留，绝不按零收费结算。

- [ ] M07-R04 档位保留有序主/备用模型列表；每模型最多一次fallback attempt，格式校验允许原模型重试一次；每次新attempt必须新费用预留，旧charge_unknown金额继续held，总deadline不重置。

- [ ] M07-R05 密钥写入AES-256-GCM密文，主密钥由部署secret注入；任何API、事件、日志、备份清单不得含明文密钥。

- [ ] M07-R06 embeddingSpaceId等于供应方ID、不可变模型版本、维度、预处理版本与归一化规则的规范JSON SHA-256；模型显示名不参与身份判断。

- [ ] M07-R07 供应方、密钥、五档及任务模型分配仅superadmin可修改（D53）；AI未配置不得阻塞站点初始化或文档功能（D55）。

- [ ] M07-R08 缺价格允许保存与调用且显示原产品warning：该调用不能完整计入金额上限；使用量保留为price_unknown，amount缺失而非0，供应方返回实际金额后优先记账。QA日额度0为不限，background独立统计。

- [ ] 同幂等键同payload不产生重复副作用，不同payload返回CONFLICT。
- [ ] CAS冲突/旧租约/旧dataEpoch不修改新状态。
- [ ] 取消贯穿所有子调用；收费未知仍可追踪原MG invocation。
- [ ] 非released屏障拒绝新业务写；限定恢复许可允许七项控制操作且不能读写普通业务。
- [ ] 快照验证、prepare、commit、rollback可重放；失败保持屏障与旧指针可恢复。
- [ ] JSON额外字段/null、越界值、非法权限均返回公共Problem。
