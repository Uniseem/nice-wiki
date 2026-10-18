审计V2源码 2d251cf79da8e0046b49c4bfff63bd395cb47d89：3既有函数+6支持，累计8既有+20支持待统一GitLab CI；另ConsumeAuditEvent等根订阅装配不计ready。旧SUCCESS输入归一新OK，V1原hash不动；新V2含site/producer/原主体/原时间/原操作/源epoch。新migration0002连续V2，仅实际授权事实，无用户proof续用。

审计V2小批：固定producer历史事实与实时兼容分支共享追加链，3个既有函数和6支持就绪待统一GitLab CI，累计8既有+20支持。ConsumeAuditEvent本体写齐，根统一订阅登记/循环后才计完整入口ready。旧V1 hash只读保留，新V2覆全来源，metadata严格白名单；修readSiteIdentity非法free costClass为空。未本地编译/生成/测试或真实调用。

首批源码 d78c1ee71968f21cea2eb674c0190de58506e4a6：精确5既有函数+14实际支持见progress.readyAwaitingCI，均待统一GitLab CI。依赖root纯服务readSiteConfig/公共loader/ReportingTimeZone字段与统一gen；go.sum只CI更新。未改2既有verified纯函数。

2026-10-18 R1收尾首批：独占codex/finish-20261018-analytics，基线7f75d58。已写真实SQLite/site-config身份读取/严格HTTP边界、完整小时/日报读取、审计签名分页与链核对；5既有函数+14实际支持代码就绪待统一GitLab CI。CSV有代码但导出入口未齐不计ready。保留原2个verified纯函数未改；完整beacon/审计写入/费用聚合/后台/快照/UI仍继续，不伪造Ready。未本地编译/生成/测试或真实业务操作，未push。

# 统计与审计会话交接

## 集成核对（2026-10-17，pipeline 53）

`83a435f322da98035cbe71319aee2ea8914a4534` 已由 GitLab pipeline 53 编译并双推。`BucketLatency` 与 `CheckPrivacyFields` 记 verified。`RedactAuditMetadata` 仍 blocked。下文写作时的“未跑 CI”只描述当时的工作树。

下一步：持续编写已按用户要求停止。动作白名单写明前不要实现 `RedactAuditMetadata`，也不要自动实现下一函数。

## 纯函数切片（2026-10-17）

### 范围与事实

- 分支 `codex/analytics-20261017-r6`，基线 `9a10eff89c91095670f7d336e84e7a010348adc6`。只改 `modules/analytics/`。未 push。
- 源码提交 `11014011ce1a008d4a829dfeb388f9328fcfc1c0`。progress.json 的 implementationCommit 与 deliveryCommit 写这个源码 SHA。合同未改。
- 未改根 STATUS、plan、framework.json、`.gitlab-ci.yml`、libs/platform 或其他模块。未新增测试、fixture、mock 或样例。
- 未执行 go build、go vet、gofmt、make build、make gen，未 SSH。`python3 scripts/render_docs.py --check` 无漂移。

### 函数

- `analytics.BucketLatency`（`modules/analytics/internal/buckets.go`）：implemented，verification not_run。桶标签只有 `0..99`、`100..499`、`500..999`、`1000..2999`、`3000+`。0 到 60000 含端点按闭区间分桶，60000 为 `3000+`。小于 0 或大于 60000 返回 VALIDATION。不读取 ctx 或 op。
- `analytics.CheckPrivacyFields`（`modules/analytics/internal/privacy.go`）：implemented，verification not_run。允许名只有 requestId 与 errorCode。空列表 allowed 为 true 且 forbidden 为空。错误表把禁止名定为 FORBIDDEN。出现 ip、peer、question、prompt、apiKey、token、referrer、query（区分大小写）时返回该错误和零值。其余非允许名进入 forbidden，只含字段名，allowed 为 false，不返回错误。
- `analytics.RedactAuditMetadata`：blocked。审计动作白名单与 metadata 编码都没有写明。对任何输入返回 FORBIDDEN，原文不进入 safeMetadata 或错误 detail。

证据：未跑 GitLab CI。

### 登记

