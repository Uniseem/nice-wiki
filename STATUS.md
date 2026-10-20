# public-site 独立 metadataRevision 缓存键 CI（2026-10-21）

从 B1 green 记录 `ddaab834` 独立接入 public-site clean `82db36cf`（源码 `0c25307d`），集成候选 `a6d5da6ba43356887c2083415c5dfb1ac5337ec1` 的 GitLab pipeline196（IID153）生成 job811、foundation812、content-media813、其余Go814、前端815全部成功；job811 `generated.patch` 为0字节，SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。本单元只覆盖 metadataRevision/v2缓存键纯函数，真实调用点、事件消费、media多来源proof、整条OG仍 blocked；content/media合同未合入。

# 2026-10-21 正式交付冻结

本次正式交付以源码/合同 GitLab pipeline196 五绿零漂移候选 `a6d5da6ba43356887c2083415c5dfb1ac5337ec1` 为代码基线，只追加纯根记录 `c2c2f19c9b550d7e9f382c2327434a20a8adb52c` 和本次 push-log/状态收尾。没有合入 `39af2b7a`、`4e5a0fd6` 或其他未 CI 的 content shareImage、metadata-source、public-site metadata-event、media manifest WIP；`28c38e5b` 根 proof-set 审查树继续独立保留。

本次保留的 CI 证据为 pipeline196（IID153）：job811/812/813/814/815 全部成功，生成补丁为0字节，SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。这只证明该精确源码候选的 GitLab 生成、三 Go 分片、前端和完整检查，不提升业务 accepted、完整 OG、BuildApplication、VerifyGlobalSeal 或协调 writer。

隔离保留：content metadata-source clean `b11ad73a` 未 CI，新增路径待 framework；public-site metadata-event `b76d684c` 未 CI且模块交接仍有未提交修改；media manifest `0dba8612` 为未 CI合同草案。它们均不进入本次 main 双推。

# B1 content→media 顺序集成（2026-10-21）

以 traffic green 记录基线 `b44e13143094b1832508a43595c95e8ebd84b5d2` 建立独立 B1 分支，按 content `27344e0f8dc8b35258699ce53fea769c654b404d`、media `151858e1791f4094ae92a0b880c5040a9e959dfc` 顺序合入。合入后的源码/合同候选 `cb72d741d9d6192757b3a9eee7a40f78e3c4d9a0` 触发 GitLab pipeline194（IID151）：生成 job801、foundation job802、content-media/traffic job803、其余Go job804、前端 job805 全部成功。

job801 原始 `generated.patch` 为 137347 字节，SHA-256 `691215cff5b59b1c4fb798aa3ee70d4ae63fd533b960a3bd3bce7d2606581183`；已原样应用，内容为 jobs v2 订阅生成登记、media 生成客户端/OpenAPI及 worker 格式化。应用后的最终候选 `41d5c43ff1498bc82cfe479ebcecb390cda9cf67` 触发 pipeline195（IID152），生成 job806、foundation807、content-media/traffic808、其余Go809、前端810全部成功；job806 `generated.patch` 为0字节，SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。结构摘要为 15 modules/2 shared、1059 files、2534 functions、408 operations、2609 types；`shareImage` 非空仍失败关闭，BuildApplication、全域 VerifyGlobalSeal、协调 writer、accepted 继续 blocked。

# 当前状态（2026-10-20，用户恢复R1实施）

主代理已审核通过并冻结 [docs/plan.md](docs/plan.md)；覆盖 15 个业务域、`libs/platform`、UI、任务与事件、保护与恢复、部署源码、R2 和最终发布交接。用户已恢复 R1 源码实施，并改为主代理只拆解、指挥、协调与复核，多个子代理负责实际修改、集成和 CI；每模块独立 worktree，公共文件由唯一集成子代理维护。本轮仍不授权 R2、真实业务接口/模型调用、服务运行、镜像构建或部署。

## 当前结构/合同 CI 候选（GitLab 隔离分支；不推 GitHub/main）

### GitLab pipeline 182/183：结构候选 CI 纠偏

精确结构候选 `5e40528eaeee22ac7fcd58597c395b62653b2a52` 已普通快进推至隔离分支 `codex/ci-20261020-r1-9b359c0e`，pipeline 182 未通过：生成 job 741、content-media job 743、AI job 744、前端 job 745 成功；foundation job 742 失败于 `modules/site-config/internal/app/wire.go:257` 的 `platform.SnapshotSwapEvidence` 未定义，未把该失败树记为源码通过。该失败不涉及网络、权限或 Runner 阻塞。

