# 第三方声明

新main的R1起开始使用以下第三方依赖，选型与许可经ADR0005确认；未复制其源码进仓库。

- oapi-codegen v2.5.1 与 oapi-codegen/runtime v1.1.2（Apache-2.0）：make gen从各域openapi.json生成本域DTO与严格服务端接口，`go run`按固定版本调用；生成物重生成即可整体替换。
- openapi-typescript 7.10.1（MIT）：make gen生成浏览器API类型，`pnpm dlx`按固定版本调用。
- kin-openapi v0.133.0（Apache-2.0，匹配oapi-codegen v2.5.1依赖图）：生成代码的嵌入spec加载与请求/响应校验；自R1-01起随各域go.mod作为库依赖引入，libs/platform的apivalidate用其openapi3filter做入口请求校验；间接依赖以其go.mod为准（gorilla/mux等仅为其测试依赖，只记go.sum校验和，不编入程序）。
- modernc.org/sqlite v1.59.0（BSD-3，ADR0005已批准的纯Go SQLite驱动）：各域私有业务库存储引擎（identity、site-config、jobs、content、media、traffic-guard、localization、memory、model-gateway、knowledge、qa）；自R1-01私有持久化工作包起引入，间接依赖以其go.mod为准。
- golang.org/x/crypto v0.55.0（BSD-3-Clause）：identity口令规则使用Argon2id；仅通过模块依赖引用，不复制源码。
- golang.org/x/text v0.41.0（BSD-3-Clause，ADR0005已批准）：identity账号名NFC规范化、knowledge与qa文本NFKC规范化、qa语言标签BCP 47解析；仅通过模块依赖引用，不复制源码。

- 前端工程（2026-10-21 用户要求改为 React + 标准 HeroUI）：仓库根一个 pnpm 工程，版本以根目录 `pnpm-lock.yaml` 为准，只作依赖打包，不复制源码。
  - react / react-dom 19.3.1（MIT，Copyright (c) Meta Platforms, Inc. and affiliates），来源 https://github.com/facebook/react 。
  - @heroui/react、@heroui/styles 3.2.6（package.json 声明 MIT，包内 LICENSE 文件为 Apache-2.0 文本，作者 HeroUI），来源 https://github.com/heroui-inc/heroui/tree/v3 ；后台与公开页的组件和样式。
  - react-aria、react-aria-components、@react-aria/ssr、@react-aria/utils、@internationalized/date（Apache-2.0，Copyright Adobe），HeroUI 的对等依赖，来源 https://github.com/adobe/react-spectrum 。
  - tailwindcss、@tailwindcss/vite 4.3.3（MIT，Copyright Tailwind Labs, Inc.）与 tw-animate-css 1.4.0（MIT）：样式构建。
  - @gravity-ui/icons 2.22.0（MIT，Copyright (c) 2022 YANDEX LLC）：界面图标，来源 https://github.com/gravity-ui/icons 。
  - Tiptap 3.32.0（MIT，Copyright Tiptap GmbH）：@tiptap/core、react、starter-kit、extensions、extension-link、extension-table、extension-task-list、extension-task-item、extension-highlight、pm；文档编辑器。
  - react-markdown 10.1.0（MIT，Copyright (c) Espen Hovlandsdal）与 remark-gfm 4.0.1（MIT，Copyright (c) Titus Wormer）：公开页 AI 问答回答的 Markdown 渲染（含表格），不渲染 HTML，来源 https://github.com/remarkjs/react-markdown 、https://github.com/remarkjs/remark-gfm 。
  - vite 7.3.7、@vitejs/plugin-react 5.2.0（MIT）与 typescript 6.0.3（Apache-2.0，Microsoft）：构建与类型检查，只在构建时使用。

OpenAPI、JSON Schema、RFC等链接是协议参考，未复制其实现。设计提及Go/Vue等技术不代表已安装依赖；实施前逐项确认版本、许可、版权与复制范围，新增借用项目取得用户确认，再记录到本文件。GPL代码只参考，不复制；Docmost社区版编辑器扩展是唯一例外（AGENTS与ADR0005）。

- golang.org/x/net v0.58.0 / idna（BSD-3-Clause，Copyright 2009 The Go Authors）：平台公开地址IDNA Lookup规范化，仅模块引用、不复制代码。实施时读取官方Go模块原LICENSE核对；该版本为当前Go1.25可兼容的最高发布版，较新版本要求更高工具链。传递依赖由VPS go命令锁定，不手写go.sum。

