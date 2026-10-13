# 第三方声明

目前没有复制进仓库的第三方源代码。通过包管理器安装的依赖保留各自许可；依赖不等于已借用源代码。

已获设计授权的候选：Docmost 社区版扩展（AGPL-3.0）、rocicorp 分数索引（CC0）。真正借用时登记具体文件、版本或提交及完整版权声明，不能提前声称已经借用。Halo 等 GPL 项目只参考思路。

新增借用项目先确认，核对与 AGPL-3.0 兼容的许可。公开站点发布时提供对应源码入口。

## KaTeX

公开页面使用 KaTeX 0.16.28 渲染公式。KaTeX 依据 MIT License 发布，版权归 Khan Academy 及其贡献者所有：<https://github.com/KaTeX/KaTeX>。

## Excalidraw

后台画板按需加载 Excalidraw 0.18.1，并通过 React 19.3.0 和 ReactDOM 19.3.0 挂载。三者依据 MIT License 发布；本仓库没有复制其源文件。来源：<https://github.com/excalidraw/excalidraw>、<https://github.com/facebook/react>。

## models.dev

`internal/llm/catalog_snapshot.json` 的模型资料由 models.dev API 生成。models.dev 依据 MIT License 发布，Copyright (c) 2025 models.dev：<https://github.com/anomalyco/models.dev>。

## DB-IP Lite

IP 国家信息使用 DB-IP Lite 数据库。DB-IP Lite 依据 CC BY 4.0 发布，Copyright (c) DB-IP.com：<https://db-ip.com/db/lite.php>。公开站点页脚保留数据来源署名。

数据库通过 maxminddb-golang v2.5.0 读取。该库依据 ISC License 发布，Copyright (c) 2015–2026 Gregory Oschwald：<https://github.com/oschwald/maxminddb-golang>。