独立 CI 窄修仅把该参数改为实际定义包 `lifecycle.SnapshotSwapEvidence`，形成 `90dc3577e02bd93c37ef2579391ecfbd28879b0f`，随后普通快进推至同一隔离分支。pipeline 183 全部成功：生成 job 746、foundation job 747、content-media job 748、AI job 749、前端 job 750；`generated.patch` 为 0 字节，SHA-256 为 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。该证据覆盖 GitLab 生成、三个 Go 分片的 readonly build/vet/gofmt 与完整 `make check`、前端检查；不等于业务验收或 accepted。原 182 失败记录保留，不以 183 结果抹除。

当前 GitLab 隔离分支 `codex/ci-20261020-r1-9b359c0e` 的远端 SHA 为 `90dc3577e02bd93c37ef2579391ecfbd28879b0f`。本地合同修正父提交为 `014db6309aad644b556f2b6db1dcad582389abe3`：`site-config.registerActivationAfterSwap` 的签名和输入明确使用实际 Go 包 `lifecycle.SnapshotSwapEvidence`，类型 `$ref` 仍保留平台合同来源；本次状态收尾后形成的新精确候选待推送并按同一隔离分支复检，不能把 90dc 的 pipeline 183 证据外推到新 SHA。用户已授权自主推进隔离 CI；不推 GitHub/main，不标 accepted，不把任何域的 BuildApplication、全域 VerifyGlobalSeal 或未接协调 writer 标为完成。

独立分支 `codex/r1-structure-fix-20261020` 从原暂停候选 `6f25dd57386081b5903bc6c02f448ea769466618` 建立；历史 checker、platform、identity、site-config 收口记录保持不变。公共 checker 修复提交为 `a8699993a87e75dcb6de5eab9c0bba4e40443bee`、`d91078748d78960a4e50266a1a9ba655144021fb`、`49c42f70963decf660163a5ac27e993201a3f147`；platform公共合同/类型/台账和文档已顺序收口，最新公共阶段提交为 `559de687b4ca849c962e1089354f9fd03c48e65c`。

identity 已顺序合入 clean `69f1d6cd52355e2b21e2e299fe35615ba4b19604`（源码/合同 `2f52be9c`），并登记本域生成 `*gen.SiteConfigClient` 类型；site-config 最终结构合同 clean HEAD 为 `b046e22c6e763c49f301e2c30494b3e0ca939e16`，不再引用过期的 `73e7676e` 作为最新代理 HEAD。两域代理交付源尚无独立模块 CI 证据；pipeline 183 只证明当时的整树候选，BuildApplication、VerifyGlobalSeal及未接协调 writer仍保持 blocked/in_progress，不标 implemented/verified。

本树 `python3 scripts/check_docs.py --summary` 已通过，摘要为 15 modules、2 shared、1060 files、2484 functions、408 operations、2603 types；JSON、`git diff --check`、`make privacy` 通过。未运行 Go、pnpm 或完整 `make check`，不能把结构零错误当作源码/业务验收。

本次集成已将本地 `main=6204dfc94510f28752fe8da81df9f3a4d3ce781d` 安全合入当前集成树，保留 `e85dde0583938bb4207bc3a1117e62549d21018c` WIP记录；main 已包含 pipeline173/174 验证的 `VerifyActivationReceipt` 与 site-config `RegisterActivation`。当前公共候选继续保留 e85 的 `SignActivationReceipt`、`ObserveTx/SealTx/ResumeTx`，并新增只读 `ReadBarrierStatus`、`MigrateRestoredDatabase`及非jobs不等待最终broker的分阶段封口合同；这些新增/修改源码尚无本次候选GitLab SHA，仍按blocked/in_progress记录。

identity 已按顺序接入 clean HEAD `f54792d29768a3c223de2b429da1628376aa84cd`（源码包 `5962be0953753b7fc88c990362da60c20260926d`），恢复迁移调用为实际 `outbox.MigrateRestoredDatabase`；jobs源码交付 `879a41d430b89af5a0b88dd8d86d24f8f25763e3`、clean HEAD `7bc01a2917302324b84e3f594c5f249b7fc22c4e`，仍等待 jobs `step=observe` service-only授权分支。identity/site-config的 `VerifyGlobalSeal` 缺协调只读证据时必须失败关闭。用户未跟踪的 `kangaroo-down-under.html` 未触碰。

