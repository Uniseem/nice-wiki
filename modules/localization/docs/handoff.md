# 本轮首组交付（2026-10-18）

独占modules/localization，分支codex/localization-20261018-bulk，基线f1d4ce0628470eda7b253993366669759dd01cdc。源码21c6a196c990d323c73429b769a38895beca7a93，其后另提交此台账。11个既有业务/已修改函数与15个新增参考helper精确IDs见progress.json.readyAwaitingCI，均blocked待统一GitLab CI，未编译/未验收。

已写：规范BCP47及别名冲突、语言权限入口修正；术语增删列表/固定代数解析/人工批准幂等/批次撤销/保护内容review和机翻stale。没有调用模型，不自动重译。003迁移新增软删/活动唯一scope、全站持久代数及批准命令站/根/原结果代数。来源语言未知的旧manifest按target保守失效，保护文本原值不动。

memory批准授权发生点、短期proof过期和撤销重试语义已在workflows明确，实际命令继续核当前本域epoch/维护/expectedGlossaryGeneration/term CAS，终身按proposalId固定命令内容和根站同代。不同内容冲突，成功/拒绝均在本地提交后通过平台outbox发布终态，不反向调memory。

参考级ResolveGlossary/HandleGlossaryChange已登记到其实际glossary实现文件，防止只登记旧路径。新增helper都登记实际签名，不隐瞒函数分母；生成目录未手改，根结构和make docs/gen/check交集成GitLab统一。

本轮仅git diff --check通过；未本地/SSH工具链，未新增或运行业务测试、真实接口/模型、进程、部署或push。其余未改verified保持原证据。

下一步（一项）：读取translation/units/guard相关精确合同，在同域权威译文文件和manifest基础上完成编辑保护、TM、增量与批次上下文/校验链，再及时交付GitLab CI；不得以DB文字索引替代权威正文或绕过人工保护。

下文旧交接不覆盖本轮首组。

---

# 2026-10-17 语言配置交接

- 分支：`codex/localization-20261017-r4`
- 基线：`a080a6c918e31d272c7a91368f59d5ad51d5f458`
- 源码 SHA：`650a1e208ff8656531aa2764dfd9e2bc4b1965b8`
- 完成函数：`localization.PutLanguage`、`localization.SetLanguageEnabled`、`localization.ListLanguages`（implemented，未验证）
- 仍 blocked：`localization.NormalizeLanguageTag`。大小写映射、脚本子标签和私用用法没有可执行规则；函数不自选 BCP 47 方言，不返回规范标签或方向。
- 未跑 CI：未执行 GitLab CI、go、gofmt、make build、make gen、make check、SSH。本域执行了 `make docs`。

## 行为

写入和列表使用 `languages` 私有表。标签按原文保存。第一个连字符前的原文与 `ar`、`he`、`fa`、`ur` 逐字相同才要求 rtl，其余要求 ltr，不折叠大小写。`profileId` 只检查标识声明，不读档位目录，不调用模型，不写默认费用。

代次必须同时等于 site-config 当前代次和本域 `runtime_meta`。水位只在为空时写入；已有不同代次拒绝覆盖。维护阶段，以及 `local_barrier` 不是 released，都会拒绝。`local_barrier` 初始为 released，只是读侧，不是 `AcquireBarrier`，也不在 control 区。

原回执按站点、主体、函数和幂等键保留 7 天。同一规范输入返回原结果；未过期而摘要不同返回 `CONFLICT`。过期视为未命中，成功提交时才替换。回执重放发生在代次和维护检查之后、本地屏障检查之前。

已有其他默认语言时 `PutLanguage` 返回 `CONFLICT`，不在一次写入里转移默认语言。直接 `SetLanguageEnabled(enabled=true)` 遇到已经打开的自动翻译返回 `CONFLICT`，不清除该标志。停用默认语言返回 `CONFLICT`。停用只取消本域该语言当前代次的 queued、running、review 任务，不碰 `translation_units`，不调用 `jobs:cancelJob`。

缺少已核本站，或站点为未签名占位 `system`，返回 `DEPENDENCY_UNAVAILABLE`。本包不跨域调用，未改公共协议。

`ListLanguagesInput` 写在业务包内。列表没有请求体，OpenAPI 生成物只有查询参数。

参考助手在 `internal/language/guard.go`，未逐个登记为函数：`requireActor`、`requireIdempotencyKey`、`acceptStoredTag`、`openLanguage`、`rejectHeldBarrier`、`digestInput`、`readReceipt`、`writeReceipt`、`scanLanguage`、`loadLanguage`、`enqueueLanguageChanged`。`config.go` 另有 `rejectOtherDefault`。`http.go` 的 `NewLanguageHandlers` 绑定三个已有 operation。

`migrations/migrations.go` 源码未改，其旧验证不覆盖 `002_language_config.sql`。

## 合同变更

- `NormalizeLanguageTag`：写明未规范的归一化失败关闭，并登记不可重试的 `DEPENDENCY_UNAVAILABLE`。
- `PutLanguage`：改为原文标签、主子标签方向、档位只声明、CAS、7 天回执和 `localization.language.changed`。依赖改为空。补 `CONFLICT` 与可重试的 `DEPENDENCY_UNAVAILABLE`。
- `SetLanguageEnabled`：停用只改本域任务行。去掉 `jobs:cancelJob` 依赖。补 `VALIDATION` 与可重试的 `DEPENDENCY_UNAVAILABLE`。
- `ListLanguages`：补代次、维护、`local_barrier` 和已核本站。补 `VERSION_MISMATCH` 与可重试的 `DEPENDENCY_UNAVAILABLE`。
- 新增文件登记：`guard.go`、`http.go`、`migrations/002_language_config.sql`。`NormalizeLanguageTag` 仍由 `EstimateLanguagePlan` 依赖，因此设计图仍可达。

