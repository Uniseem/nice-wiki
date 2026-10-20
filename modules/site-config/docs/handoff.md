# 2026-10-20 R1-F02恢复WIP接入main后的窄修交接

本工作树继续旧恢复WIP `30ac79fcf878d1f3eb3fafb472204edae19bae4b`，已普通 merge `main=6204dfc94510f28752fe8da81df9f3a4d3ce781d`；源码与合并窄修提交为 `c3c7c10c11320ebff45c994f8458e54604a75343`。主线 pipeline173/174 已验证的激活回执验签与 `RegisterActivation` 实作保留在合并源码中；本轮新/改恢复、快照、Relay、Build 接线仍待 GitLab CI，继续保持待验证。

本轮只在 `modules/site-config/` 内修复公告 journal 单条隔离，并按公共屏障协议复核：非 jobs 只封本域真实水位并使用同一无 `nonJobs` 字段 proof，不等待最终 broker 回执；最终 broker 与13域回执由 jobs 阶段核验。恢复迁移机器合同依赖统一记为 `platform.MigrateRestoredDatabase`，实际Go包调用仍由公共outbox实现；公共 `RegisterWorkTable` 同样按 `platform.RegisterWorkTable` 机器ID登记。当前无真实GlobalSeal协调器caller，已删除未使用的配置hook源码/合同；`verifyGlobalSeal` 保持失败关闭，等待jobs协调器和集成启动合同冻结。未运行本机/SSH Go、pnpm、生成、格式化、编译、vet、test、make check、真实接口、模型、服务或部署；不 push。

## 本轮结构合同收口

- 从集成候选 `6f25dd57386081b5903bc6c02f448ea769466618` 建立独立分支，仅修改本模块。
- 删除四个 activation verifier 重复函数登记，合并重复 `snapshot_service.go` future file 时保留 `ConfigureGlobalSealVerifier` 以外的全部原函数约束、步骤和依赖；`ExportSnapshot`、`StageSnapshot`、`DiscardSnapshot` 归入唯一文件条目。
- 为 `SnapshotManager`、`SnapshotCleanup`、`OutboxStore`、`SiteConfigClient`、`SigningKey`、`SwapResult`、`SnapshotSwapEvidence`、`OutboxObservation` 等真实叶类型引用本域权威平台合同；`WithRelayRuntime`、`WithSnapshotRuntime`、`authorizeResourceReferenceRead` 接入实际源码 caller 的合同依赖。
- 删除无源码 caller 的 `ConfigureGlobalSealVerifier` 参考函数源码/合同登记；`verifyGlobalSeal` 仍失败关闭，等待 jobs 协调器和集成启动合同冻结。
- 机器依赖统一为 `platform.RegisterWorkTable`、`platform.MigrateRestoredDatabase`；Go 实现包路径保持公共 outbox。生成函数/文件文档留给集成代理统一 `make docs`。

# 2026-10-20 R1-F02首批源码待根审核与CI

## 交付

- 分支 `codex/closeout-20261020-site-config`，基线 `8606ae365af973050ced01ed435c4b951642fe95`，首批源码 `42b30e419e9d6df965bbd2e4dea2a5af07d3aee6`，审核修正 `b5895d88e30099235378b6095fc43410845c76fa`。
- 实际实作共同作者：GPT-6 `<noreply@openai.com>`。
- 独占修改 `modules/site-config/`；根 framework、生成目录、根状态、公共 contracts、`libs/platform`、`deploy/` 与其他模块均未修改。
- 新增 [resource_reference_reader.go](../internal/resource_reference_reader.go)，并在 [outbox_repository.go](../internal/outbox_repository.go)、[server.go](../internal/transport/server.go) 完成对应函数和路由接线。

## 已写职责

- `ListResourceReferences` 从当前 active epoch 的真实 COMMITTED settings 修订生成完整资源引用页。来源规则直接复用现有 pin 代码：home 为 `home.background`、`home.poster`，品牌为 `branding.logo`、`branding.favicon`、`branding.share`；来源对象哈希、revisionId、sourceVersion 与 `referenceDigest` 材料保持同源。
- 当前 revisionId 产生 `current`，旧 COMMITTED revision 产生 `history`；全部为 `confirmed=true`、`expiresAt=null`。分页使用稳定引用位置签名，scope 固定绑定 caller、actor、site、epoch、assetId 筛选与limit，watermark 与位置放入验签payload；水位变化后返回 VERSION_MISMATCH。
- HTTP 授权与合同统一为 media/data-protection 纯服务或同站点 `site.manage/superadmin`；portal 委托不能绕过 identity 即时权限。
- `RelayOutbox` 仅接收完整 Application 注入的真实 active outbox Store、jobs publisher、Runtime/WriteWindow、AdmitBatch 与 BindEventScope，并调用平台 `RunRelayBatch`；没有复制领取、重试、隔离或旧epoch循环。
- `config/service.defaults.json` 与 deploy manifest 增加 `data-protection` 私有依赖地址，和现有服务端口/环境变量命名一致。

