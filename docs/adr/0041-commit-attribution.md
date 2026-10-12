# D41. 提交署名：用户是作者，AI 模型是合著者

- 状态：✅ 已定
- 日期：2026-10-12

详见 [design/engineering.md](../design/engineering.md) 的"提交署名"一节。

- 提交的作者是用户本人的 git 身份（GitHub 账号 Uniseem）。AI 助手不能修改 git 配置。
- 每个提交末尾都用 `Co-Authored-By` 如实写明本次会话实际使用的模型和版本，例如 `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
  - 邮箱必须对应这个 AI 在 GitHub 上的账号，这样才会出现在仓库的贡献者统计里。
  - 不确定自己是哪个模型时，先问用户。
  - 多个模型参与同一个提交时，各写一行。
- push-log 里记录这次 push 涉及哪些模型。
- 第一次 push 后确认贡献者列表里有 AI 的账号；如果没有，改为"AI 作为提交作者、用户作为提交者"。

**补充（2026-10-12）**：push 0001 之后确认，GitHub 把两个身份都识别了出来（Uniseem 和 claude），但仓库的贡献者列表只统计提交作者，只显示 Uniseem。用户决定**保持现状**，不改用 AI 作为作者。
