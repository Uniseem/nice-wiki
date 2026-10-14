# 第三方声明

新main的R1起开始使用以下第三方依赖，选型与许可经ADR0005确认；未复制其源码进仓库。

- oapi-codegen v2.5.1 与 oapi-codegen/runtime v1.1.2（Apache-2.0）：make gen从各域openapi.json生成本域DTO与严格服务端接口，`go run`按固定版本调用；生成物重生成即可整体替换。
- openapi-typescript 7.10.1（MIT）：make gen生成浏览器API类型，`pnpm dlx`按固定版本调用。
- kin-openapi v0.133.0（Apache-2.0，匹配oapi-codegen v2.5.1依赖图）：生成代码的嵌入spec加载与请求/响应校验；自R1-01起随各域go.mod作为库依赖引入，间接依赖以其go.mod为准。
- modernc.org/sqlite v1.59.0（BSD-3，ADR0005已批准的纯Go SQLite驱动）：identity私有业务库存储引擎；自R1-01私有持久化工作包起引入，间接依赖以其go.mod为准。
- golang.org/x/crypto v0.28.0（BSD-3-Clause）：identity口令规则使用Argon2id；仅通过模块依赖引用，不复制源码。
- golang.org/x/text v0.20.0（BSD-3-Clause）：identity账号名NFC规范化；仅通过模块依赖引用，不复制源码。

OpenAPI、JSON Schema、RFC等链接是协议参考，未复制其实现。设计提及Go/Vue等技术不代表已安装依赖；实施前逐项确认版本、许可、版权与复制范围，新增借用项目取得用户确认，再记录到本文件。GPL代码只参考，不复制；Docmost社区版编辑器扩展是唯一例外（AGENTS与ADR0005）。
