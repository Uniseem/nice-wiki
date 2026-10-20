# 2026-10-20 R1 收口与 R2/发布总计划

## 计划状态

本计划已于 2026-10-20 经主代理审核通过并冻结，是当前唯一总计划，覆盖 R1 全部 15 个业务域、`libs/platform`、部署编排、UI、任务与事件、保护/恢复、R2 和最终发布交接。`main=a32f3f4c`代表此前文档收尾基线；用户于 2026-10-20 随后恢复 R1 源码实施，当前候选仍须严格留在本计划冻结工作包和既有恢复合同内，R1-00 不新增产品或第二套公共协议。

此前文档收尾基线不包含 identity、site-config、jobs 或公共平台 WIP，也不把部分源码检查当成 R1 完成。四路旧 WIP 仍保留在各自分支，当前计数仍为 1645/2414 函数达到源码检查门槛，accepted 为 0；没有真实业务验收、模型调用或部署。

截至 `main=a32f3f4c` 的收尾事实如下；源码恢复候选另按当前交接记录：

- 基线为 `b986443d0dddf5482f7c3430e3dc7c8b99a24232`，GitLab pipeline 162 已通过且生成补丁为 0。
- 本地计划收尾基线为 `1857ef4bb1a936d3e8c7a8b091a38ee2e2331ef8`，pipeline 163 的生成、三个 Go 分片、前端和完整 `make check` 全绿且生成补丁为 0。
- 全仓函数台账共 2414 项：verified 1604、implemented 41，尚未完成状态的 planned 267、in_progress 8、blocked 494，另有 accepted 0。15 域占 2179 项，其中 verified 1463、implemented 41、非 verified 716；未完成状态为 675。
- platform 占 235 项，verified 141、blocked 94。769 是 15 域未完成状态 675 加 platform blocked 94，不再另加 94；41 个 implemented 另行在最终冻结逐项核证据。15 个完整 BuildApplication 均未闭合。
- 15 域文件台账有 638 项未到 verified；platform 另为 26 项，deploy 为 5 项。
- 同域 Dockerfile/Makefile/manifest 共 45 项，状态为 verified 3、implemented 29、blocked 3、planned 10；`deploy/` 的 5 个总编排文件全部为 planned。
- 固定接口基线为 408 个 HTTP operation、377 个 path template、18 个 jobTypes、68 个 backgroundTasks、73 条事件记录（50 publish、23 consume，映射到 15 个唯一消费 handler）。这些只是覆盖清单；完成目标仍按业务职责、页面和端到端链判断。
- `main` 此前仅交付冻结计划与收尾记录；未完成的源码 WIP 不进入 `main`。当前仅推进已授权的源码候选，不启动真实接口、模型、镜像、运行服务或部署。
- 当前首个公共候选从 `main` 独立建立，范围仅为激活回执 Claims、统一 `recordedAt` 为 UTC `RFC3339Nano` 的参考助手、严格规范 JSON 解码助手和 `VerifyActivationReceipt`；site-config 需从 `main` 独立接通 `RegisterActivation` 后再合成 CI 候选。旧 e85 公共 WIP 的 `SignActivationReceipt` 与 outbox 事务变体继续保留，未计入完成数。

计划依据按优先级为：根 `AGENTS.md`、`STATUS.md`、[全局需求](requirements.md)、[产品保留矩阵](product-preservation.md)、[界面装配](frontend.md)、已确认 ADR、各域机器合同、各域 STATUS/progress/handoff。历史只通过 [push-log](push-log/) 追溯，不在本计划继续复制流水账。

## 总目标与完成层级

- **L1 R1 源码就绪**：按最终同步后的实际合同，15 域及平台的全部职责均已实现并 verified，包括完整启动装配、任务/事件、UI、保护和恢复回调、独立镜像/manifest 与总编排。不得残留尚未实现、未接线或未检查事项，即使已有阻塞说明也不满足；当前数字仅为基线，不为追求原 ID 数制造冗余函数。只表示源码检查完成。
- **L2 R2 业务验收**：按用户给出的有界批次授权（场景、数据、费用上限、环境和模型/供应方），在隔离预发布环境构建/运行候选镜像并调用真实接口、模型或恢复链；同一授权批次内不逐次重复请求许可。证据脱敏保存，accepted 按场景更新，不得用测试套件、fixture 或 mock 替代。
- **L3 生产发布与部署交接**：全部必需 R2 通过并在相应用户授权批次内，构建固定 15 个服务镜像与 Caddy 版本，部署指定生产环境，核验基础链、准备真实回滚点并交接。当前不执行真实部署。

## 冻结覆盖与防跑偏规则