- github.com/alecthomas/chroma/v2 v2.20.0（MIT，ADR0005已选）：公开代码高亮，来源 https://github.com/alecthomas/chroma/tree/v2.20.0 ，仅依赖不复制业务代码。
- katex 0.16.22（MIT，Copyright (c) 2013-2020 Khan Academy and other contributors）：公开页公式渲染与编辑器里的公式预览，来源 https://github.com/KaTeX/KaTeX/tree/v0.16.22 ，随 portal 静态文件（/_site/katex/）分发其脚本、样式、字体，不改动源码。

- github.com/tiktoken-go/tokenizer v0.7.0（MIT，Copyright (c) 2023 tiktoken-go）：经2026-10-18用户明确批准，用于模型网关显式绑定编码的离线BPE；固定来源 https://github.com/tiktoken-go/tokenizer/tree/v0.7.0 ，原许可 https://github.com/tiktoken-go/tokenizer/blob/v0.7.0/LICENSE 。仅Go模块依赖，不复制实现；发布程序保留依赖原LICENSE。
- github.com/ledongthuc/pdf v0.0.0-20260907135840-6c8c28e0e8a0（BSD-3-Clause，Copyright 2009 The Go Authors）：同次用户批准的PDF项目，实际固定版本为BSD三条款，非最初提问误称的MIT。来源 https://github.com/ledongthuc/pdf/tree/6c8c28e0e8a0 ，原许可 https://github.com/ledongthuc/pdf/blob/6c8c28e0e8a0/LICENSE 。用于有界隔离提取，仅模块依赖；保留原版权、许可与免责声明，不复制业务实现。

本仓保留新批准依赖的固定原许可全文：[tokenizer MIT](docs/licenses/tiktoken-go-tokenizer-v0.7.0.LICENSE)、[PDF BSD三条款](docs/licenses/ledongthuc-pdf-6c8c28e0e8a0.LICENSE)。随二进制分发本声明和这些原许可，不以链接替代应保留文本。

- github.com/oschwald/maxminddb-golang v1.13.1（ISC，ADR0005/D51已确认）：analytics只读取可信部署提供的本地MaxMind数据库，不下载数据库、不发送原始地址；libs/platform/geoip 用同一版本给 portal 查访客国家（语言建议的信号），只转国家代码，地址不出入口；固定来源 https://github.com/oschwald/maxminddb-golang/tree/v1.13.1 ，许可原文 https://github.com/oschwald/maxminddb-golang/blob/v1.13.1/LICENSE 。仅Go模块依赖，不复制源码；数据库内容不属于该库许可，仍由部署方自行提供合法数据。go.sum由GitLab Go工具生成。

## DB-IP Lite Country（IP 国家数据）

经用户 2026-10-21 确认，语言建议的 IP 国家信号使用 DB-IP 的免费 IP to Country Lite 数据库（MMDB 格式，按月更新），许可 CC BY 4.0（https://creativecommons.org/licenses/by/4.0/），署名要求为 “IP Geolocation by DB-IP”（https://db-ip.com）。数据文件不进仓库：本地由 `scripts/dev.py` 从 https://download.db-ip.com/free/ 下载当月版本，部署时由运维放置并在 portal 配置 `geoipCountryDBRef` 指向它；只在入口内存里查询国家代码，不保存地址。

## opencode（设计参考，未复制代码）

模型网关的“接入供应方 / 添加模型”流程参照 opencode 的做法：供应方和模型来自 models.dev 目录，接入时只填密钥，模型从目录带出名称、上下文和模态，热门供应方排前。来源 https://github.com/anomalyco/opencode （提交 4d5965762ec5e0d1a0b14630553fc62b51a11fba），许可 MIT，Copyright (c) 2025 opencode。用户于 2026-10-21 要求参考其实现、按许可自行决定照写或复制；opencode 的 MIT 比本仓库的 AGPL-3.0 宽松，本仓库用 Go 与 React 重写，未复制其源码或数据文件，因此无需随附其许可全文。

## models.dev 公开模型目录数据

