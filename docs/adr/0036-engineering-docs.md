# D36. 文档分类与过程记录（push-log）

- 状态：✅ 已定
- 日期：2026-10-12

详见 [design/engineering.md](../design/engineering.md)。

- 文档按生命周期分类：
  - 设计文档：设计变了就改；
  - 决策记录：只追加；
  - 实施计划：任务完成时打勾；
  - 状态快照 `STATUS.md`：随时覆盖；
  - 操作手册、更新日志。
- 决策记录一条一个文件，放在 `docs/adr/`，编号保持 `Dn` 的写法，有索引表。
- 过程记录以 push 为单位，存放在 `docs/push-log/`，文件名为 `日期-序号.md`（例如 `2026-10-20-0001.md`），序号全局递增。
  - **push 严格由用户手动触发**，AI 助手绝不主动 push。
  - 两次 push 之间，每次会话的记录先追加到 `docs/push-log/_pending.md`，push 时整理成正式的记录文件。
  - 另外维护索引 `docs/push-log/INDEX.md`。
