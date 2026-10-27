# 语言、翻译与术语验收行为规格

设计状态：未实现、未运行测试、未调用真实接口或模型。以下为未来验收要求，未标记通过。

- [ ] M10-R01 语言启用不等于自动调用模型；只有已配置翻译档位且自动翻译开启的发布事件触发翻译。草稿绝不调用AI。

- [ ] M10-R02 每次结果提交必须CAS sourceRevision、targetGeneration、humanGeneration、glossaryGeneration、memoryVersionHash和dataEpoch；旧任务不能覆盖新任务或人工内容。

- [ ] M10-R03 人工保护按unitId标记；自动翻译不得改写protected=true的target，无论force或重试。解除保护只能显式人工命令。

- [ ] M10-R04 译文不覆盖content权威正文；源撤回立即停交付，读取结果前仍验证当前发布引用。

- [ ] M10-R05 术语批准命令仅来自memory已批准提议或人工管理；处理器不同步回调memory，结果通过持久事件回执。

- [ ] M10-R06 界面字典固定key/占位符不被模型新增或破坏；语言建议采用浏览器语言+时区+本地离线IP国家信号，快速模型缓存判断且超时规则兜底，不保存IP。

- [ ] M10-R07 基准language URL不强制跳转；根路径语言选择由public-site处理。无明确选择/拒绝时才建议，置信度默认≥0.8，建议语言提示文本并保持同文档位置。

- [ ] M10-R08 D47翻译流程：20字以上同hash翻译记忆复用，首次翻译自动术语直接入库且manual优先，可按抽取batch撤销；默认1000预计输出token顺序批，反思润色默认关闭。

- [ ] 同幂等键同payload不产生重复副作用，不同payload返回CONFLICT。
- [ ] CAS冲突/旧租约/旧dataEpoch不修改新状态。
- [ ] 取消贯穿所有子调用；收费未知仍可追踪原MG invocation。
- [ ] 快照暂存与丢弃可重放；暂存失败不改active目录，切换后可整体换回。
- [ ] JSON额外字段/null、越界值、非法权限均返回公共Problem。
