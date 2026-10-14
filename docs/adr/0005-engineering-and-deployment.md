# ADR0005：工程选型、平台公共库与单机部署

- 日期：2026-10-14。
- 状态：用户授权由 Claude 按建议决定；第三方代码借用与新依赖仍按 AGENTS 在实施时逐项核对许可后写入 THIRD_PARTY_NOTICES.md。
- 来源：old D2、D5、D18、D23、D51 的已确认选型；本轮接线后 15 个服务共有的基础设施。

## 选择

### 语言与依赖（沿用 old 已确认选型）

| 范围 | 选择 | 许可 | 来源 |
|---|---|---|---|
| Go 工具链 | 1.25.x，每服务独立 go.mod（`nice-wiki/modules/<slug>`） | BSD-3 | old go.mod |
| HTTP | 标准库 `net/http` 路由，不引入框架 | BSD-3 | D18 |
| 存储 | 每服务私有 SQLite（WAL、foreign_keys、busy_timeout 3000ms），驱动 `modernc.org/sqlite`（纯 Go）；查询用 sqlc 生成；迁移为各域 `migrations/` 只追加 SQL | BSD-3 / MIT | D5、D18 |
| 接口 | oapi-codegen 生成服务端严格接口、DTO 与调用客户端；kin-openapi 校验请求与响应 | Apache-2.0 / MIT | old go.mod |
| 版本镜像 | go-git v5 | Apache-2.0 | 已有设计 |
| 代码高亮 | chroma v2 | MIT | 已有设计 |
| 语言与地区 | `golang.org/x/text/language`；本地 GeoIP 用 maxminddb-golang | BSD-3 / ISC | D51 |
| 树排序键 | fracdex（Go 与 JS 版，CC0，复制进项目并注明出处） | CC0 | D51 |
| 文件锁 | gofrs/flock | BSD-3 | old go.mod |
| 签名 | 标准库 `crypto/ed25519` 实现 JWS compact，不引入 JWT 库 | BSD-3 | 本 ADR |
| 前端 | Vue 3.5、vue-router 4、Pinia 3、TypeScript 6.0.x、pnpm、Vite、shadcn-vue + Tailwind CSS 4、openapi-typescript + openapi-fetch | MIT / Apache-2.0 | D18 |
| 编辑器 | Tiptap 3.31.x 官方扩展；KaTeX；Excalidraw（React 懒加载，仅后台） | MIT | D4、D9 |
| 图片压缩 | browser-image-compression | MIT | D51 |
| 反向代理 | Caddy 2 | Apache-2.0 | D23 |

版本以 old 锁定版本为起点，实施时取同一主次版本的最新补丁写入各域 go.mod/package.json 与锁文件，不用浮动版本。PDF 文本提取库在 knowledge 实施时选型，只能选宽松许可的纯 Go 库，选定前先核对许可。

old D51 的“后台任务队列直接使用 Backlite”由 jobs 服务取代，不再使用。

### 借用代码

- Docmost 社区版编辑器扩展：沿用 D51，按用户当时的确认直接借用（Docmost 与本仓库同为 AGPL-3.0，许可兼容）；文件头注明出处与原版权，登记 THIRD_PARTY_NOTICES.md，只借社区版文件。这是 AGENTS“GPL 项目只参考不复制”的已确认例外，待用户在 R0-05 再次确认。
- Halo（GPL-3.0）、New API、Gitea、PageIndex 只参考思路，不复制代码。

### 平台公共库

新增 `libs/platform`（Go module `nice-wiki/libs/platform`），只放与业务无关的机制：服务断言与用户委托、可信操作上下文、公共 Problem 输出、outbox 登记与转发、inbox 去重与事件订阅、任务领取循环、模型调用助手、配置加载、运行状态读取与监视、恢复换目录、进程模板，以及生成服务密钥的 `nw-keys`。各服务的 RelayOutbox、ConsumeEvents、RunWorker、InvokeModel、main、BuildApplication、Serve 等约束函数只装配本域参数并调用本库。各服务以相对 replace 引用本库，仍可独立构建镜像。本库改动视为公共契约变更，由集成会话统一修改。

### 生成代码

每个服务登记 `internal/gen/openapi.gen.go`（本域 DTO 与严格服务端接口）、`internal/gen/clients.gen.go`（本域实际调用的生产方客户端，自动附服务断言）、`internal/gen/oapi-codegen.yaml`；有界面的服务另有 `<web|ui>/src/gen/api.gen.ts`。jobs 的 `config/job-types.json`、`config/event-subscriptions.json` 与 portal 的 `config/routes.json` 由全部合同生成。生成器在 R1-00 实现为 `make gen`，生成目录不手改。

### 单机部署

`deploy/` 是部署的唯一配置来源：Docker Compose 编排 15 个服务与 Caddy，`restart: unless-stopped`（恢复切换依赖退出码 75 后自动拉起）。Caddy 终止 HTTPS，`/api/*`、`/admin*`、`/ui/*` 到 portal，其余到 public-site；其他服务端口只在内部网络。每服务独占数据卷 `/data/<slug>`（active、control、restore），备份卷 `/backups` 只挂 data-protection。私钥以文件 secret 挂载，公钥表由 nw-keys 生成。

| 服务 | 端口 | GOMEMLIMIT | 服务 | 端口 | GOMEMLIMIT |
|---|---|---|---|---|---|
| portal | 8080 | 128MiB | knowledge | 8106 | 384MiB |
| public-site | 8081 | 192MiB | qa | 8107 | 192MiB |
| identity | 8101 | 96MiB | localization | 8108 | 192MiB |
| site-config | 8102 | 96MiB | memory | 8109 | 96MiB |
| content | 8103 | 192MiB | traffic-guard | 8110 | 96MiB |
| media | 8104 | 192MiB | jobs | 8111 | 128MiB |
| model-gateway | 8105 | 160MiB | analytics | 8112 | 128MiB |
| data-protection | 8113 | 192MiB | Caddy | 80/443 | 64MiB |

合计约 2.6GiB，4GB 主机留约 1GiB 给系统与页面缓存；内存紧张时先降低 knowledge 与 public-site 的缓存。

## 代价与影响

- 平台库成为所有服务的共同依赖；它的缺陷影响全部服务，因此只放机制、由集成会话统一维护。
- 15 个容器的运维成本高于单体；部署、升级与日志集中由 deploy/ 管理。
- 依赖版本在实施时才最终锁定；任何新增依赖都要先核许可。
