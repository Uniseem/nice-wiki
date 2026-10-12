# lobe-i18n（Lobe CLI Toolbox）

> 用模型翻译多语言 JSON 文件和 Markdown 文档的命令行工具。

| 项目 | 信息 |
|---|---|
| 方向 | 翻译 |
| 仓库 | lobehub/lobe-cli-toolbox |
| 许可证 | MIT |
| 主要语言 | TypeScript |
| Star | 0.4k |
| 最近更新 | 2026-10 |

## 它是什么

LobeHub 团队的命令行工具集之一，用于自动化多语言工作流，支持 JSON 语言包和 Markdown 文档。

## 和我们相关的部分

- README 列出的能力：大文件自动切分、增量更新、可定制模型参数。
- 配置项 `splitToken` 按 token 数切分语言包。
- Markdown 有两种处理方式：整篇当作字符串翻译；或者先解析成语法树（mdast），只把需要翻译的文字交给模型，尽量去掉 Markdown 语法和链接。

## 借鉴什么

- "先解析成语法树、只把文字交给模型"——和我们按块、并用占位标签保护行内格式的思路一致，印证了这条路是对的。

## 注意

- README 没有说明"内容修改了的键"如何处理，增量的细节未核实。

## 资料

- [仓库](https://github.com/lobehub/lobe-cli-toolbox)
- [npm 包](https://www.npmjs.com/package/@lobehub/i18n-cli)
