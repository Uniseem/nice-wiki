# 站点配置与初始化会话交接

## 持续实施第一轮最终集成事实（2026-10-15）

- 初始化与settings权威提交、恢复和文件IO通过；配置编辑、完整HTTP/后台仍未完成。
- 主线实际源码提交：a0d6adf、0e8cc9f、c2d734b；模块分支交付经cherry-pick后SHA已在progress.json映射，后文原分支SHA保留用于溯源。
- 三域逐个合入后各自VPS make docs/gen、14模块readonly build/vet/gofmt与完整make check通过；最终850文件/943函数/372操作/2186类型，生成回传零变化。现有设计检查补核managementPermission与jobManagementGrants集合一致。
- 全部编译/格式/vet/生成检查均在VPS；没有新增测试，也没有运行业务测试、SQL行为验收、服务/真实接口/模型、镜像、部署或push。部分文件与入口继续in_progress/blocked，不把检查当业务验收。
- 下一项以本节和根plan最新工作包为准，后文“等待VPS”等旧说明为过程历史。


## 本轮 R1-02 初始化完成与 settings 权威提交（2026-10-15）

- 基线 `325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，分支 `codex/site-config-initialization-complete-r1`，独占 `modules/site-config/`。本轮源码提交 `5b6bc4e39fd72457d302031e0f369cece55ae1c5`，格式/生成/台账提交 `6396715d91a56bb6a12cb4ddd151b4117ff27520`；旧分支保留。配置 IO 边界修复 `53156c7d74fb065b1355250ab916d03518c49d0b`、格式/生成与 vet 修正 `8613254c72d63ddf30ce3710cf222d21a7738181` 均已通过本轮 VPS 复跑。
- 实际链路：InitializeSite → PersistInitializationDecision → ResumeInitialization → WriteConfigRevision → CommitConfigHead → PublishConfigEvent；AbortInitialization 在 commit 前持久取消与原始结果。准备材料哈希和原始业务请求哈希明确分开，重试不重造 preparedAt。
- 权威文件使用 ConfigDocument v1（schemaVersion/config），不可变对象临时写/fsync/Link/fsyncparent；PREPARED/COMMITTED 独立原子记录，head 临时写/fsync/Rename/fsyncparent。COMMITTED 落盘后只能补完，不能以 SQLite 准备材料冒充权威文件。
- CAS：不可反悔文件决定前取得 SQLite 写锁，检查目标水位没有另一 COMMITTED；SQL 同事务写索引/配置水位/原始回执/平台 outbox 与 config_committed，另事务才置 completed/initialized=true。允许重启恢复中间水位，后续配置修订不改首次初始化结果。
- 启动先 RecoverConfigJournal 核明确提交决定及权威文件，补完 SQL 和 head；孤儿 PREPARED 隔离，有初始化 commit 的准备记录留给 Resume。非维护状态下继续未完成初始化。未知 schema、坏 hash、缺文件或原始身份回执失败关闭。
- 集成裁决：公共 ServiceConfig 新增可选 publicBaseURL/allowLocalHTTP，边界校验 https 或显式 loopback http；本域 ConfigureInitialization 接收受信结果，缺失仅在初始化使用时失败。PublishConfigEvent 加本域 WriteTransaction 参数复用平台 outbox。
- 新增实际文件：internal/config_files.go、operation_repository.go、outbox_repository.go（均已有未来结构），以及 internal/migrations/active/0003_config_commit.sql（新增迁移合同）。上述 4 个新增实际文件均需集成会话登记根 framework。新增合同入口 RecoverConfigJournal、参考 ConfigureInitialization，Resume 依赖改为真实配置链路。
- 检查：集成会话隔离 VPS `make build` 通过全部 14 个 Go 模块 readonly build/vet/gofmt（BUILD_STATUS=0），`make docs/gen` 成功并回传本域产物；`make check` 仅余公共函数可达性与新增实际文件登记，本域 4 个由集成会话登记。该条为首轮历史证据，最终以随后追加修复复跑记录为准；本轮不在本机编译、格式、lint、类型检查、测试或生成校验。未新增测试/样例/fixture/临时验证程序，未运行真实接口/模型/服务/联调/镜像/部署/push。
- 集成逐函数复核追加修复：settings 文件 IO 集中使用 os.Root 能力逐段 Lstat，父目录链拒绝链接/非目录，正规文件 NOFOLLOW/NONBLOCK 有界读取并核打开前后身份；正文、head、journal、不可变复用和隔离标记均限 16 MiB。该追加修复已通过后续隔离 VPS docs/gen/build/check，不新增测试。
- 最终追加修复检查：8613254 快照在隔离 VPS make docs/gen 成功，全部 14 个 Go 模块 make build（readonly build/vet/gofmt，含本域）及完整 make check 通过（850 文件/943 函数/372 操作/2186 类型）；先前 configIOError 非恒定格式串 vet 失败已修复并复跑通过。结合集成逐函数复核，已交付完整函数记 verified，仅对应静态/编译/结构范围，不标行为验收通过。
- 剩余装配：全量 x-handler、RelayOutbox、恢复换目录回执、快照与屏障仍未完成，不能把本组源文件或结构检查当作完整站点行为验收。

### 下一步（一项）

在集成会话统一 VPS 门槛和逐函数复核通过后，接入本域初始化/配置读取的 HTTP x-handler 与平台入口校验，先读 OpenAPI、types、初始化和配置恢复合同；同时使用既有可信 publicBaseURL 注入。完成条件是已实现 handler 调用本组用例并正确返回已持久结果，不包含真实接口调用、行为验收或部署。

---


## 本轮统一复核结果（2026-10-15）

- 三个模块在独立worktree交付，集成分支逐个审查新增逻辑与合同；最终隔离VPS linux/amd64 go1.25.5、GOMAXPROCS=8 的14模块make build和完整make check通过。编译、结构检查不代表业务验收。
- make docs无漂移；make gen第二次运行零变化，核对发生在用户追加VPS专用要求之前；后续所有编译/测试只在VPS，VPS不可用则blocked，不自行本机回退。本轮没有新增或运行业务测试、真实接口、模型、服务或部署。
- 根framework已登记本轮10个实际文件，生成器与消费者统一刷新；本轮已完成函数和完整文件按证据转verified，部分文件与入口仍如实in_progress/blocked。主分支实际实现SHA以progress.json为准。

## 本轮 R1-02 初始化准备与决定（2026-10-15）

- 基线 `00dd199239aba487bceb5227aac52f27ce949d74`，独占分支 `codex/site-config-init-r1`，范围仅 `modules/site-config/`。公共 nullable 生成修复及 content 适配由集成会话提供，作为前置提交引入，不计本域交付。
- 本轮代码提交：`02dfb5bb8c46d51b4506bfd42a049a85422ec0ff`；未提交业务文件为空，台账同步另作提交。
- 实际函数：PrepareDefaultConfig、ReadInitializationStatus、PersistInitializationDecision、ReadInitializationRecord。配置准备使用可信 preparedAt，所有默认值明确登记 data/types/implementation；不读取 Host、不触发 AI、不制造示例。
- 类型差异：InitializationState 的 operationId/siteId 可 null；状态接口固定 initialized + 安全 state，完整 coordination 仅同站点超管或 identity/site-config 服务返回。消费者 identity/portal 的生成类型需集成会话统一刷新，路径/operationId 不变。状态完整性错误补 OpenAPI 422。
- 决定表保存完整规范准备配置及 SHA256，CAS 当前状态与原始幂等结果同事务提交，FULL 同步；commit 后保持 preparing。未实现 settings 权威文件提交、ResumeInitialization 或完整 InitializeSite，不能宣称初始化链路已打通。
- nullable 修复的本域适配：RuntimeState 的 restoreId/targetDataEpoch/barrierId 保留 null，比较按值、SQL 写入空值、恢复完成清空回执；platform RuntimeView 的内部可选字符串仅在适配点转换。
- 检查：本机 Go 工具链已找到并使用，本域 readonly build/vet/gofmt 通过；全仓 `make build` 通过（14 个 Go 模块 readonly build/vet/gofmt）；`make docs` 已生成本域文档；`make check` 仅因根 framework 尚未登记本轮 4 个实际文件失败，其余合同/台账/引用检查通过，由集成会话统一登记后复跑。未新增测试/样例、临时验证程序；未调用真实接口/模型，未运行业务服务或部署，未 push。
- 公共结构请求：新增已获集成会话许可的 `modules/site-config/internal/migrations/active/0002_initialization.sql`，请根 framework 登记；新增 ReadInitializationRecord 为本域私有参考函数。

### 下一步（一项）

实现 InitializeSite/ResumeInitialization 与 settings 权威文件提交这一组紧密关联函数：先读 types/implementation 中 WriteConfigRevision、CommitConfigHead、PublishConfigEvent 及本轮完整准备材料格式，补齐权威文件 journal/receipt/outbox 的原子恢复边界；可信 publicBaseURL 进程配置注入由 HTTP 装配工作包统一接入。完成条件是持久 commit 可从 preparing 幂等推进 initialized，不包含真实行为验收或部署。

---

以下为先前会话历史，当前事实以本轮记录和 progress.json 为准。


## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- 修复 32 处编译错误：两个工作包重名的 `activeStore` 改为 `currentActiveStore()`，补 `database/sql` 导入，`RecoverAnnouncementJournal*`、`ReadRuntimeStateInput`、`ValidateRuntimeTransition*`、`CheckWriteEpoch*` 改在本域声明；types.json 变更后重跑 make gen。
- 完成换回只要求已换入的域全部 rolled-back（不得残留 switched 回执）；从未换入的域不产生回执，不再导致换回永远无法完成。合同步骤同步说明 begin_switch 的暂存完成由 data-protection 核验。
- 公告恢复时，被后续修订取代但确实提交过的 PREPARED 不再计为孤儿；可选时间戳为零值时按完整性错误拒绝。修正 go.sum 中抄错的校验和。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。
- 遗留：已提交恢复后平台对“running 阶段仍残留 switched 日志”的处理需在恢复工作包核对。

## 本次事实（R1-02 语言标签纯规则切片，2026-10-15）

- 独占范围：`modules/site-config/`；实际共享工作区分支为 `codex/analytics-r1-storage-contract-audit`。并行 `jobs` 会话留下的工作区变更未暂存、未修改、未纳入本提交。
- 本次实现提交：`a2599e2`、`663bf07`。只新增/修订 `internal/language_service.go`，未修改根文件、公共契约、生成目录、其他模块，未新增测试/样例、真实接口、模型、部署或 push；提交窗口误带入的 `jobs` 变更已在本地拆回未提交工作树。
- `ValidateLanguageTag` 已按本域类型合同实现：输入结构、BCP47 基本语法、扩展/私有子标签、重复子标签拒绝，以及语言/脚本/区域/扩展大小写规范化。
- 已运行 `python scripts/check_docs.py --privacy-only` 与 `git diff --check`；本机没有 `go`/`gofmt`，未执行格式化、构建、vet、普通测试或业务验收。

## 本次阻塞与契约缺口

- `PrepareDefaultConfig` 的输入需要 `publicBaseURL`，但初始化 HTTP 输入和 `ServiceConfig` 都没有受控来源；合同也未给出主题色、资源限额、安全策略、源码链接的默认值。
- `InitializationState` 要求未初始化状态仍提供非空 `operationId` 与 `siteId`，但身份服务尚未向该函数提供合法来源；不能生成占位 ID，也不能提前创建站点。
- 因上述缺口，本次没有猜测实现初始化、持久 commit、默认配置文件或 HTTP handler；`BuildApplication` 仍因完整 x-handler/恢复回执/outbox 等后续装配保持 blocked。

## 下一步（一项）

先由设计/集成会话补齐上述受控字段来源与未初始化状态表示，再在具备 Go 工具链后运行本域 gofmt/build/vet；之后实现初始化决定持久化与 handler，不把语言纯规则的提交标记为业务验收通过。

## 本次事实（R1-02 active 业务存储与公告 journal，2026-10-15）

- 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`；分支：`codex/site-config-r1-02-active-storage`。
- 独占范围：`modules/site-config/`。未修改根 AGENTS/STATUS/README、根 docs、scripts、根 contracts、libs、deploy 或其他模块；未读取 `vps.txt`、old 分支或旧业务源码。
- 本次只实现 active DB 迁移、公告决定 journal 恢复和 Ready schema/journal 校验；未实现 HTTP 业务路由、outbox 循环、后台任务循环或真实恢复联调。
- 实现提交：`d2a7f9100a3de824cecba9d66eeb645fdf0196e5`、`02b18cb0ed1c97949730ef1c1b6b62047710520d`；台账同步提交：`9656920`；未 push、未部署。