## 复核与未闭项

- `ValidateLanguageTag`、`ReadRuntimeState`、`All` 复核为与内部参考合同一致，未改写。
- `RecoverAnnouncementJournal` 原合同要求缺 head/正文等证据的单个 COMMITTED 公告隔离并告警；现有代码整体启动失败关闭，是有明确依据的不一致，保留原合同并把实现修正列为本包待修，不扩大本轮业务语义。
- `CheckWriteEpoch` 现有代码只检查 running/epoch，未满足合同要求的同一 SnapshotManager barrier/released gate，保持 blocked，待共享接口冻结。
- `RelayOutbox` 单批函数源码已写，但尚无同一 SnapshotManager，因此不能装 BackgroundTasks，按ADR0004当前不可达；完整启动仍 blocked。根不得通过新增伪造依赖边把它标为可达，真正后台接线后才提交完整 Relay 交付。
- 新文件需根 framework 登记；OpenAPI、files/functions/api/workflows 生成物需根 `make docs`。`gen` 未手改。
- 完整 `check_docs` 当前只剩根登记/生成漂移与 Relay runtime 不可达；privacy-only、JSON解析和 `git diff --check` 通过。

## 下一步（一项）

主代理审核首批提交与修正提交范围；批准后由根登记新文件、执行生成并统一运行GitLab CI。Relay 真正后台接线、公告恢复差异修正及 SnapshotManager/activation/barrier/快照仍留在本包后续，不提前伪造完整启动。
---
# 2026-10-20 持久激活回执验签源码单元已通过

## 交付

- 候选分支 `codex/activation-receipt-20261020-site-config`，基线 `a32f3f4c4876f94ce798826e9dee0b170bbb612a`。
- 实现提交 `1075bcb8824838874e6a3ed775778f4194fa6969`，共同作者 `GPT-6 <noreply@openai.com>`。
- 独占修改四个源码/合同文件：`internal/app/wire.go`、`internal/runtime_store.go`、`internal/runtime_transition.go`、`contracts/implementation.json`；未修改公共 `libs/platform`、OpenAPI/types、生成目录、根文件或旧WIP。

## 本轮实际完成

- `BuildApplication` 继续通过既有 `NewRuntimeClient(config)` 工厂满足 `ServiceSpec.RuntimeClient` 固定签名；随后调用本域 `ConfigureActivationVerifier(anchors)`，把已加载信任锚装入同一 `RuntimeStore`。runtime monitor 的无锚调用只打开/复用读取器，不覆盖验签锚。
- `RegisterActivation` 规范化 `RecordedAt` 为UTC `RFC3339Nano`，保持同operationId同摘要直接回放；新操作校验当前epoch、revision、restoreId、phase/state后读取可信锚，绑定 `restoreId` 与receipt的 module/stageId/targetDataEpoch/state/manifestHash/recordedAt，调用 e85 分支精确接口 `serviceauth.VerifyActivationReceipt`，验签通过后才写现有回执/CAS事务。
- 合同新增并登记 `ConfigureActivationVerifier`、`RuntimeStore.ConfigureActivationVerifier`、`RuntimeStore.activationVerifierAnchors`、`verifyActivationReceipt`，HTTP形状没有变化。公共平台首单元只需提供 Verify 所需助手，并须让 Sign/Verify 在hash前采用相同 UTC `RFC3339Nano` 时间规范。

## 检查与状态

已执行 `jq` JSON解析与 `git diff --check`，均通过。按用户规则未在本机/SSH运行Go或pnpm生成、编译、vet、gofmt、lint、类型检查或测试；未执行真实接口、模型、服务或部署。pipeline173（候选`eb96455b2366ac8511c471e7955de41b5ffdab97`）的生成/格式696、foundation697、content-media698、ai699、前端700全部成功，generated.patch为0字节；本轮8个函数及runtime_store/runtime_transition文件按范围verified，BuildApplication/wire/完整服务仍blocked。