1. **业务清单优先**：G01-G24、B01-B19 和 `docs/frontend.md` 全部页面是完成尺度；函数百分比仅是附录。不得删除回收站、历史回滚、预览、编辑器、多语言、SEO、llms、媒体视频、问答岛、备份镜像等功能来换取快速完成。
2. **包覆盖全部**：每个工作包在基线读取本域全部非 verified 文件/函数；不得只挑主函数。blocked 必须逐项分类为“占位/缺真实分支”“已有逻辑待接线”或“真实外部依赖未就绪”；文件存在不能证明函数完成。存在明确外部依赖时只能形成“部分交付/待依赖”，不能称为包完成；包完成要求范围内全部项 verified。
3. **实现项复核**：41 个 implemented 函数和 108 个 implemented 文件中的相关项进入所属包复核；baseline verified 只在本包改动、依赖变化或实际证据冲突时重开，不做无差别重审。
4. **约束先于代码**：HTTP处理、事件、任务、启动入口按 implementation.json 精确合同实现。缺约束步骤或外部字段时先由集成会话修唯一合同；参考助手可按 ADR0002 随实现调整并在交付台账同步。
5. **不新增产品/架构**：新增需求、新服务、新协议、新存储或范围缩减均须用户明确同意。包内只允许为修复已登记需求而做细化，记录“原工作包内、解除哪个已登记阻塞”。
6. **单一决策口**：出现真实横切歧义时，集成会话登记一个具体决策并暂停受影响包；不得各域自造兼容层。共享协议只改一次，再分发给模块代理。每个模块包同时覆盖 implementation.json 的所有 jobTypes/backgroundTasks 与 events.json 的全部生产、订阅和 handler；全部生产事务走 outbox→jobs→inbox 本地提交后 ack，不另立遗漏清单。
7. **所有权**：每个模块包独占一个 worktree；最多 3 个模块包并行。集成工作树独占根文档、`contracts/`、`libs/platform/`、`deploy/`、生成器、Git index、顺序合并与 GitLab CI。
8. **检查门槛**：Go 变更只能由自部署 GitLab CI 三分片执行 readonly build/vet/gofmt，并运行前端检查和完整 `make check`。生成目录、go.sum、pnpm lock 只由冻结工具生成；GitLab 不可用即 blocked，不退回本机或 SSH。
9. **默认不写测试**：不新增测试文件、脚本、fixture、mock、样例或临时验证程序。真实接口、真实模型、行为验收和部署分别需要当次明确授权。
10. **完成包只按缺陷重开**：只有现有需求冲突、CI 证据失效或集成缺陷才重开；不能因新想法、美化、性能或“顺手重构”重开。

## 当前剩余量

| 包 ID | 域 | 函数 | 非 verified（含 implemented） | implemented | 非 verified 文件 |
|---|---:|---:|---:|---:|---:|
| R1-F01 | identity | 107 | 16 | 6 | 16 |
| R1-F02 | site-config | 92 | 14 | 5 | 31 |
| R1-F03 | jobs | 120 | 34 | 5 | 35 |
| R1-F04 | traffic-guard | 92 | 17 | 4 | 24 |
| R1-D01 | content | 266 | 41 | 0 | 43 |
| R1-D02 | media | 184 | 29 | 6 | 27 |
| R1-D03 | public-site | 86 | 24 | 0 | 34 |
| R1-D04 | portal | 66 | 57 | 8 | 32 |
| R1-A01 | model-gateway | 274 | 109 | 1 | 59 |
| R1-A02 | memory | 115 | 26 | 4 | 31 |
| R1-A03 | localization | 163 | 36 | 1 | 49 |
| R1-A04 | knowledge | 221 | 166 | 0 | 103 |
| R1-A05 | qa | 121 | 66 | 0 | 42 |
| R1-O01 | analytics | 147 | 24 | 1 | 58 |
| R1-P02 | data-protection | 125 | 57 | 0 | 54 |
| R1-00 | libs/platform | 235 | 94 | 0 | 26 |
| R1-D05 | deploy 总编排 | - | - | - | 5 个计划文件 |

15 域合计 716 个非 verified 函数，其中“未完成状态”675（planned 267、in_progress 8、blocked 400）、implemented 41；platform 另有 blocked 94。总计非 verified 810，769 不含随后单独核证据的 implemented。所有数以当前 progress.json 为基线，UI、任务、事件和保护回调不另建统计体系。

每个包只使用三种交付状态：

- **接口就绪**：合同、类型、operationId/event/job 名冻结，可允许相邻包并行编码；不代表任一函数完成。
- **源码单元交付**：本包文件/函数通过精确 SHA 的 CI，但依赖方真实实现未接入时只叫“部分交付/待依赖”，不得称包完成。
- **源码链闭合**：本包与所有真实依赖已接入，范围内全部记录 verified，相关跨包批次门槛才可关闭。R2 的实际运行证据仍属后续层级。

`R1-G-A`、`R1-G-B`、`R1-G-C` 是跨包源码链门槛，不重复计算函数或文件；它们只判定相邻包的真实版本化调用是否闭合。

## 固定工作包

