# 第三方声明

新main的R1起开始使用以下第三方依赖，选型与许可经ADR0005确认；未复制其源码进仓库。

- oapi-codegen v2.5.1 与 oapi-codegen/runtime v1.1.2（Apache-2.0）：make gen从各域openapi.json生成本域DTO与严格服务端接口，`go run`按固定版本调用；生成物重生成即可整体替换。
- openapi-typescript 7.10.1（MIT）：make gen生成浏览器API类型，`pnpm dlx`按固定版本调用。
- kin-openapi v0.133.0（Apache-2.0，匹配oapi-codegen v2.5.1依赖图）：生成代码的嵌入spec加载与请求/响应校验；自R1-01起随各域go.mod作为库依赖引入，libs/platform的apivalidate用其openapi3filter做入口请求校验；间接依赖以其go.mod为准（gorilla/mux等仅为其测试依赖，只记go.sum校验和，不编入程序）。
- modernc.org/sqlite v1.59.0（BSD-3，ADR0005已批准的纯Go SQLite驱动）：各域私有业务库存储引擎（identity、site-config、jobs、content、media、traffic-guard、localization、memory、model-gateway、knowledge、qa）；自R1-01私有持久化工作包起引入，间接依赖以其go.mod为准。
- golang.org/x/crypto v0.55.0（BSD-3-Clause）：identity口令规则使用Argon2id；仅通过模块依赖引用，不复制源码。
- golang.org/x/text v0.41.0（BSD-3-Clause，ADR0005已批准）：identity账号名NFC规范化、knowledge与qa文本NFKC规范化、qa语言标签BCP 47解析；仅通过模块依赖引用，不复制源码。

- localization 界面工程（ADR0005前端选型，MIT/Apache-2.0）：vue 3.5.22、vue-router 4.5.1、pinia 3.0.3、vite 7.1.7、@vitejs/plugin-vue 6.0.1、typescript 6.0.0-beta；版本以 `modules/localization/ui/pnpm-lock.yaml` 为准（integrity 已与 npm registry 核对），不复制源码。

- 后台界面工程（ADR0005前端选型，MIT）：modules/portal/web 与 modules/content/web、modules/jobs/web 使用 vue 3.5.22、vite 7.1.7、@vitejs/plugin-vue 6.0.1、typescript 6.0.0-beta；文档编辑器使用 Tiptap 3.32.0（@tiptap/core、vue-3、starter-kit、extension-task-list、extension-task-item、extension-highlight、pm）；版本以各目录 pnpm-lock.yaml 为准，仅依赖不复制源码。

OpenAPI、JSON Schema、RFC等链接是协议参考，未复制其实现。设计提及Go/Vue等技术不代表已安装依赖；实施前逐项确认版本、许可、版权与复制范围，新增借用项目取得用户确认，再记录到本文件。GPL代码只参考，不复制；Docmost社区版编辑器扩展是唯一例外（AGENTS与ADR0005）。

- golang.org/x/net v0.58.0 / idna（BSD-3-Clause，Copyright 2009 The Go Authors）：平台公开地址IDNA Lookup规范化，仅模块引用、不复制代码。实施时读取官方Go模块原LICENSE核对；该版本为当前Go1.25可兼容的最高发布版，较新版本要求更高工具链。传递依赖由VPS go命令锁定，不手写go.sum。

- github.com/alecthomas/chroma/v2 v2.20.0（MIT，ADR0005已选）：公开代码高亮，来源 https://github.com/alecthomas/chroma/tree/v2.20.0 ，仅依赖不复制业务代码。
- katex 0.16.22（MIT，Copyright (c) 2013-2020 Khan Academy and other contributors）：公开页公式渲染，来源 https://github.com/KaTeX/KaTeX/tree/v0.16.22 ，随 portal 静态文件分发其脚本、样式、字体，不改动源码。
- vue-tsc 3.1.1（MIT，Vue官方language-tools）：前端类型检查，来源 https://github.com/vuejs/language-tools ，仅开发构建依赖。

