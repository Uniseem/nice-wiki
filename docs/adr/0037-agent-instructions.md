# D37. AI 助手说明文件分层

- 状态：✅ 已定
- 日期：2026-10-12

详见 [design/engineering.md](../design/engineering.md)。

- 根目录、`server/`、`web/` 各放一份 `AGENTS.md`，这是唯一真实的内容。
- 同一目录下的 `CLAUDE.md` 只有一行 `@AGENTS.md`，让 Claude Code 读到同一份内容。
- 说明文件只写看代码看不出来、做错了代价很大的内容：会话流程、完成的定义、不变量、禁止事项。细节放在 `docs/`，说明文件里只给链接。
- 需要同时修改多处的重复操作，写成操作手册，放在 `docs/playbooks/`。