`modules/analytics/internal/buckets.go` 与 `modules/analytics/internal/privacy.go` 已写入根 `docs/structure/framework.json`。GitLab 编译还没有覆盖这次提交，不要把这两个已写函数记为 verified。

### 下一步（一项）

由合同所有者先写明审计动作登记表的动作名枚举，以及 `analytics.RedactAuditMetadata` 的 metadata 编码。写明前该函数保持 blocked，不要猜测 JSON 或编造动作名。

## 历史：集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- site-config 运行状态适配改为解开 `{"state": RuntimeState}` 外层，此前字段全空会被平台按“运行状态不完整”拒绝启动。
- 补齐 go.mod 间接依赖与 go.sum，gofmt 整理 `internal/transport/server.go`。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。

## 本次事实

2026-10-15 本次工作包在分支 `codex/analytics-r1-storage-contract-audit`、基线
`3db911390e35a07007d70a1d2f5cc15fe4092667` 上复核了交接指定的
`migrations/0001_domain.sql` 与 `analytics.BuildApplication` 依赖。未实现业务代码；仅更新本域台账并记录阻塞。

2026-10-15 已完成本域 R1 启动与契约生成准备。基线为
`ac593c57861d2242f27d07562a2acae535d18c38`，独立分支为
`codex/analytics-r1-bootstrap`，代码提交为
`7d97e7e0e25b260d0d8edcd58802fe20613fc42a`，未 push。

本次只修改 `modules/analytics/`：

- `cmd/analytics/main.go`：只装配 `platform/lifecycle.RunService`、本域 `BuildApplication` 和运行权威客户端。
- `internal/app/wire.go`：建立本域 `active/control/restore` 目录，加载信任锚与签名引用，接入 site-config 运行权威和 `ApplyRestoreSwap`，构造 HTTP 应用。
- `internal/transport/server.go`：实现 `Serve`、`Shutdown`、`Health`、`Ready`，并完整绑定生成 server interface 的 17 个路由方法。
- `go.mod`、`go.sum`、`Makefile`、`config/defaults.json`、`deploy/manifest.json`、`Dockerfile`：固定依赖、非敏感默认值和独立部署入口。

业务路由没有伪造成功：匿名 beacon、审计、访问/费用汇总、导出、屏障和快照方法当前统一返回
`DEPENDENCY_UNAVAILABLE`。生成目录中的 `openapi.gen.go`、`clients.gen.go`、`oapi-codegen.yaml` 和浏览器类型未手改。

## 台账状态

`progress.json` 已切换为 `implementation`，记录本次分支、基线和提交。入口、生命周期、健康/就绪、配置与部署文件为 `implemented`；`BuildApplication` 文件与函数为 `blocked`，因为本域 SQLite/journal、恢复激活回执、后台任务/事件循环和真实 x-handler 业务函数尚未实现。其余业务文件/函数保持 `planned`。实现、验证、行为验收和部署状态严格区分。

## 检查

- `python scripts/check_docs.py --privacy-only`：通过。
- 配置与 manifest JSON 解析：通过。
- 生成 server interface 的 17 个方法静态覆盖核对：通过。
- `git diff --check`：通过。
- `make gen`：未执行成功，当前环境没有 `make`；因此未触碰生成目录。
- Go、gofmt、make 均不在当前环境，未运行编译、vet、格式化、业务测试、真实接口、真实模型或部署验收。

根 `framework.json`、根 STATUS/plan、公共合同和其他模块属于集成会话所有权，本次未修改；集成时需登记新增 9 个本域文件并在可用 Go/VPS 环境复跑阶段检查。

## 本次合同阻塞

交接要求的“SQLite/迁移/journal”不是当前机器合同中的可实施切片：

- `implementation.json` 只有迁移文件的概括性 purpose，没有登记迁移执行器、SQLite 打开/配置、control journal 恢复、fsync/完整性收敛的文件、函数、精确签名、错误、副作用、并发和调用边界。
- `analytics.BuildApplication` 只将“本域journal恢复函数”列为 resource，未给出可实现的函数合同。
- `types.json` 只有供 `Application` 使用的 `*sql.DB` 和状态布尔字段，没有上述内部存储/journal接口或恢复记录类型。