用户于2026-10-18明确批准仅引入MIT公开目录JSON，未复制业务代码。来源[models.dev API](https://models.dev/api.json)，固定内容版本`sha256:80a19a3c78de1a7b0eb0a72e13e44df21180f0681d6cdf1dbd4c4c345fae3fb6`，原始5806411字节，取回时间2026-10-18T07:44:00.690256Z。API数据版本以实际内容hash固定，不声称某开发分支commit生成该数据。版权Copyright (c) 2025 models.dev，许可MIT，原文见[登记许可](docs/licenses/models-dev-MIT.txt)及内置目录LICENSE，source.json保留来源与ETag。

## go-git、go-billy 与 goldmark（站点导入、导出与 Git 同步）

2026-10-27 起 content 域使用以下三个 Go 模块，仅作模块依赖，不复制源码：

- github.com/go-git/go-git/v5 v5.16.3（Apache-2.0，Copyright 2018 Sourced Technologies, S.L.）：Git 同步在内存里克隆、提交、推送和拉取用户的仓库，不落盘、不调用 git 命令；出站 HTTP 用平台的防 SSRF 客户端。来源 https://github.com/go-git/go-git/tree/v5.16.3 ，原许可 [go-git Apache-2.0](docs/licenses/go-git-v5.16.3-Apache-2.0.txt)。版本就是 ADR0005 登记的 v5.16.3；更新的 v5.19 要求 Go 1.26，暂不升级（仓库工具链是 Go 1.25）。
- github.com/go-git/go-billy/v5 v5.6.2（Apache-2.0）：go-git 的内存文件系统，同上用于内存仓库。来源 https://github.com/go-git/go-billy/tree/v5.6.2 ，原许可 [go-billy Apache-2.0](docs/licenses/go-billy-v5.6.2-Apache-2.0.txt)。
- github.com/yuin/goldmark v1.8.6（MIT，Copyright (c) 2019 Yusuke Inuzuka）：导入时把 Markdown 解析成语法树再转成正文块（GFM 扩展：表格、删除线、任务列表）。来源 https://github.com/yuin/goldmark/tree/v1.8.6 ，原许可 [goldmark MIT](docs/licenses/goldmark-v1.8.6-MIT.txt)。

go-git 的传递依赖（ProtonMail/go-crypto BSD-3-Clause、skeema/knownhosts MIT、kevinburke/ssh_config MIT、jbenet/go-context MIT、cyphar/filepath-securejoin BSD-3-Clause 等）以 go.sum 为准，仅随模块引入。

## Rust 服务（检索 search 与公开入口 edge）

仓库里 `rust/` 是 Cargo 工作区，版本以 `rust/Cargo.lock` 为准，只作依赖，不复制源码；主要依赖与许可如下（传递依赖以锁文件为准，均为 MIT 或 Apache-2.0 一类宽松许可）：

- tantivy 0.22（MIT，Copyright (c) Paul Masurel）：检索服务的全文倒排索引与 BM25 打分；分词由调用方（Go 侧）完成，服务只接收已切好的词。来源 https://github.com/quickwit-oss/tantivy 。
- tokio 1（MIT）、hyper 1 与 hyper-util、http-body-util（MIT）、axum 0.7（MIT）、bytes（MIT）：两个服务的异步运行时与 HTTP。
- tokio-postgres 0.7 与 deadpool-postgres 0.14（MIT/Apache-2.0）：检索服务读写 pgvector 向量表。
- serde、serde_json、anyhow、hmac、sha2、hex、subtle（MIT/Apache-2.0）：序列化、错误处理与服务间共享密钥的 HMAC 认证。
- pgvector（PostgreSQL License，Copyright (c) 2021-2026 Andrew Kane）：向量类型与 HNSW 索引，随部署用的 `pgvector/pgvector:pg17` 镜像分发，不在本仓库里。

## 部署镜像用到的第三方镜像

编排直接使用 Docker 官方镜像：postgres 系 pgvector/pgvector:pg17（PostgreSQL License，含 pgvector）、caddy:2-alpine（Apache-2.0，Caddy 的按需证书用其自带的 on_demand_tls）、alpine 与 debian-slim（各自许可）。镜像里的软件按各自许可分发，本仓库不重新分发它们的源码。

## Excalidraw 与 React（后台编辑器画板）

后台内容编辑器的画板块使用 @excalidraw/excalidraw 0.18.1（MIT，Copyright (c) 2020 Excalidraw），与后台共用 react / react-dom 19.3.1。来源 https://github.com/excalidraw/excalidraw/tree/v0.18.1 ；仅作为 npm 依赖打包进后台应用，由根目录 pnpm-lock.yaml 固定，不复制其源码；画板代码只在用户打开画板时懒加载。画板用到的字体（Excalifont、Virgil、Cascadia Code、Comic Shanns、Liberation Sans、Lilita One、Nunito、小赖字体等，均随 @excalidraw/excalidraw 包分发，各自按 SIL OFL 或同类宽松许可发布）在构建时原样复制到后台静态目录（/_admin/excalidraw/），不改动、不单独再分发。

## ZCode 工具活动界面与 Lucide 图标

用户要求沿用 [ZCode 官方开源仓库](https://github.com/zai-org/ZCode) 的工具加载链与图标。参考版本为仓库 `package.json` 标示的 3.16.2（main 源码，无同名 tag）。问答工具摘要改编自 `packages/ui/src/ToolCallBlocks/ToolSummaryRow.tsx`、`ToolLayout.tsx` 及 read/search/explore 渲染器：保留静态搜索图标、运行文字扫光、悬停展开箭头及行内摘要；接入本站 SSE 与 HeroUI 主题，参数、分页和耗时在详情中展开。Copyright 2026 Z.AI Co., Ltd，Apache-2.0；原许可见 [ZCode Apache-2.0](docs/licenses/ZCode-Apache-2.0.txt)，适用声明见 [ZCode NOTICE 摘录](docs/licenses/ZCode-NOTICE.txt)。修改说明保留在组件文件头。

图标通过 `lucide-react` 1.17.0 依赖使用，与上述 ZCode 源码声明的版本一致；使用其 Search、Wrench、ChevronRight 及对话操作图标。Copyright (c) 2026 Lucide Icons and Contributors，ISC；源自 Feather 的图标另受 MIT 许可约束，Copyright (c) 2013-present Cole Bemis。来源 [Lucide 官方仓库](https://github.com/lucide-icons/lucide)，完整许可见 [Lucide / Feather 许可](docs/licenses/lucide-react-1.17.0-LICENSE.txt)。上述许可与声明随公开站静态资源分发。

## ZCode 原生运行时

M16 采用同一 3.16.2 main 源码中的 `@zcode/core`、`contracts`、`dynamic-workflow`、`shared-types`、`shared`、`model-option-map` 与公开 `zcode-cua` 占位包，存于 `libs/platform/zcode/upstream`。保留原始源码、版权和 Apache-2.0 许可，省略测试及不参与运行的模块说明；原生 AgentRuntime、工具注册/执行、上下文和压缩机制直接调用上游实现。宿主适配在 `worker.mjs` 与平台 Go 包中：把模型及只读文档工具交回问答服务，移除产品无关的终端、桌面及账号接入，不修改上游源码。Copyright 2026 Z.AI Co., Ltd；适用许可仍见上述 ZCode 原许可，原始完整声明见 [运行时 NOTICE](docs/licenses/ZCode-runtime-NOTICE.txt)。

实际打包的运行依赖由 pnpm 锁定：diff 9.0.0（BSD-3-Clause）、ipaddr.js 1.9.1（MIT）、marked 16.4.2（MIT）、meriyah 7.4.0（ISC）、TypeScript 5.9.3（Apache-2.0）、unbash 4.0.1（ISC）、yaml 2.9.1（ISC）、zod-to-json-schema 3.25.2（ISC）、zod 3.25.76 与 4.6.5（MIT）。这些包通过 npm 工作区依赖打包，不复制修改源码。构建器使用 esbuild 0.25.12（MIT）。`pnpm build:agent` 按真实打包清单收集各包的完整 LICENSE、NOTICE 和 TypeScript 第三方声明，随 `agent-dist/licenses`、运行文件及镜像一起分发；不以此处的许可名称替代原文。

上游另包含两处派生源码：`core/src/tool/handlers/generated/bash-command-registry.ts` 来自 Fig autocomplete 2.692.3 的生成数据（原始仓库 MIT、npm 元数据 ISC，Copyright (c) 2021 Hercules Labs Inc. (Fig)），保留 [Fig 原始 MIT](docs/licenses/Fig-autocomplete-MIT.txt)；`shared/src/zcode-protocol-v4/wire-codec.ts` 源于 Visual Studio Code IPC/common 工具（MIT，Copyright (c) Microsoft Corporation），保留 [VS Code 原始 MIT](docs/licenses/VSCode-MIT.txt)。原许可文本取自 ZCode 对应源码快照的第三方声明，亦随原生服务分发。