| 阶段 | ID | 范围 | 开发前置（接口冻结即可） | 最终关门依赖 | 交付/结束条件 |
|---|---|---|---|---|---|
| A | R1-00 | 公共接口冻结与 platform | 本计划 | R1-FZ 最终同门关闭 | 合同/生成器冻结；所需平台源码单元完成 CI；消费接口清单确定 |
| A | R1-F01 | identity | R1-00；登录/guard 接口 | R1-G-A、R1-G-C、R1-P01 | 登录/恢复/outbox/保护/Build |
| A | R1-F02 | site-config | R1-00；bootstrap/runtime 接口 | R1-G-A、R1-G-C、R1-P01 | 无 AI 初始化、runtime/资源引用/保护 |
| A | R1-F03 | jobs | R1-00；task/event 合同 | R1-G-C、R1-P01 | 队列/事件/调度/租约/owner恢复/屏障协调 |
| A | R1-F04 | traffic-guard | R1-00；identity 登录准入接口 | R1-G-A、R1-G-C、R1-P01 | 准入、挑战、封禁、登录保护、后台/UI |
| A | R1-G-A | 登录/bootstrap 联合门 | R1-F01–R1-F04 源码单元交付 | 无额外实现包 | 真实 guard/identity/site-config/jobs 登录与初始化链 |
| B | R1-D01 | content | R1-00、R1-F01–R1-F04；media 引用接口 | R1-G-B、R1-P01 | 文档生命周期、编辑器、回收站、历史、预览、保护 |
| B | R1-D02 | media | R1-00、R1-F01–R1-F04；content 事件接口 | R1-G-B、R1-P01 | 上传、变体、视频、字节流、引用、清理 |
| B | R1-D03 | public-site | content/media 接口；localization/QA 岛接口 | R1-G-B、R1-G-C、R1-P01 | 全部既定公开与预览 operations |
| B | R1-D04 | portal | 各域生成客户端与 auth 合同 | R1-G-B、R1-G-C、R1-P01 | 壳、代理、认证、SSE、组合页面 |
| B | R1-G-B | 基础文档联合门 | R1-D01–R1-D04 源码单元交付 | 无额外实现包 | 建站、编辑、上传、预览、发布、公开读取、撤回链 |
| B | R1-D05 | deploy 源码 | 15 服务 manifest/配置路径 | R1-G-C、R1-P01 | 5 个总文件及 45 个服务产物 |
| C | R1-A01 | model-gateway | R1-00、R1-F01–R1-F04 | R1-G-C、R1-P01 | 目录/渠道/执行/费用/账本 |
| C | R1-A02 | memory | R1-A01、R1-F03 接口 | R1-G-C、R1-P01 | 条目/映射/预算/助手/概述 |
| C | R1-A03 | localization | R1-A01、R1-A02、R1-D01 接口 | R1-G-C、R1-P01 | 语言计划/翻译/术语/UI 字符串/建议 |
| C | R1-A04 | knowledge | R1-A01、R1-A02、R1-D01、R1-D02 接口 | R1-G-C、R1-P01 | 提取/索引/检索/重排/执行/恢复/UI |
| C | R1-A05 | qa | R1-F04、R1-A01–R1-A04、R1-D03 接口 | R1-G-C、R1-P01 | 问答/流式/缓存/反馈/压缩/调试/公开岛 |
| C | R1-O01 | analytics | R1-F01–R1-F03、R1-A01、R1-D03、R1-D04 接口；QA 事件接口 | R1-G-C、R1-P01 | 访问/审计/费用/聚合/报表 |
| C | R1-G-C | AI/非 DP 后台联合门 | R1-A01–R1-A05、R1-O01、R1-D04 源码单元交付 | 无额外实现包 | 模型、问答、翻译、记忆、统计及非 DP 页面链；不含 backup/setup-restore |
| D | R1-P01 | 保护公共闭包 | R1-00 接口、A/B/C 源码单元、R1-P02 本域保护/控制/协调器源码单元 | 全部 14 域真实回调 | 审查 P02 本域回调并完成 14 域参与、13 非 jobs 封口、DP 对另外 13 域导出 |
| D | R1-P02 | data-protection | R1-00 及 14 域冻结保护接口；本域回调先于 P01 交付 | R1-P01 | 备份/恢复/便携包/镜像/保留/初始化恢复 |
| E | R1-FZ | 最终源码冻结 | R1-P01、R1-P02、R1-D05 | 无 | 15×6 入口、全 operation、任务/事件、UI、部署检查 |

## A：基础域与公共装配

### R1-00 公共接口冻结与 platform

- 范围：`libs/platform` 全部 94 个未 verified 函数及 26 个未 verified 文件；根 `contracts/`、错误表、生成器和必要公共类型。覆盖 serviceauth/opctx、请求边界、outbox relay、inbox consumer、worker lease/scope、runtime/write context、TaskGroup、modelclient 只读观察、snapshot/barrier/archive/swap、cleanup scope 与 secret 封装。
- 先固定 `PrepareBarrier → DrainBarrier → ObserveBarrier → SealBarrier → ResumeDrain` 协议、14 个有状态参与域（portal 不参与）、jobs broker 水位、cleanupScope、真实 user/service scope、snapshot V1/V2 和 model-result 读取形状；旧 Acquire/Release 名称只做兼容映射，不能出现两套协议。
- 交付证据：合同与生成物先更新；平台单模块及全树 GitLab CI 通过；每个平台函数能由真实入口或登记任务到达；所有消费方引用的名称/字段一致。
- 停止边界：若某一域需要新行为而非合同补齐，暂停该域，登记一条决策；平台不得为各域复制私有逻辑。R1-00 通过源码检查后只因实际缺陷重开，消费者接线在对应包完成。