CI170在`make docs`阶段因本轮新增合同的非空输入/输出缺少`constraints`触发KeyError，尚未编译。修正提交 `aaa35cbd` 仅补齐 `RuntimeStore.activationVerifierAnchors` 两个outputs、`verifyActivationReceipt` 三个inputs和一个output的constraints；progress合同哈希已同步为 `b3a1647f9ad5fa168bfba5ffeb8701d779d9f683146436c1996ef8407adc91eb`，源码与状态等级不变。

旧的 `resource_reference_reader.go`、RelayOutbox及其状态只存在于原 `codex/resume-20261020-site-config` WIP工作树/历史链；本候选从main基线独立建立，集成时不应整条cherry-pick旧WIP。

### 收尾

主代理已复核本源码单元范围。用户当前要求结束并push，停止后续实施；完整BuildApplication、wire、Relay、快照/屏障和恢复回执继续blocked，后续WIP只留本地，不进入本次交付。
以下为历史记录。

# 2026-10-18第三批源码检查通过

自部署GitLab pipeline95（源码1e65c5f；生成436、三个Go分片437/438/439、前端440）全部成功；16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及三个实际前端类型检查/构建通过。精确生成补丁零字节。普通测试均无测试文件；不代表业务验收或部署。

本域本批完整函数2项通过源码检查。完整服务、快照/恢复、业务验收按逐项台账继续未完成；不把部分装配记完整。未本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 站点配置与初始化会话交接

分支 `codex/site-config-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`，源码 `37097fce696c8d0093bd58476f385d142ab536d8`。未跑 GitLab CI，未编译，不记 verified。

完成函数：UpdateBranding、UpdateFooter、UpdateBanner、ReadConfigHistory、RestoreConfigRevision；闭环助手 ValidateAnnouncementBody、WriteAnnouncementRevision、CommitAnnouncementHead。

仍失败关闭的资源引用：合同未写明的品牌资源（favicon 只接受 `image/` 与 kind=image，不猜测 MIME，无确认字段，不读原始字节）；恢复只 `readAsset`、不获取新 pin、不解析文档链接；空品牌 pin 若媒体拒绝则封存；公告正文缺失或不一致则拒绝回滚；清横幅、当前横幅无 head、JSONL/head 已写而配置 SQL 未提交的窗口沿既有恢复失败关闭。完整启动继续失败关闭。

### 下一项（一项）

品牌、页脚、横幅和配置历史已由 GitLab pipeline 42 记 verified。changedSections 已含 branding、footer、banner。下一步不要补完整启动；从合同步骤已经写明、仍为 planned 的函数继续。恢复继续只 readAsset。

## 集成需处理（不是本域下一步）

- 在根 `docs/structure/framework.json` 登记五个新文件：`internal/branding_service.go`、`internal/footer_service.go`、`internal/banner_service.go`、`internal/config_history.go`、`internal/announcement_validator.go`。
- `make gen` 后 OpenAPI 的 `changedSections` 才包含 branding、footer、banner，且 maxItems 为 9。本域 journal 手写校验已按九组失败关闭。
- 品牌 confirm 走既有首页引用恢复循环；pin 封存可能返回 CONFLICT，这是首页协议，不是第二套 pin。
- 不要在本域展开公告产品或补齐 BuildApplication。快照、屏障、资源引用列表、RelayOutbox 与恢复回执仍缺真实 handler。

以下为历史交接。旧“下一项”已由本轮承接，不能再当继续授权。

集成实际源码ce37652，记录7addf01；root实际树88210ba已在VPS通过docs/gen、14模块readonly build/vet/gofmt与完整check（858/1109/379/2256），生成零变化。progress已映射实际SHA；后文aa96f42/92caa2b保留原分支溯源。

## 第四轮首页权威与引用恢复（2026-10-15，已交付冻结）