## 本次完成

- `internal/migrations/active/0001_active.sql`：新增 active 私有 SQLite 迁移，建立 schema_migrations、site_config_state、config_revision_index 与 announcement_heads；正文文件仍是权威数据。
- `internal/runtime_store.go`：新增 active 数据库句柄、WAL/foreign_keys/busy_timeout=3000ms 单连接、连续迁移校验、epoch/初始化状态校验及 `ValidateActiveReady`；control.db 仍独立于 active 恢复换目录。
- `internal/announcement_files.go`：实现 `RecoverAnnouncementJournal`；严格解析 JSONL，COMMITTED 逐项核对 head、对象路径、大小、hash 与公告正文 schema，孤儿 PREPARED 写入私有隔离区，不推定为已发布。
- `internal/app/wire.go`：BuildApplication 在无换目录动作时打开 active 库并完成 journal 恢复；任何迁移、epoch、对象或 journal 失败均返回错误，不创建可用应用。
- `internal/transport/server.go`：Ready 接入 active schema、epoch、journal 指纹和初始化状态；Shutdown 关闭 active 与 control 句柄。
- `contracts/implementation.json`、`docs/files.md`、`docs/functions.md`：登记 active 迁移及两个存储校验函数，并用 `python scripts/render_docs.py` 生成同步文档。

