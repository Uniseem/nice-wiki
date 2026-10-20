# 2026-10-20 持久激活回执验签候选待CI

独立候选分支 `codex/activation-receipt-20261020-site-config` 从 main=`a32f3f4c4876f94ce798826e9dee0b170bbb612a` 建立；实现提交 `1075bcb8824838874e6a3ed775778f4194fa6969`。本轮只拥有 `modules/site-config/`，没有带入资源引用列表或Relay旧WIP。

- `BuildApplication` 保留 `NewRuntimeClient(config)` 的进程模板固定签名；可信Build在control store打开后装配已加载的TrustAnchors。runtime监视器调用只读工厂，不传空锚、不覆盖已装配验签器。
- `RegisterActivation` 保留原HTTP输入/输出形状和同operationId幂等回放顺序；新操作在当前dataEpoch、revision、restoreId、phase/state检查后，以 `input.restoreId` 和receipt全部签名字段构造 `ActivationReceiptClaims`，调用精确 `serviceauth.VerifyActivationReceipt`，再沿原control.db CAS事务upsert。
- `RecordedAt`在claims及规范化input中统一为UTC `RFC3339Nano`；公共Sign/Verify两端的同一时间规范化是集成公共首单元的窄依赖，不在本提交修改公共文件或引入新签名框架/schemaVersion。
- `modules/site-config/contracts/implementation.json` 已登记Build→Configure→RuntimeStore.Configure、Register→getter/verify→platform.Verify的真实边；progress.json 已同步合同哈希和本轮函数/文件状态。
- CI170在`make docs`阶段因新增验签合同的输入/输出缺少`constraints`而KeyError，尚未进入编译；提交`aaa35cbd`仅补齐六个缺失约束，源码不变，待集成CI重跑。

本轮未运行本机Go/pnpm编译、格式化、lint、测试或生成；未新增测试、样例、fixture、mock，未运行真实接口/模型/服务/部署。源码与合同待本轮GitLab CI，相关状态保持 `in_progress`/`blocked`，不记 `implemented`/`verified`。

以下为历史记录。

# 2026-10-18第三批源码检查通过

自部署GitLab pipeline95（源码1e65c5f；生成436、三个Go分片437/438/439、前端440）全部成功；16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及三个实际前端类型检查/构建通过。精确生成补丁零字节。普通测试均无测试文件；不代表业务验收或部署。

本域本批完整函数2项通过源码检查。完整服务、快照/恢复、业务验收按逐项台账继续未完成；不把部分装配记完整。未本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 集成检查（2026-10-17）

品牌、页脚、横幅和配置历史已合入 main，GitLab pipeline 42 把本轮函数记为 verified。changedSections 已含 branding、footer、banner。完整启动仍 blocked。下面是合入前的模块记录。

# 站点配置与初始化当前进度

## 品牌、页脚、横幅与配置历史（2026-10-17，implemented）

- 分支 `codex/site-config-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`，源码 `37097fce696c8d0093bd58476f385d142ab536d8`。独占 `modules/site-config/`。未跑 GitLab CI，未编译，不记 verified，accepted=0。
- 完成 UpdateBranding、UpdateFooter、UpdateBanner、ReadConfigHistory、RestoreConfigRevision，以及闭环 ValidateAnnouncementBody、WriteAnnouncementRevision、CommitAnnouncementHead。原回执、配置 CAS、历史与 outbox 仍走既有 settings 提交；品牌 pin 复用首页 `home_reference_decisions` 与失败封存，来源为 `HashBytes("site-config.branding.v1|"+siteId)`。
- 仍失败关闭：合同未写明的资源引用、favicon 超出 image 前缀与 kind=image、恢复时媒体未就绪或公告正文缺失、空 pin 被媒体拒绝、清横幅、无 head 的当前横幅，以及公告 JSONL/head 已写而 SQL 未提交的崩溃窗口。恢复不获取新 pin，不解析文档链接。完整 BuildApplication 继续拒绝启动。
- 新文件 `internal/branding_service.go`、`internal/footer_service.go`、`internal/banner_service.go`、`internal/config_history.go`、`internal/announcement_validator.go` 待根登记。OpenAPI 的 changedSections 仍待 `make gen`。无新迁移、无新测试。