site-config 已按顺序接入 clean HEAD `d51bd3df0efe0c2156b1e0e443a82c6986a43c64`（源码修复 `c3c7c10c11320ebff45c994f8458e54604a75343`）；模块改动范围仅 `modules/site-config/`，保留main验签接线。其恢复迁移合同明确调用 `outbox.MigrateRestoredDatabase`，不调用根 `platform` 影子函数；尚无本域单独 CI 证据，整树 pipeline183 不把静态审查或本域WIP升为verified。

traffic-guard 已接入 clean HEAD `85fc4639b34d0d321c59e59f3b6169319f7dfcba`（源码单元 `0c0e2e17e00adf2bb159de97c31f7bf8b58892b0`），仅新增快照归档读取单元并登记框架路径；其 `BuildApplication` 尚未装配真实 `SnapshotManager`，相关文件/函数继续 blocked，未标完成。

GitLab 隔离 CI 候选 `9b359c0e70ffb8857189d77dfa0adaccb00200cd` 已推送到 `codex/ci-20261020-r1-9b359c0e`，pipeline 179 生成/前端/AI 成功，foundation 与 content-media 因公共/模块编译缺口失败；`generated.patch` 非零（803264字节）。原SHA保持不变；本地已记录窄修但未推送新SHA，等待下一次授权。

精确修正候选 `284737bee5c95632ddcce4ce8fb034d72e7f7f96` 已快进推送同一GitLab分支，pipeline 180：生成/格式、content-media、AI、前端成功；foundation job 732 失败于 `identity/internal/app/wire.go:199` 调用的生成 `SiteConfigClient` 缺少 `RegisterActivation`。job 731 的 `generated.patch` 为792字节，SHA-256 `f15530fd247b4c195af91d1e40691a36453e0f6e8a5d3e8949647b2d35de3211`，仅含identity barrier源码格式生成差异；该原始补丁已在本地按哈希核对后原样应用，未推送第三个SHA。

静态枚举确认该缺边由唯一业务跨服务依赖误写造成：identity `restoreIdentityAfterSwap` 合同已在本地修为 operationId `site-config:registerActivation`，identity 文档已重生；生成 client 留待后续 CI `make gen/docs`，本机未运行 make gen，未推送新SHA。

基于本地依赖修正提交 `7fbd7918535a3c941084a7f2b48d85d8f6d306af` 的候选已应用job731原始生成补丁；`make docs` 未产生额外差异，全仓JSON解析、`git diff --check` 与 `make privacy` 均通过。当前只保留identity barrier的预期格式改动及本次状态记录，未运行Go、测试、pnpm或`make check`，候选仍待主代理复核，不标记为verified/accepted。

精确候选 `2de5fa21d40c07ae80ae69cc7c8fb96a5f8f0a3d` 已普通快进推至 GitLab 隔离分支 `codex/ci-20261020-r1-9b359c0e`，pipeline 181 结果为 failed：生成 job 736、content-media job 738、AI job 739、前端 job 740 成功；foundation job 737 在 `modules/site-config/internal/resource_reference_reader.go:88,91,95` 使用不存在的 `historyRevision.RevisionID`（生成类型字段为 `RevisionId`）时失败，`make check` 尚未执行。随后本地窄修提交 `9e234cba216beb4fcb027c8a71a301d61bfa7100` 仅修正这三处字段名；job 736 的 `generated.patch` 为829字节，SHA-256 `68a7a99c30d58deebf61e5003301cee95b2d9d54925ca16ee0232158d78bf024`，仅含生成的 `modules/identity/internal/gen/clients.gen.go` RegisterActivation client，已在本地候选提交 `89df86a0c0396d5a496b2afbc7b353fdf4aea040` 中原样应用。当前候选未推送，等待主代理审批。

此前“结构摘要检查失败203项”只属于旧 `9b359c0e` job726 follow-up 树上直接运行的 `check_docs.py --summary`，不对应 `2de5fa21`；不能作为本轮 `make check` 结果外推。当前需另行授权后再处理 foundation 编译缺口。

已从job726重新下载并应用原始`generated.patch`（SHA-256 `e56c0714c2f483045d9864e0e8e206145c1557d96bbf8686d8f49df951a93a9e`）；40个非冲突artifact路径已写入本地，两个已窄修源码路径保留本地修复。JSON、`git diff --check`、`make privacy`通过；`check_docs.py --summary`仍因203项合同类型/函数可达性/交接摘要漂移失败。未运行Go、测试、pnpm或make check，未推送新SHA。