因此本次没有创建 `migrations/0001_domain.sql`，没有新增 `internal/storage.go` 或 `internal/journal.go`，也没有接线 `BuildApplication`；否则会把未定义行为藏入实现，违反本域 AGENTS.md 的“缺约束先修契约”要求。需要公共/本域合同所有者先登记该切片后，才能继续编码。

## 历史：存储合同阻塞（2026-10-15）

当时的阻塞：`Ready` 会在本域 journal 未恢复时返回 `DEPENDENCY_UNAVAILABLE`；`BuildApplication` 在恢复发生换目录时也拒绝开放入口，避免把未实现的存储或回执当成成功。该次提交只有台账记录，没有业务验收结论。

当时的下一项：由合同所有者先登记 SQLite/journal 切片（目标文件、约束级函数/签名、迁移版本、journal状态机、错误与恢复边界）；登记完成后再按 `data.md`、ADR0003 和该合同实现 `migrations/0001_domain.sql`、私有存储/journal，并回到 `analytics.BuildApplication` 接入。完成条件是失败可恢复、当前 dataEpoch 校验、所有 OpenAPI x-handler 不再依赖失败关闭，并在 Go/VPS 环境通过允许检查。当前下一步以上文 2026-10-17 一节为准。

匿名访问第三批源码 `fb17c9b5990457cc903d28e86edbf638672de683`：新增6个原函数 ValidateBeacon/HashDayToken/ClassifyCrawler/LookupCountry/IngestBeacon/RecordCrawlerAccess，新增7个实际参考支持函数；累计14原函数+27支持待统一GitLab CI。已登记portal私有签名入口，原IP/UA仅内存、离线mmdb固定v1.13.1、日界线持久固定、来源仅域名、独立爬虫幂等计数。FlushBeacons本体与期水位已写但后台/屏障drain未接，不计完整ready；启动整体和快照继续blocked。go.sum和全部生成/格式/编译只统一GitLab，未本机执行。
本批新增文件 beacon_privacy.go、beacon_store.go、beacon_validation.go、crawler.go、crawler_store.go、geoip.go、http_beacons.go。根须登记新beacon_privacy.go目录。私有ingestTrustedBeacon正文IngestBeaconInput，portal纯服务，4096字节；peerAddress规范IP、userAgent≤1024，公共浏览器正文仍2048。后续接共享Manager安全写门、每5秒flush与最终drain，不得safe之后继续写active；聚合执行镜像只保存真实jobs租约/fence，不另建队列。

小时/日报第四批源码 `4b1ee516ddc0cfe9cc92ab89251c59f09e33f942`：新增3原函数 FlushBeacons/AggregateHourly/AggregateDaily，新增16实际参考支持；累计17原+43支持待统一GitLab CI，原verified2保持。Serve真实周期启动、Shutdown停worker后排空队列；计划固定真实period来源水位、输入字节ObjectRef/原操作/原jobs回执；锁外CheckLease和事务内精确租约镜像CAS；日报总UV与维度UV分开distinct。RunWorker仅hour/day完整，费用两个执行器仍明确不可用，整体/Ready/恢复不计ready。未本机编译/生成/运行接口或测试。
依赖公共新API worker.RunWorkerLoopWithResult（真实jobs最终完成/失败/取消确认计数）及WorkerConfig.OnCancellationObserved（根926fd07）。原输入定义AggregateJobBody已机器同步；新增aggregate_jobs.go/background.go/worker.go及0003迁移需framework登记。公共Manager安全cut仍未装，屏障HTTP继续503且Ready明确blocked；恢复Migrate必须清旧执行镜像/旧任务授权并保原不可变事实，不得复活旧fence。各域snapshot实际inventory与跨域refs仍不可猜。

