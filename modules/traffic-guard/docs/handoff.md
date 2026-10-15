# 流量风控会话交接

## 持续实施第三轮登录保护（待VPS）

更新：2026-10-15。本轮：持续实施第三轮登录失败保护；源码/合同已准备，状态in_progress（待VPS make build及docs/gen/check），业务验收未执行。

- 基线：`9acad1ab56532521222f592af0d4b3a849792db5`；分支：`codex/traffic-login-protection-r1`。独占本域，未更改根/平台/其他模块。
- 已写BeginLoginAttempt/FinishLoginAttempt、可信PolicyStore注入/打开代次绑定、两个真实HTTP映射和只追加0002迁移。规则见[data](data.md)；并发和终局均短事务，不持锁核验口令。
- login/bootstrap共享滚动15分钟失败窗（账号5/网络20）、固定15分钟锁及并发（账号1/网络8）；30秒预留，过期仅禁首次authenticated，late rejected按Finish受信时间计失败；同终局重放不重复计数或续锁。
- 完整BuildApplication、GetPolicy/UpdatePolicy、控制屏障/恢复/后台与匿名保留清理仍blocked或planned；本组HTTP映射必须由统一ServiceHandler装配，不能裸挂或伪装完整服务。
- 全部生成/格式/编译/vet/检查由根会话统一在VPS执行；本机仅阅读和差异检查。未新增测试、样例、fixture、mock或临时验证程序，未运行真实接口、模型、服务、部署或push。

具体新增实际路径：`internal/admission.go`、`internal/http_admission.go`、`migrations/0002_login_protection.sql`。生成目录未手改，docs/functions.md与files.md等待根统一VPS生成。新迁移已获本轮授权，根framework登记由集成会话处理。源码与台账尚未提交，最终SHA在VPS产物回传后回填。


## 持续实施第二轮公共登录合同（尚未实现）

- 新增beginLoginAttempt/finishLoginAttempt两个identity专用服务API，统一账号/网络失败保护、持久并发保留与幂等终局；阈值/隐私边界见data.md和机器合同。
- 登录网络HMAC只取已认证portal服务断言，identity用自身私钥HMAC规范账号名后调用guard；绝不从浏览器正文/header获取可信网络键。
- 本轮仅合同/生成代码，两个业务函数planned；不宣称实际登录风控已可用。根公共快照VPS编译与结构检查通过，不代表业务行为。


## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- 迁移改为 go:embed 读取 `0001_domain.sql`，删除内联副本；决定日志读取按条目 ID 聚合，open 后跟 done 不再误判未决。
- 运行状态适配解开 `state` 外层；补齐 go.sum，gofmt 整理。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增的迁移包、迁移文件与函数已回填修复提交 `978e0cee659dc342650193c628710ff75115add5` 并按上述检查证据转为 `verified`。

## 本次事实（R1-13 私有策略库启动恢复切片，2026-10-15）

- 实际基线：`588de8fa991340df7dcf019f8c11849f85e1b796`；当前共享工作区分支：`codex/analytics-r1-storage-contract-audit`，分支名与本域不一致；traffic-guard 源码提交：`7aa44567609713b8810b9b8ca9ba61579648d55a`，迁移辅助文件收回提交：`13396bc`；未push、未部署。
- 独占范围：仅 `modules/traffic-guard/`；并行会话期间另有 identity/jobs 未提交或提交变化，本次未暂存、未修改其文件；根 `docs/structure/framework.json` 未越权修改。

## 本次完成