- 源码 `aa96f429e0eecc45cf6a6c51773a8b1892bde648`；合同-only原提交 `fa2d1b7b738c25dbd38daebc4149a8c9a2181c67` 已根集成6372a32。独占modules/site-config/；分支codex/site-config-home-r1，基线de21a2bcb5e1a8b25e41ec84ba3734fc163b5ab1；主线实际cherry-pick SHA由根统一映射。
- 完整交付UpdateHome/ReadPublicHome/ValidateHomeLinks，及RecoverHomeReferences/ReadHomeReferenceDecision、RecoverHomeReferenceState和追加0005。一屏hero保留0–3稳定按钮、视频poster与完整AssetVersionRef，不新增卡片精选、付费任务或自动重算。
- 固定候选与完整排序refs/acquire intent同事务，sourceObjectId为HashBytes("site-config.home.v1|"+siteId)，sourceVersion用全局config.revision，sourceRevisionId为实际不可变修订，空[]同协议。先持久pin再COMMITTED，confirm_pending同正式SQL/原回执/outbox事务；旧成功只读原正式结果，已有文件决定只补原水位，不重查新可见性/ready/expiry。
- 终止恢复直接Release完整source、nullable pinId与终局reason。无COMMITTED才与提交共锁持久aborted；媒体回源明确aborted/rejected后原子sealed，无pin也封存，迟到Acquire不得复活。终止不重Acquire，不把404/pending/未知/超时当释放证明；COMMITTED只confirm持久pin，缺失完整性失败。
- ReadPublicHome使用独立公开水位和显式安全投影，不改变私有配置授权。保留全部按钮，文档可见语言必须等于请求语言，发布ref含正revision/schemaVersion，不可见href/publication/resolvedLanguage均null；拒绝query/控制空白和非请求锚点。QA免费只读，无配置/失联禁用按钮，hero仍返回；Footer沿既定原形，banner仅当前启用窗口。
- 构造只核本地RecoverHomeReferenceState；全部真实handler齐备且Build成功返回后，RunService才启动BackgroundTasks/公共RunRecoveryLoop。域内无timer/构造RPC；每RPC前后复用NewWriteContext核安全窗口、原op及active epoch。Drain只排HTTP，等后台结束再Shutdown关库。完整Application因其他handler及转发/恢复回执未齐仍blocked。
- 最终统一VPS linux/amd64 Go1.25.5 GOMAXPROCS=8：docs/gen、14模块readonly build/vet/gofmt、完整check通过（858文件/1109函数/379操作/2256类型），纳入全部Home Go/0005/最终修正，0回传冲突；已实现完整函数/文件记verified。没有行为验收、联调或真实服务启动，不扩展结论。
- 新实际文件3个：internal/home_service.go、internal/home_validator.go、internal/migrations/active/0005_home_reference_decisions.sql，根统一登记。公共依赖9ff93ae71e8f262ea0fc057aa3f6eea71ce52f22及后续media封存/SSE，由根合成；gen/docs为VPS回传。无新库/测试/mock/fixture/样例/临时业务验证程序，无本机Go工具、真实接口/模型、服务、部署或push。

### 历史下一项（已由本轮承接）

推进UpdateBranding品牌权威组：先核机器合同和本轮固定候选/来源决定机制，明确品牌资源的media pin来源与失败封存恢复，再接既有settings不可变提交链；公共协议缺口先交根统一收口，不复制机制。当前本域交付后冻结，不自行展开该包。

## 第二轮最终集成与停止范围

- 一般设置权威读写、校验、对象原始流及HTTP核心通过；Home等及完整启动仍未完成。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


## 持续实施第二轮：一般设置与原始对象读取（2026-10-15）

