# 第三方声明

新main的R1起开始使用以下第三方依赖，选型与许可经ADR0005确认；未复制其源码进仓库。

- oapi-codegen v2.5.1 与 oapi-codegen/runtime v1.1.2（Apache-2.0）：make gen从各域openapi.json生成本域DTO与严格服务端接口，`go run`按固定版本调用；生成物重生成即可整体替换。
- openapi-typescript 7.10.1（MIT）：make gen生成浏览器API类型，`pnpm dlx`按固定版本调用。
- kin-openapi v0.133.0（Apache-2.0，匹配oapi-codegen v2.5.1依赖图）：生成代码的嵌入spec加载与请求/响应校验；自R1-01起随各域go.mod作为库依赖引入，libs/platform的apivalidate用其openapi3filter做入口请求校验；间接依赖以其go.mod为准（gorilla/mux等仅为其测试依赖，只记go.sum校验和，不编入程序）。
- modernc.org/sqlite v1.59.0（BSD-3，ADR0005已批准的纯Go SQLite驱动）：各域私有业务库存储引擎（identity、site-config、jobs、media、traffic-guard、localization、memory、model-gateway）；自R1-01私有持久化工作包起引入，间接依赖以其go.mod为准。
- golang.org/x/crypto v0.55.0（BSD-3-Clause）：identity口令规则使用Argon2id；仅通过模块依赖引用，不复制源码。
- golang.org/x/text v0.41.0（BSD-3-Clause，ADR0005已批准）：identity账号名NFC规范化、knowledge与qa文本NFKC规范化、qa语言标签BCP 47解析；仅通过模块依赖引用，不复制源码。

- localization 界面工程（ADR0005前端选型，MIT/Apache-2.0）：vue 3.5.22、vue-router 4.5.1、pinia 3.0.3、vite 7.1.7、@vitejs/plugin-vue 6.0.1、typescript 6.0.0-beta；版本以 `modules/localization/ui/pnpm-lock.yaml` 为准（integrity 已与 npm registry 核对），不复制源码。

OpenAPI、JSON Schema、RFC等链接是协议参考，未复制其实现。设计提及Go/Vue等技术不代表已安装依赖；实施前逐项确认版本、许可、版权与复制范围，新增借用项目取得用户确认，再记录到本文件。GPL代码只参考，不复制；Docmost社区版编辑器扩展是唯一例外（AGENTS与ADR0005）。

- golang.org/x/net v0.58.0 / idna（BSD-3-Clause，Copyright 2009 The Go Authors）：平台公开地址IDNA Lookup规范化，仅模块引用、不复制代码。实施时读取官方Go模块原LICENSE核对；该版本为当前Go1.25可兼容的最高发布版，较新版本要求更高工具链。传递依赖由VPS go命令锁定，不手写go.sum。