### 首批并行

- **R1-F01 identity**：`cmd/identity/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`internal/outbox_repository.go`、`internal/barrier_service.go`、`internal/snapshot_service.go`、`internal/snapshot_archive.go`，并核对本域 6 个 implemented 函数。接通超管重置观察、RelayOutbox、快照/屏障、完整 HTTP 与恢复激活。Portal 负责账号页；identity 负责接口和权限。
- **R1-F02 site-config**：`internal/resource_reference_reader.go`、outbox、barrier、snapshot、archive、wire/main。闭合无 AI 初始化、runtime 权威、站点设置、资源引用、历史/首页/公告恢复和 activation。Portal 负责站点设置页。
- **R1-F03 jobs**：`internal/queue.go`、`query.go`、`event_store.go`、`barrier.go`、`barrier_coordinator.go`、`app/owner_recovery.go`、retention/snapshot/import、transport、web。每个任务领取、事件转发/订阅、调度、重试、取消、进度和 owner 恢复都装真实 scope；jobs 最后封口。
- 三域必须联合核对 bootstrap：identity 注册只看自身 bootstrap claim；site-config 未初始化时仍可提供 runtime；jobs 不把“已初始化”当 broker 启动条件。

### R1-F04 traffic-guard

- 范围：Build/transport、events、snapshot/import/barrier、web 4 项及全部剩余准入函数。覆盖 Turnstile 优先/PoW 备用、频率与并发、匿名主体、登录保护、AI 禁止、封禁、租约、事件和后台任务。
- 完成条件：identity/portal 的登录链与 qa/model 的准入链均使用原根；无 token 不得绕过；后台管理页可操作但服务端仍即时核权。
- 回退：guard 不可用时 AI 和高风险入口明确不可用，基础文档初始化/编辑/公开阅读不得被错误阻断。

## B：完整文档站使用链

### R1-D01 content

- 范围：`metadata_service.go`、`history_service.go`、`trash_service.go`、`preview_service.go`、`object_reader.go`、outbox/barrier/snapshot/transport 与全部 editor web。覆盖稳定块 ID、树/排序/slug、共享草稿、锁、发布/撤回、301/410、回收站、差异/历史回滚、署名、预览授权、分享图/引用和模型入口。
- UI 命中：`docs/frontend.md` 文档管理与编辑页整行；编辑器 14 个未完成 UI 函数全部属于本包。`InvokeModel` 只用于发布后的既定派生任务，草稿保存绝不调用。
- 完成条件：所有 document/block 操作有真实 HTTP、幂等/CAS/租约/取消；媒体 pin 与分享图协议冻结后接真实 media，不猜 metadata pin；保护回调接 R1-P01 前的平台接口。

### R1-D02 media

- 范围：字节 GET/HEAD、图片变体、四平台视频元数据/embed/重定向、delivery manifest、事件订阅、outbox、worker、snapshot/barrier、Build 和上传完成链。四平台行为按 [media requirements](../modules/media/docs/requirements.md) M05-R03 与 [产品保留矩阵](product-preservation.md) B05；覆盖内容寻址、分段 Range、SVG/动画、引用保护和人工清理。
- 冻结协议：先解决 `content.resources.changed` 与 `ApplyReferenceEventInput` 字段不一致；发布前 acquire/confirm 与空引用路径必须同源。Portal 负责资源库组合页。
- 完成条件：图片、附件、画板、视频在编辑器和公开页均走真实版本化 API；未知/未引用证明失败关闭，不自动删除。

### R1-D03 public-site

- 范围：`routing.go`、renderer、home/page/ask/error/markdown/llms handlers、cache/invalidation/prewarm、events、worker、barrier/import、Build、`PreviewPanel.vue`/`PublicSettingsPanel.vue`。覆盖 contracts/openapi.json 的全部既定公开 operations，包括 robots、根重定向、语言 sitemap/index/shard；同时实现完整 HTML、多语言 cookie/根路径、导航/上下篇、SEO/hreflang/分享图、复制 Markdown、llms和受保护预览。
- 关闭前必须修完 `LoadPublishedBundle` 富文本译文映射和 `ResolvePublicMedia` 尺寸/alt/变体缺口；不得降低为只渲染纯文本。
- 完成条件：撤回/不可见/媒体失效在同一权威水位失效缓存；私有预览不进入公开缓存。

### R1-D04 portal

- 范围：`LoadBootstrap`、health、preferences、route registry、proxy/auth、events、audit、transport/wire 与全部 16 个 web 函数。覆盖 cookie/CSRF、单次 refresh、权限即时变化、SSE、错误与离线/缺能力状态。
- 固定页面清单来自 `docs/frontend.md`：初始化/登录、后台壳/个人设置、仪表盘、内容、资源库、多语言、模型、记忆、问答、知识、防滥用、任务、统计、站点、账号、备份共 16 类页面。R1-G-C 只关闭 AI 与基础文档相关页面；backup/setup-restore 页面依赖 R1-P02，在 R1-FZ 关闭。identity/site-config/media 无独立前端时由本包 Portal 组合页消费其 API，不能漏账号/站点/资源库。
- 完成条件：`EmitAccessAudit` 的请求字段映射先冻结；统计匿名 beacon 不泄露正文/原始 IP；代理只向已登记服务发送签名请求。

### R1-D05 deploy 源码

- 范围：15 个模块的 Dockerfile/Makefile/manifest，以及 `deploy/docker-compose.yml`、Caddyfile、services、trust-anchors、`.env.example`。核对端口、内存、卷、健康/能力/版本、自动重启、只读公钥和 secret 引用；退出码 75 只用于 ADR0003 恢复切换/换回后的重启，不是升级失败回退机制。
- 实际文件缺口已核对：`portal`、`public-site`、`qa` 缺 Dockerfile，`public-site`、`qa` 缺 deploy manifest；其中台账状态不能代替文件存在和内容审查，R1-D05 必须补齐并复验全部 45 项服务产物。
- 证据：静态引用、生成与完整 CI；镜像定义与 manifest 一致。默认不真实构建/拉取镜像、不生成私钥、不启动服务。
- 完成条件：Compose 中 15 服务与 Caddy 地址/依赖/数据卷完整；备份卷只挂 data-protection；secret 只挂引用不含值。

### UI 工程交付边界

- UI 固定为 12 个工程：当前已有的 analytics、data-protection、knowledge、localization、model-gateway、public-site 6 个，加上 portal、content、jobs、memory、qa、traffic-guard 6 个待补齐工程。范围包括 package/config、入口、构建、Portal 同源挂载、页面/API client、权限/错误/空态/费用确认和卸载清理，不只是函数 ID。
- identity、site-config、media 没有独立 UI：账号、站点设置、资源库这 3 类 Portal 组合页面消费其版本化 API；不得因无独立工程漏页面。
- 公开站问答浮窗/独立页与语言建议岛由 qa/localization 提供 UI，public-site 提供挂载位置；content 编辑器、public-site 预览和管理页面按 `docs/frontend.md` 全表逐项验收源码接线。

## C：AI、检索、问答与统计

### R1-A01 model-gateway

- 范围：`admin/`、billing、catalog、invocation/execute/dispatch、runtime、security/credential+headers、provider response、outbox、transport/wire、snapshot/barrier、UI。六类协议与五档有序备用按 [产品保留矩阵](product-preservation.md) B09、[模型网关要求](../modules/model-gateway/docs/requirements.md) 与 [AI 上下文规格](ai-context-and-cache.md)；覆盖目录刷新、任务覆盖、双超时/并发、预留/结算/未知费用、取消和用量 V2。
- 完成条件：每个实际 attempt 有唯一费用事实；原 invocation lookup/get 只读复用；核心 7 个启动/保护函数闭合；无渠道不影响基础站点启动。
- 拦截：分析/成本统计消费方未就绪时不得伪造 0 或 complete coverage。

### R1-A02 memory

- 范围：事件、助手、概述、dispatcher、prune、保护协议、worker、Build、UI。覆盖人工条目/映射/预算/上下文清单，助手只提议，批准后再应用。
- 完成条件：术语派发、结果回传、提案批准、概述人工保护与原任务重投全有真实事件/任务；所有模型调用经 R1-A01，恢复不重付。

### R1-A03 localization

- 范围：正文/UI/cache-answer 三类任务、事件、术语、建议、UI、worker、保护和 Build。20 字 TM、1000 token 顺序批与上下文规则按 [翻译 ADR](product-baseline/adr/0047-translation-pipeline.md)、[本地化要求](../modules/localization/docs/requirements.md) M10-R08 和 [产品保留矩阵](product-preservation.md) B14；保留人工保护、只重译一次与默认关闭反思。
- 先冻结 Portal background costClass 与 cache-answer `translationVersionHash` 来源，再接真实执行；估价/批准后收费，草稿不触发。
- 完成条件：语言关闭/撤回/源修订失效拒绝旧译文，缺译回退有可见提示；非强制跳转规则保持。

### R1-A04 knowledge

- 范围：config、build、source/chunk/extract、index/search、model、events、runtime/worker/recovery、admin、transport/wire、UI、保护和 Build，共 166 项。
- 固定依赖：只处理发布正文与授权媒体；嵌入空间五字段绑定供应商/模型/版本/维度/预处理；PDF/图片/视频描述复用已付费结果；检索必须返回可重新核验的块 ID/范围/文本 hash。
- 完成条件：build/extract/rerank/query embedding 的真实任务、取消、恢复与 paid refs 全闭合；索引代数切换与清理不删除活动/未知依赖；UI 覆盖配置、构建、取消和诊断。

### R1-A05 qa

- 范围：conversation/turn、answer、context/cache、history compaction、debug、policy、events、runtime/recovery/worker、transport/SSE、UI 和保护，共 66 项。覆盖浮窗/独立页、当前页提示、子问题、检索/重排、引用、停止、恢复、清空、反馈、精选、校准和调用隔离。
- 完成条件：guard 原根和许可、knowledge 检索、memory 上下文、localization cache translation、model attempt 全部可追踪；无来源失效、负反馈、撤回和语义不完整不能命中旧缓存；付费截止只约束原调用，过期观察不重付。

### R1-O01 analytics

- 范围：beacon/access/audit、usage V2、hourly/daily/usage aggregate、reconciliation、queries、outbox/consumer/worker、exports、barrier/snapshot、transport/wire、UI 和 Build。
- 完成条件：人类/AI 爬虫分计、日盐匿名、原始 IP/正文不入库；费用 changes/observations 同水位且 unknown/held 不填零；查询/导出/审计页有真实接口。

## D：保护、恢复与镜像

### R1-P01 保护公共闭包

- 14 个有状态参与域为 identity、site-config、jobs、traffic-guard、content、media、public-site、model-gateway、memory、localization、knowledge、qa、analytics、data-protection；portal 只保留独立维护门，不冒充有状态 owner。
- 每个参与域交付真实 `ObserveWork/SealWork/ResumeWork`、HTTP/job/event/background/file IO scope、capture 库存和 activation/restore 回调；13 个非 jobs 域先 seal，jobs 最后封口。data-protection 自身状态本地捕获，另对另外 13 个域使用导出端口。
- 停止边界：任一 owner 回调缺失、水位未知、旧 epoch 活跃写入未分类或回执不能复验，整阶段 blocked；禁止用空成功、计数 0 或简化导出放行。

### R1-P02 data-protection

- 范围：backup/manifest/snapshot client、restore create/stage/switch/validate/continue/abort/cleanup、便携导入导出、初始化恢复、mirror/validation/publication projection、remote config/sync、schedule/retention、decision journal、events/worker/transport/wire/UI。
- 镜像必须覆盖允许的业务历史投影，不能把整份数据库/私钥塞入 data git；完整包导出与 data git 是不同的产物。
- 完成条件：可选密钥、跨域引用、短委托、20GiB/100000 上限、失败保留旧代、退出 75 换目录、revision watermark、全部激活回执、便携包与保留清理均闭合。
- 无 AI 初始化恢复走同一暂存/切换流程，凭证由私有 setup 通道取得，不要求模型或已建 superadmin 会话。

### 集成先解决的合同缺口

- **costClass/短委托**：Portal 为 `localization.estimateLanguagePlan`、`approveLanguagePlan` 及 knowledge 同类后台操作装配可信 `background` 类别；浏览器不得自报。data-protection 把原持久超管批准转为受限短委托，服务 `ExportDomainSnapshot` 不得伪造用户 root/site/epoch，`readSnapshotArchive` 仍按原授权核 viewer。
- **审计字段**：冻结 `portal.EmitAccessAudit`、`analytics.AppendAudit` 与 `analytics.RedactAuditMetadata` 的一一映射；`AppendAuditRequestBody` 的 auditId/action/resourceId/resultCode/metadata 必须由 `RedactedAuditRequest` 和 `RequestOutcome` 的真实白名单字段派生，不能把请求正文、cookie、原始 IP 或可重放 proof 写入审计。
- **媒体 manifest/分享图 pin**：统一 `content.resources.changed` 与 `media.ApplyReferenceEventInput` 的来源字段、`media.BuildDeliveryManifest` 五字段 `AssetVersionRef`、content 发布分享图 metadata pin 与 public-site 消费。空引用、变更/撤回、历史 revision 和私有预览必须有唯一语义。
- **全域封口/控制授权**：平台 `PrepareBarrier/DrainBarrier/ObserveBarrier/SealBarrier/ResumeDrain`、`SnapshotWorkState`、`SnapshotSealProof`、`cleanupScope` 与 jobs 最后封口一次冻结；覆盖 14 个有状态参与域的真实 SQL 水位、HTTP/job/event/file scope、activation 回执，并覆盖 DP 对另外 13 域的导出端口。上述只修既有机制，不新增协调服务或第二套屏障。

## 批次与阶段门槛

1. **批次 A1**：R1-00 接口冻结且所需 platform 源码单元交付后，identity、site-config、jobs 三模块先交付源码单元；随后 traffic-guard 源码单元。R1-00 的最终集成在 R1-FZ 同门关门，不作为 A1 开发前置。`R1-G-A` 只关闭登录、注册与 bootstrap 链；guard 的 AI/QA 路径在 `R1-G-C` 才关闭。批次 B 可在 A 源码单元和接口冻结后开始，不等待未来消费者。
2. **批次 B1**：先冻结 content/media 引用、分享图和事件载荷，content 与 media 可并行；随后 public-site 和 portal 源码单元，完成基础 `R1-G-B`。deploy 可先写总编排，但 45 个服务产物必须等 C 阶段所有 manifest 到位后才关闭。
3. **批次 C1**：model-gateway 单独先行；通过后 memory、localization、knowledge 中最多三项并行；随后 qa，最后 analytics 和 Portal 非 DP 组合页面及 deploy 产物。执行 `R1-G-C` 关闭真实模型、问答、翻译、记忆、统计、公开岛和非 DP 页面链；backup/setup-restore 页面留到 R1-FZ。
4. **R1-P01/R1-P02**：按冻结接口交错交付。data-protection 模块代理先交付本域保护、控制回执与协调器源码单元；R1-P01 集成会话只审查接线并审计全部 14 域回调和真实水位。只有 14 域闭包通过后才关闭 R1-P02 的全备份/恢复链，不越权并行修改 data-protection。
5. **R1-FZ**：冻结全树，只修复阻断项。

每次合并只处理一个模块包，不能与另一个模块共用 Git index。生成目录、锁文件、根合同和公共平台冲突由集成会话处理；模块代理不得跨域修复。每个阶段未达到门槛时，下一阶段不得用宽泛阻塞掩盖。

## 无 AI 初始化核对点

- identity 以自身不可逆 bootstrap claim 决定注册开放，不以 site-config initialized=true 为前提；已有账号不得重开注册。
- site-config 从自身 control/active 读取 runtime 和 uninitialized 状态，初始化不要求模型目录、渠道、embedding 或 AI 服务可用。
- portal 的 setup/login/portable restore 路由可在无超管会话时进入；只调用 OpenAPI 允许的固定匿名/服务操作，其他后台页面保持受保护。
- data-protection 首次 setup 导入可在未初始化站点完成；校验目标备份与 secret 选择后走暂存/重启换入，不要求已存在新站超管会话。
- jobs、traffic-guard 及其他服务启动时遇空/初始代次必须区分“fresh”与“损坏/缺依赖”；不得把未初始化当 started/ready，也不得形成“初始化需要服务 ready、服务 ready 又需要 initialized”的环。
- 15 个进程启动均不得探测模型；无渠道时模型/索引/翻译/问答报告缺能力，但初始化、编辑、预览、发布和公开基础文档继续可用。
- 集成会话维护启动依赖表：初始化前可达、初始化后可达、仅维护/恢复态可达三类；任何循环或隐式已初始化前置都属于 R1-P01 及首批 R1-F01–R1-F03 的阻塞。

## 变更管理

- 包内细化必须写明需求 ID（G/B/页面）、原合同处、解除的具体阻塞、兼容影响和调用方；兼容的内部参考助手可在交付时同步合同。
- 已授权需求范围内发现合同缺字段、事件载荷不一致或实现步骤不清时，集成会话直接先修唯一合同来源、审阅并重新生成所有消费者，不新增用户审批。
- 新需求先进入“待决”，不自动扩张；范围缩减与删除既有功能同等需要用户同意。一个阻塞只形成一个具体决策，解决后关闭。
- 结构外新增、删除、移动或职责变化，以及新产品、新外部依赖或对外范围改变，按根 AGENTS 取得明确授权；内部参考助手按 ADR0002 随实现调整并在交付时同步。
- 参考助手合理合并或改名时，同步合同去向和职责覆盖；不得为追逐旧 ID 保留冗余包装，也不得靠删台账刷完成。当前不向机器状态 schema 新增 waived/excluded 等旁路。
- 已 verified 的 baseline 证据继续有效；只有本包改动、依赖形状变化或证据与实现不一致才重开。

## R1-FZ 最终冻结

- 以冻结业务清单核对：G01-G24、B01-B19、16 类后台页面、公开问答岛、语言建议岛、15 个服务完整装配、全部 jobTypes/backgroundTasks/event handlers、保护回调和部署产物。
- 按最终同步后的实际合同，所有业务职责、启动入口、handler、后台、事件、UI、保护与部署产物已实现并 verified；不得残留尚未实现、未接线或未检查事项。即使阻塞已有说明也不满足 R1 完成。
- 当前 716 个域内非 verified、platform 94 个 blocked、41 个 implemented 函数及 108 个 implemented 文件只是基线清单，不是必须保留原 ID 的目标；合理合并或改名必须同步合同和职责覆盖。
- 15 域逐域核对 main、BuildApplication、Serve、Shutdown、Health、Ready 六个入口；HTTP 全量 408 operations 由生成路由和真实 handler 一一覆盖。Build 不再因源码缺 handler/后台/scope 固定返回 `DEPENDENCY_UNAVAILABLE`，运行时缺必要配置仍显式 503/ready=false。
- 12 个 UI 工程、16 类后台页面、3 类 Portal 组合页面、公开问答岛和语言建议岛全部完成源码与挂载；45 个服务部署产物和 5 个总编排文件均有归属。
- 18 个 jobTypes、68 个 backgroundTasks、50 个事件生产记录、23 个消费记录及 15 个唯一消费 handler 全部可执行；后台启停、恢复、永续循环退出和注销均接入生命周期。
- 最终精确 SHA 的 GitLab 三分片、所有实际前端、完整 `make check`、生成/重复生成零漂移通过；隐私审阅通过，未跟踪用户文件未动。
- 此阶段仍 accepted=0、未部署。源码检查通过不等于站点可业务验收。

## R2 隔离预发布验收（当前不执行）

R2 按下列固定场景逐项取证，不新增测试套件、fixture 或 mock。用户授权以有界批次给出，每个批次需列场景 ID、测试数据范围、模型/供应方与费用上限、环境；授权批次内不逐次请求许可。只读本地或沙箱接口所需权限仍按当次批次执行。

| ID | 场景 | 通过证据 |
|---|---|---|
| R2-01 | 无 AI 全新初始化、登录、强改密、重启恢复 | 无模型配置；setup→唯一超管→基准语言→重启仍可登录 |
| R2-02 | 账号/权限/会话撤销/服务器重置 | 十项权限、即时撤销、旧会话失效、重置文件一次性消费 |
| R2-03 | 文档编辑到公开 | 树/锁/冲突/稳定块 ID/上传/预览/发布/撤回/301/410/历史回滚 |
| R2-04 | 多语言与媒体 | 增量译文、人工保护、回退提示、Markdown/llms/sitemap、视频/变体/Range |
| R2-05 | 任务与事件 | 去重、租约、取消、重试、死信、调度、SSE、旧 epoch 不重放 |
| R2-06 | 风控与模型费用 | 挑战/封禁、原根许可、五级备用、取消、unknown/held、账本对账 |
| R2-07 | 知识与问答 | 发布源索引、混合检索、引用核验、缓存失效、反馈/精选、流式恢复 |
| R2-08 | 记忆与翻译 | 任务映射、预算、助手批准、正文/UI/cache-answer 翻译与陈旧拒写 |
| R2-09 | 统计与隐私 | PV/UV、爬虫分计、日盐、审计链、费用聚合、导出不含敏感字段 |
| R2-10 | 备份、镜像、恢复 | 全域 safe/seal、完整包/便携包、可选密钥、失败换回、重启回执、保留与 data git |
| R2-11 | 隔离预发布升级与回退演练 | 15 候选镜像/manifest、Compose/Caddy、secret/公钥、兼容性检查、升级前备份、镜像回退；确认不影响 ADR0003 数据恢复换回 |

证据只保存 operationId、脱敏状态/错误码、时间与修订、费用/invocation/job ID、manifest/hash、截图或可视记录。禁止保存密码、密钥、原始 IP、提问/文档敏感正文和不必要的访客数据。未授权的真实模型、恢复或部署场景保持 pending。

## L3 生产发布、回退与交接（待当次授权）

1. 全部必需 R2 场景通过后，才允许在该生产环境从冻结 SHA 构建/固定 15 个服务镜像与 Caddy 版本，并固定 deploy manifest、服务地址、内部网络、数据/备份卷、公钥和 secret 引用。
2. 各域 `dist` 按 manifest 同源打包并挂载到 portal/public-site 的登记入口；上线前取得生产备份与真实回滚点，先核健康/能力/版本、初始化入口保护和维护状态。真实模型渠道按用户单独批准的配置范围启用。
3. 执行有界上线基础链并留存脱敏证据。代码/镜像升级回退使用上一兼容镜像和升级前备份/迁移兼容证据；ADR0003 数据恢复换回只按持久 phase、全域决定和回执执行退出码 75，退出码 75 不是通用部署失败回滚按钮。
4. 完成后交付版本/镜像 SHA、Caddy 版本、UI 打包/挂载清单、部署参数位置（不含 secret 值）、健康状态、备份/恢复与回滚步骤、已知限制和唯一下一维护动作。

## 终止标准

- 包可停止：范围内全部记录 verified 且有精确 CI 证据；无顺手改动；handoff 只保留实际结果与一个下一步。
- 阶段可停止：按对应开发前置完成源码单元，并通过该阶段明确列出的联合门；共享合同唯一，没有把后续真实消费者误当作早期开发前置，也没有用缺失 handler/scope/假 ready 放行。
- R1 可停止：R1-FZ 全部门槛通过，源码就绪；不得宣称业务验收或部署。
- R2 可停止：获授权批次范围内场景全部有脱敏证据并明确 accepted；未授权场景保持 pending，不得批量推定。
- 项目可结束：用户授权的全部范围完成，R1/R2 和相应部署回退证据及运维说明可核；未授权的后续 R2、模型或部署保持 pending，不宣称全部结束。