- 基线 `468986fc34e1c90b6b5ca575539fdd2b9748cd06`，分支 `codex/site-config-settings-r1`，独占本域；旧初始化分支保留。源码提交 948a834、39ddb52、315a3ff、9a21f1b；已交付核心经最终隔离 VPS 门槛与统一复核，记 verified。依赖根公共26138c1及后续统一修复，由集成会话合入；本域仅交付自身变更。
- 完成源码：ReadSiteConfig、UpdateSiteConfig、ValidateSiteConfig、NormalizeBaseURL、ReadConfigObject；ConfigureInitialization 扩受信 allowLocalHTTP；CommitConfigHead 补已完成原始回执优先返回，避免同操作并发晚到后错误覆盖/拒绝旧成功。
- 普通配置候选材料和原始 actor/site/epoch/hash 同事务保存，固定首次 updatedAt 和 revisionId。只从真实 COMMITTED 权威文件返回正式/原始结果；未提交普通候选不在启动自动发布。新迁移对对象 ID/站点修订建立唯一索引。
- 对象协议：version/hash 必填且严格类型；HTTP 返回原始对象字节，绑定 ETag/Content-Length/Type/对象 ID/版本，Reader 仅本进程使用并关闭，不造内存句柄表。当前仅支持已真实提交 settings，其他对象类型不存在完成索引时 NOT_FOUND。
- 原始严格输入的规范 JSON（含 kind/expectedRevision）先作业务摘要并读取旧成功；只有新操作/未完成操作应用当前受信能力，候选材料 hash 另存规范后的完整内容。部署撤销 loopback HTTP 能力不能阻断旧正式结果读取。
- 根裁决的公共前置：lifecycle.NormalizePublicURL(raw,allowLocalHTTP)，根负责 Go 官方 IDNA 依赖、许可和 VPS mod/sum；本域不修改 libs/根文件或手写依赖和生成物。公共 HTTP 身份/操作上下文/schema 工厂接口冻结后再接线。
- 新增实际文件 4 个需根 framework 登记：internal/config_service.go、internal/config_validator.go、internal/object_reader.go、internal/migrations/active/0004_config_operations.sql。未新增测试、样例、fixture 或临时业务验证程序；所有格式/生成/编译/vet/检查只交根统一 VPS 执行。
- HTTP 适配已按冻结工厂签名接真实已实现的 config/init/runtime 操作；固定 caller、纯 service actor 子白名单和维护控制白名单由本域装配，统一身份/操作上下文/schema/identity 授权机制只用平台。原始对象流设置写截止、绑定元数据、消费关闭，响应开始后不再写 JSON 错误。
- 全部 OpenAPI 必须绑定真实 handler；当前 Home/品牌/公告/历史/快照/屏障等尚无实现，NewServiceHandler 会拒绝启动，BuildApplication 继续 blocked，不能把部分 HTTP 映射当完整模块就绪。
- 首轮合成VPS快照315a3ff+公共26138c1的docs/gen和14模块readonly build/vet/gofmt通过，check余4项均非本域。回传生成物/gofmt/modsum后，按交叉复核追加updateSiteConfig维护边界：只读旧成功路径列白名单，其余新/未完成操作在任何候选SQL/文件IO前CheckWriteEpoch，Commit内部CAS门禁不省略；该追加修复已被最终VPS复跑覆盖。
- 最终VPS：make docs/gen成功、全部14模块readonly build/vet/gofmt及完整make check通过（855文件/1012函数/374操作/2211类型），含候选IO前写门禁和维护只读旧回执；格式/生成物已回写。仅对应检查范围，无SQL/HTTP行为验收、真实接口、模型、服务或部署。
- 用户最新指令：第二轮完成和本轮获授权推送后停止、清空持续目标，不启动第三轮。本域冻结、不push；剩余Home/完整HTTP/后台/恢复事项不因停止而标完成。
- Home 组明确留待下一紧密组，不写假完成函数：ReadPublicHome 缺按钮 disabled/解析 URL 与 AI 可用性权威表达；UpdateHome 需持久 media pinId/确认阶段及 COMMITTED 后崩溃续办。不能删按钮冒充禁用，不能省略恢复路径。

### 下一步（一项）

交由集成会话合并本轮模块提交并按已授权范围完成推送；本域冻结，不自动开展第三轮，不新增验证范围。后续实现须由用户新的请求重新启动，Home等剩余项保持原状态。本轮编译与结构检查不代表业务验收或全部产品完成。

---


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
# 2026-10-20 恢复全包WIP停止交接（用户结束）

本域原 `resume` 工作树已按用户“先结束然后push”指令停止新增功能；本代理不 push、不回退、不 reset、不删除旧 WIP，也不启动新检查。源码/合同交付提交为 `30ac79fcf878d1f3eb3fafb472204edae19bae4b`，父提交为 `6d83888b39ebbd3a624f58d820274e7c6f3072c7`，分支为 `codex/resume-20261020-site-config`。

本轮 WIP 包含真实激活回执证据读取与验签、active 业务表登记、同事务 outbox 观察/封口/重开、SnapshotManager 屏障与 Ready 门禁、Capture/Stage/Archive、公告单条隔离、首页 cleanup、Relay 后台及完整 Build 源码/合同/业务文档。公共 `platform.*` 实现、GlobalSeal、生成物和集成 CI 仍由集成代理负责；未 CI 的新/改函数和文件均保持 `in_progress`/待验证，不写 `implemented` 或 `verified`。

本地收尾前的脏文件仅为本域 `STATUS.md`、`progress.json`、`docs/handoff.md`；提交后工作树应干净。验签独立小候选的三份元数据由集成代理接管，本分支不与其互改。

唯一下一步：集成代理接收 `30ac79fcf878d1f3eb3fafb472204edae19bae4b`，统一登记/生成并运行授权 CI，再按精确 SHA 更新台账。用户已结束，本域停止工作。

以下为历史交接记录。