问答第五批源码 `e7368ddeb2b53b7b9e0aa0f9a4e0c098e4138df8`：新增3原函数 ProjectQaTurn/QueryContentGaps/PublishAggregateEvents、2实际支持applyQaCounts/server.QueryContentGaps；累计20原+45支持待统一GitLab CI。QA完整来源修订/inbox同事务，迟到高修订撤旧分类后新计；缺口查询完整可信日历日、额外qa.manage、真实分子分母与双水位，原文不进入analytics。实际QA订阅与outbox周期已接。ConsumeEvents仍有MG精确安全页/审计producer registry缺口，整体不计ready；不确认未知费用事件、不填0。未本机编译/生成/测试。
新增consumer.go及0004_qa_projection.sql需framework登记；aliases引用MG a3c7536新EventInvocationReconciledPayload，根须先集成该协议。QA V4旧非空无来源汇总拒绝静默迁移。MG QueryUsage仍root必填≤256且无全站固定watermark/cursor，LedgerEntry缺ledgerRevision/operation/class/provider/model等安全统计字段；已给根明确补端口建议，本域不得以event.revision猜ledgerRevision。

导出/保留期第六批源码 `bfaca7b7b881a26f7fd758f7d728ee021173f322`：新增6原函数 QueryUsageStatistics/CreateExport/ValidateCSVCell/PurgeBeacons/PurgeAuditHistory/PurgeExports、1新增绑定OpenExport、18实际参考支持；累计26原+63支持+1新增绑定待统一GitLab CI。固定权限双路由CSV封存/完整核hash下载、过期幂等墓碑、启动journal恢复；真实30天明细/旧日盐/180天审计连续前缀清理已周期可达。费用查询仅已有投影且明确原水位/最后对账，未知不造0；完整MG全站投影与公共快照/Ready仍blocked。未本机编译/生成/运行接口或测试。
新路径usage_queries.go/exports.go/export_recovery.go/0005_export_journal.sql需framework登记；原计划export.go未落地，已按实际替换为exports.go，PurgeExports实际迁至export_recovery.go，QueryUsageStatistics实际迁至usage_queries.go。静态allowlist CSV直接列安全字段，不调用只允许requestId/errorCode的日志字段检查器，也不改变该既有verified函数。恢复Migrate须使旧导出授权/任务镜像失效，本批不伪造公共恢复完成。

界面封口源码 `96a9091` → 过期明确重建窄修 `1b70c56` → 公共周期复用 `bdddc35`：新增4原函数 LoadStatistics/LoadAuditPage/DownloadAuthorizedExport/mountModuleUI，20实际参考支持（含2后台装配支持）；累计30原业务+83支持+1新增HTTP绑定待统一GitLab CI，既有verified2未变，当前136登记函数。不是行为验收/已部署/全域完成。独立Vue库/固定版本、同源端口、权限失败清空、统一卸载、稳定原操作导出、32MiB有界流及真实链校验已写；审计UTC日期进入SQL和游标绑定。周期回调复用lifecycle.RunRecoveryLoop，worker/consumer真实返回和失败监督，不复制ticker机制。未本机编译/生成/测试，lock仅CI。

当前冻结前阻塞：
- MG安全统计全站固定水位页尚缺真实ledgerRevision/owner/site/sourceEpoch/root/op/class/provider/model/amountKnown+amount/state/time，FetchUsagePage/ProjectUsage/AggregateUsage/ReconcileUsage/ConsumeAnalyticsEvent以及完整RunWorker/ConsumeEvents不计ready。旧QueryUsage根查询不能替代。
- 审计producer事件registry/outbox生产适配根统一未齐；ConsumeAuditEvent本体有真实来源V2链但不计完整事件入口ready。
- 公共Manager快照/屏障/完整inventory/恢复Migrate及target epoch清旧授权未装；Acquire/Release/Create/Stage/Discard、完整BuildApplication/Ready/启动恢复仍blocked，绝不把目录存在当safe/恢复完成。
- 原布局控制区与active隔离；恢复时不可复活aggregate_jobs旧fence、export旧epoch授权或原日盐，原audit来源/hash不改，付费事实只保真实引用。
- Root须扩framework登记web/package.json/tsconfig.json/vite.config.ts/src/style.css，CI覆盖新analytics UI并生成lock；另本批函数从Vue参考迁client.ts，合同/进度已按实际同步。
