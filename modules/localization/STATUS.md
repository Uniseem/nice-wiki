# 集成检查（2026-10-17）

语言读写已合入 main，GitLab pipeline 42 把本轮函数记为 verified。NormalizeLanguageTag 仍 blocked。下面是合入前的模块记录。

# 2026-10-17 语言配置

阶段：实施。分支 `codex/localization-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`，源码提交 `650a1e208ff8656531aa2764dfd9e2bc4b1965b8`。部署 `not_started`。accepted 为 0。GitLab CI 未跑，未执行 go、gofmt、make build、make gen、make check 或 SSH。本轮在本域执行了 `make docs`（只调用 Python 渲染）。

- `localization.PutLanguage`、`localization.SetLanguageEnabled`、`localization.ListLanguages` 记为 implemented，验证 not_run。它们读写本域 `languages` 表，核对运行权威与 `runtime_meta` 代次，拒绝非 released 的 `local_barrier`，写入保留 7 天的原回执。
- `localization.NormalizeLanguageTag` 保持 blocked。合同没有写明大小写、脚本子标签和私用用法，函数只拒绝空标签和超过 35 的标签，其余返回 `DEPENDENCY_UNAVAILABLE`，不返回改写后的标签或方向。
- `BuildApplication` 已绑定上述三个 HTTP 操作，仍因其余处理器、事件订阅和任务循环缺失返回 `DEPENDENCY_UNAVAILABLE`。
- 停用只把本域 `translation_jobs` 的 queued、running、review 标为 cancelled，不调用 `jobs:cancelJob`，不删除人工译文，不调用模型，不写默认费用。
- 新文件尚未写入根 `docs/structure/framework.json`。公共协议未改。

下一步：等集成会话审核合入并跑 GitLab CI。

历史记录见下文，其中的旧下一步不再执行。

---

# 2026-10-17第三轮公共站点协议接续（仅合同）

根公共候选统一签名siteId，清proof后仍可保留已核本站，传输层删除旧proof头；无声明的旧服务控制仍兼容system。EstimateLanguagePlan的未来调用协议已同步，不能用system替代真实本站或自报未签名站点。本域函数仍planned，没有业务源码、调用或完成数提升，公共代码检查由根GitLab统一。

下面第二轮“公共站点传递未齐”是当时历史事实；当前公共来源与检查结果以根本轮交付为准。

---

# 2026-10-17第二轮公共免费估算依赖（仅合同）

根集成会话为已登记的EstimateLanguagePlan补统一model-gateway:estimateTokens免费文本估算依赖，替代域内重复字节估算。按接口限制分批传实际文本，禁止截断；纯localization服务清原用户委托并保留站点/epoch/root/费用类别/更早截止，当前功能权限由本域先核。已批准的翻译任务和费用执行均未接入。

交叉审查发现公共前置缺口：当前平台无portal proof的纯服务actor站点固定为system，尚不能在HTTP接收端保留本站。EstimateLanguagePlan实施前须收口可信站点传递协议；不得发送未签名站点或保留原proof绕过，本轮未改公共协议，也未声称此调用链已通。

本次只更新未来函数合同、相应流程与契约摘要，EstimateLanguagePlan保持planned，未创建业务源码或提升完成数。生成与结构检查由根GitLab统一，未执行真实接口/模型、业务验收或部署；本轮授权仍由根统一双远程推送。

---

# 本地化当前进度

更新：2026-10-15。阶段：R1-15 实施中；本域启动私有持久化与恢复校验已落地，翻译、术语、人工保护、模型调用、任务和事件业务尚未完成、尚未验收、尚未部署。

- 2026-10-15 集成复核修复：迁移改为 `migrations/migrations.go` go:embed 读取 `001_initial.sql`，删除内联 SQL 副本；执行器支持多编号迁移并校验名称。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`；当前分支：`main`；本次提交：`cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`。
- 已完成登记的启动与契约生成准备：`cmd/localization` 入口、`BuildApplication`/HTTP 生命周期骨架、独立 `go.mod`、Makefile、非敏感配置、manifest、Dockerfile，以及 Vue/Vite/TypeScript UI 构建基础和 pnpm 锁文件。
- 本次新增本域私有持久化启动切片：active 区 `localization.db` 使用 SQLite WAL/外键/忙等待，control 区追加 `localization-journal.jsonl`；按固定迁移集建立语言、翻译任务/单元、译文清单、术语、界面字典、语言建议、已批准提议和缓存回答译文表，并初始化平台 inbox/outbox。
- `LoadRuntime` 已接入 `BuildApplication`：迁移版本与表结构校验、journal 损坏/未完成决定隔离、默认语言至多一个、术语代数读取及旧租约登记均在启动阶段执行；任一步失败都关闭资源并返回已登记错误码。业务 handler、事件订阅、outbox 转发和任务循环缺失时仍按合同返回 `DEPENDENCY_UNAVAILABLE`，普通业务路由、翻译和模型调用未接入。
- UI `pnpm run typecheck`、`pnpm run build` 通过；未进行浏览器行为验收。`python scripts/check_docs.py --privacy-only` 通过；完整文档检查未运行，根 `docs/structure/framework.json` 的本次新文件登记由集成会话处理。
- 本机没有 Go、gofmt、make、Docker 或可用 VPS，因此未生成 `go.sum`，未执行 `make gen`、Go 编译/vet/gofmt、Docker 构建或业务测试。生成的 Go/TS 文件未手改、未变更。
- 本次没有新增测试、样例、真实接口调用或模型调用；部署状态仍为 `not_started`。逐文件、逐函数事实见 [progress.json](progress.json)。

下一步：接入完整 OpenAPI handler、持有并在 Shutdown 中关闭本域 Store，再按合同接入事件订阅、outbox 转发和任务循环；业务验收仍需单独授权。

并行工作只改本域，公共资料交集成会话；根文件、其他模块和生成目录由集成会话维护。