- 新增 `internal/policy_store.go`：打开 `active/traffic-guard.db`，启用 SQLite WAL/foreign_keys/busy_timeout=3000ms，单连接串行；调用公共 `outbox.EnsureSchema`；执行本域只追加迁移；打开 `control/traffic-guard-journal.jsonl` 并校验损坏/未完成条目；提供安全关闭。
- 新增已登记迁移 `migrations/0001_domain.sql`：`guard_policy`、`policy_operation_receipts`（只存 operationId 输入摘要/修订/代次，不存正文）和 `schema_migrations`。`go.mod` 增加 `modernc.org/sqlite v1.59.0`；未手写 `go.sum`。
- `BuildApplication` 在继续 fail-closed 的前提下真实执行策略库打开、迁移和journal恢复；缺少完整 x-handler、identity授权客户端、activation回执与后台循环时仍返回 `DEPENDENCY_UNAVAILABLE`，不以健康路由冒充可用服务。
- `GetPolicy`、`UpdatePolicy` 未猜测实现：保留为 blocked。没有改变 OpenAPI、types、events 或生成目录。

## 契约阻塞审计

- `GetPolicyInput`要求 `siteId`，但 GET `/v1/policies` 没有 query/path 参数；合同未规定是否从可信 `OperationContext.Actor.SiteID` 推导。策略行缺失时的错误也只登记了 `VERSION_MISMATCH`，没有初始化/不存在语义。
- `UpdatePolicy`要求调用 `identity:checkAuthorization`、同事务 policy-changed outbox 和 operationId 同输入幂等；精确函数签名没有客户端/存储注入边界，原数据文档也没有幂等回执表。本次仅在私有迁移中预留摘要表，未假装完成绑定函数。

## 检查与下一步

- 已通过 traffic-guard contracts JSON 解析、traffic-guard 台账/合同覆盖核对、`git diff --check` 和暂存差异检查；本机缺少 Go/gofmt/make，未运行编译、vet、迁移运行验证或 `make check`；未新增测试、样例、真实接口、模型调用或部署。完整 `python scripts/check_docs.py` 当前被并行会话新增文件未登记根 `docs/structure/framework.json` 及其他会话台账未落提交SHA阻塞，traffic-guard 自身合同摘要与台账覆盖已通过。
- 下一步一项：先由契约负责人收口 `GetPolicy` 的站点来源/缺失策略错误、`UpdatePolicy` 的授权与存储注入及 operationId 回执语义，再在本域实现 `GetPolicy`/`UpdatePolicy` 和把 `PolicyStore` 挂入完整 Application；收口前不得猜测 HTTP 或外域调用。

## 本次事实（R1-12 ValidatePolicy 纯规则，2026-10-15）

- 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`。
- 分支：`codex/traffic-guard-r1-12-policy`；在指定独立 worktree 中完成。
- 实现提交：`d597401755cabc77cf31dadf5b9c12260b60b43b`；台账提交在本次交付结束前同步完成；未 push、未部署。
- 独占范围：仅 `modules/traffic-guard/`；根文件、公共 contracts/docs/scripts/libs/deploy、生成目录和其他模块均未修改。

## 本次完成

- 新增 `internal/policy.go`，实现 `traffic-guard.ValidatePolicy` 及其输入/结果类型和本域 `OperationContext`/`GuardPolicy` 别名。
- 纯规则校验整数主体速率 `1..1000`、突发量 `1..100`、站点速率 `1..100000`、主体/站点并发 `1..128`，并要求 `qaBurst≤qaPerSubjectPerMinute`；成功返回相同策略值，失败返回零值与 `VALIDATION`。
- `GuardPolicy` 当前类型合同没有挑战难度或 ban TTL 字段，本次没有增加未登记字段或修改生成物；费用/余额字段不进入策略类型，未知字段由 schema 边界拒绝。
- 不新增测试、样例、快照、E2E、真实接口/模型调用或部署，不产生数据库、日志或其他副作用。

## 检查证据与状态

- `git diff --check` 通过；提交前暂存内容仅含 `modules/traffic-guard/internal/policy.go`，作者为仓库既有 GitHub 代用身份。
- 本机缺少 Go、gofmt、make，未执行 `go build`、`go vet`、格式化或 `make check`；没有将源码标记为 verified。
- 业务验收未执行；`M12-R002`/`M12-R003` 仍未进行行为验收。`progress.json` 将本文件与函数标为 implemented、verification 标为 not_run。

## 下一步（一项）

继续实现本域私有策略持久化前，先由集成会话登记 `docs/structure/framework.json` 中的新文件；随后按依赖顺序处理 `GetPolicy`/`UpdatePolicy` 与 SQLite/journal。不得把本次纯函数实现视为业务准入或恢复验收通过。

## 本次事实（R1-11 启动与契约生成准备，2026-10-15）

- 基线：`ac593c57861d2242f27d07562a2acae535d18c38`。
- 分支：`codex/traffic-guard-r1-11`；独立 worktree：本会话的 traffic-guard worktree。
- 实现提交：`dc3245eacb5630ecad3a76ac82a90692a920f378`；未提交文件：本台账更新待单独提交。
- 独占范围：`modules/traffic-guard/`。根 AGENTS/STATUS/README、根 docs、scripts、contracts、libs、deploy 和其他模块均未修改。
- 已创建：`cmd/traffic-guard/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`Makefile`、`config/defaults.json`、`Dockerfile`、`deploy/manifest.json`。
- 入口只传入本域参数给 `platform.RunService`；基础装配不实现产品算法。缺少完整 x-handler、业务存储/journal、activation 回执及后台函数时，`BuildApplication` 明确返回 `DEPENDENCY_UNAVAILABLE`，不以空 handler 冒充成功。