## 第四轮首页权威组（2026-10-15，已交付）

- 源码提交 `aa96f429e0eecc45cf6a6c51773a8b1892bde648`；合同-only原提交 `fa2d1b7b738c25dbd38daebc4149a8c9a2181c67` 已由根集成。分支codex/site-config-home-r1，基线de21a2bcb5e1a8b25e41ec84ba3734fc163b5ab1，仅拥有modules/site-config/；主线cherry-pick SHA由根统一映射。
- 完成UpdateHome、ReadPublicHome、ValidateHomeLinks、ReadHomeReferenceDecision与RecoverHomeReferences：固定候选/完整refs/pin进入既有COMMITTED/CAS/原始回执/outbox链。终止直接Release完整source及nullable pinId，媒体回源明确终局并sealed，无pin亦封存；终止不重Acquire，COMMITTED绝不release，确认只用持久pin。
- 公开hero保留全部按钮，明确目标/锚点/语言/AI与依赖不可用；当前文档安全根相对URL/发布引用按请求语言消费，固定可信地址构造QA链接，不暴露私有hero target。公共NewWriteContext/RunRecoveryLoop及后台Drain生命周期复用根实现，构造只核本地journal/决定。
- 最终VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14模块readonly build/vet/gofmt及完整make check通过（858文件/1109函数/379操作/2256类型），无回传冲突；明确纳入全部Home Go、0005及最后正revision/后台登记修正。完整函数/文件按范围verified，BuildApplication及全量handler/RelayOutbox/恢复回执仍blocked；不称行为验收或联调通过。
- 新实际文件为internal/home_service.go、internal/home_validator.go、internal/migrations/active/0005_home_reference_decisions.sql，根统一登记。公共依赖9ff93ae71e8f262ea0fc057aa3f6eea71ce52f22及后续冻结的media封存/公共SSE，由根合成；所有生成产物来自VPS。
- 无本机Go/gofmt/gen/vet/类型检查/测试，无新测试/mock/fixture/样例/临时业务验证程序，无服务、真实接口/模型、部署或push。本轮已冻结，后包等待集成调度。

以下为历史记录，旧停止指令已被本轮持续授权撤销；逐项状态以progress.json为准。

## 第二轮最终集成与停止范围

- 一般设置权威读写、校验、对象原始流及HTTP核心通过；Home等及完整启动仍未完成。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


## 持续实施第二轮（2026-10-15）

- 基线 `468986fc34e1c90b6b5ca575539fdd2b9748cd06`，分支 `codex/site-config-settings-r1`，仅拥有 `modules/site-config/`；已交付核心函数通过最终统一 VPS 门槛和复核，记 verified（仅对应检查范围）。
- 已写一般设置的 ReadSiteConfig/UpdateSiteConfig、纯 ValidateSiteConfig/NormalizeBaseURL、ReadConfigObject 原始对象流；复用既有文件 IO、COMMITTED/CAS/回执与平台 outbox，不复制公共身份/签名机制。
- 一般设置候选材料只供未完成操作恢复，保存首次 updatedAt；正式读取与成功重试均读不可变文件。新增 0004 只追加迁移保存候选材料并约束对象 ID/站点修订唯一。
- NormalizeBaseURL 依赖根提供的 lifecycle.NormalizePublicURL（统一 IDNA/URL/显式 loopback HTTP 能力）；公共 HTTP 工厂冻结后再接线，不自造入口机制。
- ReadConfigObject 限当前真实已提交 settings；返回进程内 Reader，HTTP 直接消费关闭并输出绑定元数据，不发送 bodyHandle。
- Home 组保留下一紧密组：按钮禁用/解析 URL/AI 可用性协议及 media pin 崩溃续办尚未收口；完整 HTTP、RelayOutbox、恢复回执与快照/屏障继续未完成。
- 已写真实 HTTP 协议适配：config 读写/原始对象流、初始化状态/三用例、runtime 读/迁移/activation 与健康/就绪；统一工厂建立身份/epoch/期限/schema，并使用平台 identity 权威授权回调。缺 Home/品牌/公告/历史/快照/屏障等真实 handler 时工厂拒绝启动，完整 BuildApplication 仍 blocked。
- VPS 合成快照（315a3ff+公共26138c1）docs/gen及14模块readonly build/vet/gofmt通过；check余4项均非本域。随后按复核补维护边界：updateSiteConfig仅旧成功只读重放可到达，新/未完成操作在任何候选IO前CheckWriteEpoch；该追加修复已被最终VPS复跑覆盖（见下条最终证据）。
- 最终证据：隔离 VPS make docs/gen、14模块readonly build/vet/gofmt及完整 make check 全通过（855文件/1012函数/374操作/2211类型），包含维护边界修复；原先“等待VPS/非本域检查失败”为过程历史，已由此最终结果覆盖。
- 本轮源码提交 948a834、39ddb52、315a3ff、9a21f1b；依赖根公共26138c1及后续统一修复，交付只含本域，真实主线SHA由集成会话映射。
- 按用户最新指令，本轮收口后停止自动实施，不开展第三轮；本域不push，集成会话统一执行本轮已获授权的推送。Home等剩余项保持planned/blocked，不标全产品完成。
- 本轮未在本机格式化、生成、编译、vet、lint、类型检查或测试；未新增测试/样例/fixture/临时验证程序，未运行服务、真实接口、模型、部署或 push。

