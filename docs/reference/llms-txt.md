# llms.txt 规范

> "让大模型读懂网站"的约定：根目录放一份 Markdown 索引，页面提供 .md 版本。

| 项目 | 信息 |
|---|---|
| 方向 | 整体架构 |
| 仓库 | AnswerDotAI/llms-txt |
| 许可证 | Apache-2.0 |
| 主要语言 | Jupyter Notebook |
| Star | 2.7k |
| 最近更新 | 2026-09 |

## 它是什么

Answer.AI 提出的约定，我们 D32 就是按它设计的。

## 和我们相关的部分

- `/llms.txt`：Markdown 格式的站点索引，列出各页面的链接和简介。
- 页面的 Markdown 版本放在同一地址，加 `.md` 后缀（`page.html.md`）或把扩展名换成 `.md`（`page.md`）。
- 建议通过 HTTP 响应头告诉客户端：`Link: </docs/page.html.md>; rel="alternate"; type="text/markdown"`。

## 借鉴什么

- 除了 HTML 里的 `<link>`，再在 HTTP 响应头里加上 `Link`，让不解析 HTML 的客户端也能找到 Markdown 版本。

## 资料

- [仓库](https://github.com/AnswerDotAI/llms-txt)
- [规范网站](https://llmstxt.org/)