## 实现、验证与业务验收的区别

- 已实现但未运行验证：基础 HTTP 生命周期代码、配置/目录/信任锚/签名密钥/runtime/恢复换目录装配、默认配置和部署描述。`progress.json` 已把 `Serve`、`Shutdown`、`Health`、`Ready` 及配置文件登记到本次实现提交。
- 已阻塞：`main` 与 `BuildApplication` 仍等待本域业务 handler、私有持久化、journal 恢复、activation 回执和后台循环；`go.sum` 未生成；本次 `make gen` 未能执行，基线生成文件未标为本次已验证。
- 业务验收：未执行。未启动真实服务，未调用 identity/site-config/jobs/data-protection、真实接口或模型，未新增测试/样例/E2E/快照，未部署。

## 检查证据

- 云端同步：在初始工作区执行 `git fetch origin` 和 `git pull --ff-only origin main`，结果为 Already up to date；本会话随后从该基线建立独立 worktree。
- 通过：目标 JSON 配置/manifest 解析；提交前 `git diff --cached --check`；提交作者为仓库既有 GitHub 代用地址，提交内容未发现个人邮箱、密钥、凭证、访客数据或私有路径。
- 通过：`python scripts/check_docs.py --privacy-only`。
- 受公共文件所有权阻塞：完整 `python scripts/check_docs.py` 仅报告本次 8 个新文件尚未登记在根 `docs/structure/framework.json`；该根文件属于集成会话，本会话未越权修改，合并时需由集成会话补登记后重跑。
- 未通过/未执行：本机 `gofmt`、`make gen` 均因命令不存在无法执行；本机 Go 构建、vet、go.sum 生成、Docker 构建和 UI 生成均未执行。未读取私有 VPS 连接信息，未在 VPS 运行检查。

## 遗留阻塞与明确决策

- `go.sum` 只能由可用 Go 工具链生成，不能手写占位。
- `internal/gen/openapi.gen.go`、`clients.gen.go`、`oapi-codegen.yaml` 与 `web/src/gen/api.gen.ts` 只能由根 `make gen` 更新；本次未运行生成入口，不宣称生成验证通过。
- `BuildApplication` 必须继续 fail-closed；在完整 handler、持久化和后台循环接入前不能返回可服务 Application。
- 未新增第三方库；`go.mod` 只声明当前生成代码与平台代码所需的固定版本，许可证登记未扩展。

## 下一步（一项）

实现 `traffic-guard.ValidatePolicy`（`internal/policy.go`，参考级）：先读本域 `docs/functions.md` 对应合同、`docs/data.md` 与 M12-R002/M12-R003，按范围/无费用字段/整型速率约束完成纯规则校验，不新增测试或接口；完成后同步本域台账并在可用 Go 工具链上运行既有检查，再继续处理私有持久化以解除启动阻塞。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
