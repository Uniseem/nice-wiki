> 产品需求基线快照：来源 old:docs/adr/0048-memory-refinements.md，blob 363ace859adcaafd18091307e3eaa5a7b37fe229。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D48. 长期记忆的细化：单条上限、说明、只读、逐条判断

- 状态：✅ 已定
- 日期：2026-10-12
- 补充：D46

详见 [design/memory.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/memory.md)。依据见 [reference/README.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/reference/README.md) 的记忆部分。

- 每个条目默认不超过 1.5KB，可单独调整；每个任务的注入总量仍不超过 5KB（参考 Letta）。
- 注入时每个条目前带上它的说明（参考 Letta）。
- 条目可以设为只读：记忆助手不能修改，只能建议人手动修改（参考 Letta）。
- 记忆助手整理时逐条判断新增、更新、删除、不变（参考 mem0），结果以差异形式确认。