- github.com/tiktoken-go/tokenizer v0.7.0（MIT，Copyright (c) 2023 tiktoken-go）：经2026-10-18用户明确批准，用于模型网关显式绑定编码的离线BPE；固定来源 https://github.com/tiktoken-go/tokenizer/tree/v0.7.0 ，原许可 https://github.com/tiktoken-go/tokenizer/blob/v0.7.0/LICENSE 。仅Go模块依赖，不复制实现；发布程序保留依赖原LICENSE。
- github.com/ledongthuc/pdf v0.0.0-20260907135840-6c8c28e0e8a0（BSD-3-Clause，Copyright 2009 The Go Authors）：同次用户批准的PDF项目，实际固定版本为BSD三条款，非最初提问误称的MIT。来源 https://github.com/ledongthuc/pdf/tree/6c8c28e0e8a0 ，原许可 https://github.com/ledongthuc/pdf/blob/6c8c28e0e8a0/LICENSE 。用于有界隔离提取，仅模块依赖；保留原版权、许可与免责声明，不复制业务实现。

本仓保留新批准依赖的固定原许可全文：[tokenizer MIT](docs/licenses/tiktoken-go-tokenizer-v0.7.0.LICENSE)、[PDF BSD三条款](docs/licenses/ledongthuc-pdf-6c8c28e0e8a0.LICENSE)。随二进制分发本声明和这些原许可，不以链接替代应保留文本。

- github.com/oschwald/maxminddb-golang v1.13.1（ISC，ADR0005/D51已确认）：analytics只读取可信部署提供的本地MaxMind数据库，不下载数据库、不发送原始地址；固定来源 https://github.com/oschwald/maxminddb-golang/tree/v1.13.1 ，许可原文 https://github.com/oschwald/maxminddb-golang/blob/v1.13.1/LICENSE 。仅Go模块依赖，不复制源码；数据库内容不属于该库许可，仍由部署方自行提供合法数据。go.sum由GitLab Go工具生成。

## models.dev 公开模型目录数据

用户于2026-10-18明确批准仅引入MIT公开目录JSON，未复制业务代码。来源[models.dev API](https://models.dev/api.json)，固定内容版本`sha256:80a19a3c78de1a7b0eb0a72e13e44df21180f0681d6cdf1dbd4c4c345fae3fb6`，原始5806411字节，取回时间2026-10-18T07:44:00.690256Z。API数据版本以实际内容hash固定，不声称某开发分支commit生成该数据。版权Copyright (c) 2025 models.dev，许可MIT，原文见[登记许可](docs/licenses/models-dev-MIT.txt)及内置目录LICENSE，source.json保留来源与ETag。

## go-git v5.16.3

ADR0005已批准的版本镜像依赖，固定github.com/go-git/go-git/v5 v5.16.3，Apache-2.0；Copyright 2018 Sourced Technologies, S.L.。原许可见[固定版本许可](docs/licenses/go-git-v5.16.3-Apache-2.0.txt)，源为[官方tag](https://github.com/go-git/go-git/tree/v5.16.3)。数据保护独立WIP拟通过依赖API使用，不复制业务实现；本次main尚未引入该依赖，保留固定版本许可登记，未执行远程业务镜像同步。

## Excalidraw 与 React（后台编辑器画板）

后台内容编辑器的画板块使用 @excalidraw/excalidraw 0.18.1（MIT，Copyright (c) 2020 Excalidraw）及其对等依赖 react / react-dom 18.3.1（MIT，Copyright (c) Meta Platforms, Inc. and affiliates）。来源 https://github.com/excalidraw/excalidraw/tree/v0.18.1 、https://github.com/facebook/react/tree/v18.3.1 ；仅作为 npm 依赖打包进 `modules/content/web`，由 pnpm-lock.yaml 固定，不复制其源码；画板代码只在用户打开画板时懒加载。