---


最新统一集成事实（2026-10-15）：初始化与settings权威提交、恢复和文件IO通过；配置编辑、完整HTTP/后台仍未完成。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：a0d6adf、0e8cc9f、c2d734b，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


更新：2026-10-15。阶段：R1-02 实施中；本轮初始化完成与 settings 权威提交及配置 IO 边界修复已实现，经隔离 VPS 全仓编译/生成/完整结构检查与集成复核，已交付函数记 verified（仅对应检查范围）。业务验收与部署未开始。

- 本轮源码提交 `5b6bc4e39fd72457d302031e0f369cece55ae1c5`，配置 IO 修复 `53156c7d74fb065b1355250ab916d03518c49d0b`，格式/生成与 vet 收口 `6396715d91a56bb6a12cb4ddd151b4117ff27520`、`8613254c72d63ddf30ce3710cf222d21a7738181`；尚未 push。
- 基线 `325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，分支 `codex/site-config-initialization-complete-r1`，独占 `modules/site-config/`。
- 已写 InitializeSite/ResumeInitialization/AbortInitialization：唯一超管会话与 site 作用域、稳定业务摘要、原始结果回执、不可反悔决定、blocked 后幂等补完；不调用 AI、identity owner 准备或真实接口。
- 最终检查：隔离 VPS `make docs/gen` 成功；14 个 Go 模块 `make build`（readonly build/vet/gofmt，含本域）及完整 `make check` 均通过（850 文件/943 函数/372 操作/2186 类型）。此前 IO 错误包装 vet 失败已修复并经本轮复跑通过。
- 逐函数复核追加：配置 IO 使用 os.Root 目录能力逐段 Lstat，拒绝链接/特殊文件，正规文件打开前后身份核对，正文与 journal/head 的 JSON 上限 16 MiB；该追加修复已通过本轮 VPS 门槛。
- 已写 WriteConfigRevision/CommitConfigHead/RecoverConfigJournal：schemaVersion=1 权威文件、不可变 PREPARED/COMMITTED、排他 Link、head Rename、逐级 fsync、SQLite 写锁/CAS、崩溃回放、孤儿隔离、SQL 与文件水位核对。
- `0003_config_commit.sql` 只追加 config_operations；配置水位/回执和平台 outbox 同事务，不复制公共 outbox 表或转发机制。RelayOutbox 循环仍待 HTTP/后台装配工作包。
- 可信公开地址由公共 ServiceConfig 边界校验后 ConfigureInitialization 注入，缺失地址初始化失败关闭；不从请求 Host 推导。启动恢复只根据明确 COMMITTED 决定，旧准备决定缺身份回执明确失败关闭。
- 未完成：完整 OpenAPI 业务 x-handler、其他设置用例、RelayOutbox 循环、恢复换目录回执与快照/屏障装配。BuildApplication 的本组接线不等于完整模块装配完成。

本轮未在本机执行编译、格式化、lint、类型检查、测试或生成校验；未新增测试、样例、fixture 或临时验证程序；未运行服务、真实接口、模型、联调、镜像、部署或 push。