## 需要集成处理

- 把新文件登记进根 `docs/structure/framework.json`。
- 翻译执行接入时，若需要同步取消 jobs 任务，把 `jobs:cancelJob` 加回 `SetLanguageEnabled` 并在同一次业务里调用。
- 完整屏障协议、control 区放置、跨代次水位迁移都不在本切片。
- 三个语言操作已绑定，但 `BuildApplication` 仍失败关闭。
- 审核后跑 GitLab CI。不要把本切片记为 verified 或 accepted。

## 下一步（一项）

语言读写已由 GitLab pipeline 42 记 verified。NormalizeLanguageTag 仍 blocked，不要自选大小写、脚本或私用规则。下一步从合同步骤已经写明的其他 planned 函数继续。

下文是历史交接，其中的下一步不再执行。

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

# 本地化会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- 迁移改为 `migrations/migrations.go` go:embed 读取 `001_initial.sql`，删除内联 SQL 副本；执行器支持多编号迁移并校验名称。
- 运行状态适配解开 `state` 外层；补齐 go.sum。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增的迁移包、迁移文件与函数已回填修复提交 `978e0cee659dc342650193c628710ff75115add5` 并按上述检查证据转为 `verified`。
- 遗留：`ui/package.json` 使用 TypeScript `6.0.0-beta`，ADR0005 登记为 6.0.x 正式版；改为正式版需联网重建 lockfile，留待前端工作包。

## 本次事实

2026-10-15 在当前集成分支 `main` 基于 `8ae6fcfc6302e2fd41a998faef18be52e2a9714c` 完成本域私有持久化与启动恢复校验切片，提交为 `cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`；未修改合同、生成物、根文件或其他模块。

已实际完成：

- `cmd/localization/main.go` 只装配 platform 进程生命周期与本域参数。
- `internal/app/wire.go` 建立数据目录、加载身份材料、装配 site-config 运行权威客户端和恢复换目录入口；因业务依赖未实现，按合同返回 `DEPENDENCY_UNAVAILABLE`。
- `internal/runtime/bootstrap.go` 打开 active SQLite 与 control journal，初始化 platform inbox/outbox，按 `001_initial.sql` 执行单调迁移，校验表结构、journal、默认语言唯一性和术语代数，并登记旧翻译租约数量；不在启动阶段重放模型或任务副作用。
- `migrations/001_initial.sql` 建立语言、翻译任务/单元、译文清单、术语、界面目录、语言建议、批准提议和缓存回答译文权威表；执行时使用本域二进制内固定迁移内容，生产镜像不依赖源码路径。
- `internal/transport/server.go` 提供健康/就绪路由、HTTP Serve/Shutdown 骨架；未接入普通业务路由、事件订阅或 worker，Store 尚未由 Application 持有。
- `go.mod`、Makefile、非敏感配置、部署 manifest、Dockerfile、Vue/Vite/TypeScript UI 构建基础和 `pnpm-lock.yaml` 已建立。
- `mountModuleUI` 仅完成本域挂载基础与重复挂载取消；未创建业务页面或 API/SSE 客户端。

## 检查与限制

- `pnpm install --lockfile-only`、`pnpm run typecheck`、`pnpm run build`：通过。
- `python scripts/check_docs.py --privacy-only`：通过。
- `git diff --check`、本域 JSON 解析和暂存内容隐私扫描通过。
- `python scripts/check_docs.py`：本次未运行完整检查；根结构登记属于集成会话范围，不能在本域会话修改。
- `make gen`：未执行成功，本机没有 make；现有 Go/TS 生成文件未手改、未变更。
- 本机无 Go、gofmt、Docker，VPS 不可用；未生成 `go.sum`，未执行 Go build/vet/gofmt、Docker 构建、业务测试、真实接口或模型调用。上一代码提交的 staged diff 检查还提示 Makefile 文件末尾有空行，代码范围未在本次台账修正中改动。

## 基线、范围与状态

- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`。
- 代码与台账提交：`cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`。
- 独占范围：`modules/localization/`；本次实际修改包含 `go.mod`、`internal/app/wire.go`、`internal/runtime/bootstrap.go`、`migrations/001_initial.sql` 和三份本域台账。
- 合同未修改，`contractSha256` 保持 `02c1a90a759bb421b0d59c1585167e2e5c352f242a3926a4f76a0b5a1082a313`。
- `localization.LoadRuntime`、`internal/runtime/bootstrap.go` 与 `migrations/001_initial.sql` 已登记为 implemented，但因本机无 Go/gofmt，验证仍为 `not_run`。
- 完整 OpenAPI handler、翻译/术语/界面业务、事件与任务循环、Store 持有/Shutdown 接线和行为验收仍未完成；progress 中未涉及的条目保持 `planned`。

## 下一步（一项）

先在具备 Go 工具链的 VPS 或集成环境编译、gofmt、vet 本次本域代码，确认 modernc SQLite 依赖和多语句迁移可用；通过后接入完整 handler，并让 `transport.Application` 持有本域 Store，在 Shutdown 中按安全点关闭。不得同时实现真实模型、测试样例或跨域存储。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。


## 第四轮首页跨域只读协议

listLanguages明确includeDisabled默认false，public-site纯服务仅该只读范围。接口/后台仍按台账未完成，不能据页面进程ready推断语言启用。