## 状态区分

- 已实现：上述 active 存储、迁移、journal 恢复、Ready 校验和启动接线代码；代码提交后以实际 SHA 回填台账。
- 已验证：JSON 解析、生成文档同步、隐私静态检查与 `git diff --check`。
- 未验证：Go 格式化、依赖 tidy、build、vet、现有普通测试和业务行为验收；当前环境没有 `go`/`gofmt`，也未读取 VPS 私有连接信息。
- 未验收：M03-R01～M03-R06、崩溃恢复、并发 revision、未知 schema/损坏对象的运行行为和部署均未执行。

## 遗留阻塞与契约决策

- BuildApplication 仍 blocked：完整 OpenAPI x-handler、换目录后的 RegisterActivation 回执、outbox/后台循环和其余业务用例尚未实现；本次没有用占位成功值掩盖这些缺口。
- active journal 的 COMMITTED 记录要求对应 `active/announcements/<announcementId>/head.json` 与不可变正文对象；缺失、hash/大小/schema 不一致会阻止启动和 Ready。孤儿 PREPARED 只隔离并计数，不会公开。
- 未初始化 active 状态返回 Ready 的 `initialized=false`、`activeDataEpoch=null`；初始化后的状态必须有匹配的 config_revision_index 与 runtime dataEpoch。
- 公共 `docs/structure/framework.json` 以及根第三方声明属于集成会话所有权；本会话只在交接报告，不顺手修改。

## 下一步（一项）

在具备 Go 工具链后，先对本提交执行 `gofmt`、`go build ./cmd/site-config` 与 `go vet ./...`，再由集成会话登记公共结构文件并复跑根 `make check`；随后才实现本域初始化/配置业务函数。