历史远端推送基线为 `b986443d0dddf5482f7c3430e3dc7c8b99a24232`，对应 pipeline 162 全绿且生成补丁 0；当前本地 `origin/main` 与 `gitlab/main` 跟踪 ref 均为 `6204dfc94510f28752fe8da81df9f3a4d3ce781d`。本地计划收尾基线为 `1857ef4bb1a936d3e8c7a8b091a38ee2e2331ef8`，对应 pipeline 163 全绿且生成补丁 0；当前集成提交和模块候选尚未推送。

四路 WIP 原样保留，不 reset、不强推：

- `codex/closeout-20261020-integration`：`e85dde0583938bb4207bc3a1117e62549d21018c`。pipeline 166 生成、三个 Go 分片 build/vet/gofmt、既有普通测试和前端通过，生成补丁 0；完整 check 仍因 5 个公共函数没有真实消费者依赖边而失败，不交付。
- `codex/closeout-20261020-identity`：HEAD `5466c47d22249c224d57009a329786f1d019fa80`。新增后台参考函数尚未完整登记，实际 caller 未接齐，不交付。
- `codex/closeout-20261020-site-config`：HEAD `6d83888b39ebbd3a624f58d820274e7c6f3072c7`，源码链为 `42b30e41`、`b5895d88`。relay 仍不可达，不交付。
- `codex/closeout-20261020-jobs`：HEAD `76325f0d024563daec80037f42e021112de39fad`，源码链为 `34d3fa5a`、`0644f1d1`。真实后台、readiness 与保护来源仍不完整，且没有完整 CI 证据，不交付。

三域 worktree 均干净且代理已关闭，提交只保留在本地 WIP 分支；没有 push、进入 main 或新增完整 CI，完成数不增加。

全仓函数台账仍为 1645/2414（68.1%，已含 platform）：verified 1604、implemented 41；未完成状态 planned 267、in_progress 8、blocked 494，accepted 0。15 域为 2179 项，其中未完成状态 675；platform 为 235 项，其中 blocked 94。两者合计未完成 769，不再另加 platform；41 个 implemented 另行冻结核验。15 域完整 `BuildApplication` 均未闭合。

文件台账仍有 669 项未到 verified（blocked 274、implemented 108、in_progress 13、planned 274；verified 391）。同域 Dockerfile/Makefile/manifest 共 45 项中，verified 3、implemented 29、blocked 3、planned 10；`deploy/` 的 5 个总编排文件仍全部 planned。

冻结覆盖口径：

- 15 域非 verified 716 项（未完成 675、implemented 41）与平台 blocked 94 项全部归入 19 个交付包及 3 个无重复计数的联合闭合门；全仓非 verified 合计 810，769 只计未完成状态。
- 41 个 implemented 函数及 108 个 implemented 文件在最终源码冻结核验；baseline verified 只在改动、依赖变化或实际证据冲突时重开。
- `docs/frontend.md` 的完整后台页面表和公开问答/语言建议岛属于完成范围；identity、site-config、media 的账号、站点和资源库页面由 Portal 组合实现，不能漏项。
- 15 个服务独立镜像/manifest、Compose/Caddy/服务地址/公钥/secret 引用和自动重启属于 R1-D05 与最终冻结证据；退出码 75 仅用于 ADR0003 数据恢复切换/换回后的重启，代码镜像升级回退另按兼容镜像与升级前备份点。实际已核对 portal/public-site/qa 缺 Dockerfile，public-site/qa 缺 deploy manifest；台账状态不能代替文件存在。源码阶段不生成或保存真实私钥，不构建或部署真实实例。
- 无 AI 初始化按 bootstrap claim、site-config 自身 runtime、首次恢复通道和启动依赖表核对，禁止形成“初始化等待服务 ready、服务 ready 等待 initialized”的环。

R1-FZ 通过只表示源码就绪。R2 真实业务/接口证据、真实模型调用和部署分别等待用户当次授权；R2 通过后才可进入指定环境的发布与回滚交接。

本轮实际交付和最终回执见 [2026-10-20-0001](docs/push-log/2026-10-20-0001.md) 及 [push-log 索引](docs/push-log/INDEX.md)。源码工作保持冻结，不自动开始下一包；R2、真实模型/接口、部署未执行。
