# 第三方声明

新main的R1-00起开始使用以下第三方工具，均为构建期工具依赖，未复制其源码进仓库；选型与许可经ADR0005确认。

- oapi-codegen v2.5.1 与 oapi-codegen/runtime v1.1.2（Apache-2.0）：make gen从各域openapi.json生成本域DTO与严格服务端接口，`go run`按固定版本调用；生成物重生成即可整体替换。
- openapi-typescript 7.10.1（MIT）：make gen生成浏览器API类型，`pnpm dlx`按固定版本调用。

OpenAPI、JSON Schema、RFC等链接是协议参考，未复制其实现。设计提及Go/Vue等技术不代表已安装依赖；实施前逐项确认版本、许可、版权与复制范围，新增借用项目取得用户确认，再记录到本文件。GPL代码只参考，不复制；Docmost社区版编辑器扩展是唯一例外（AGENTS与ADR0005）。
