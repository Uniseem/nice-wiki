# 2026-10-21 B1 content→media pipeline194

独立 B1 集成分支以 traffic green 记录基线 `b44e13143094b1832508a43595c95e8ebd84b5d2` 建立，按 content `27344e0f8dc8b35258699ce53fea769c654b404d`、media `151858e1791f4094ae92a0b880c5040a9e959dfc` 顺序合入。源码/合同候选 `cb72d741d9d6192757b3a9eee7a40f78e3c4d9a0` 的 GitLab pipeline194（IID151）生成801、foundation802、content-media/traffic803、其余Go804、前端805全绿。生成801原始补丁137347字节，SHA-256 `691215cff5b59b1c4fb798aa3ee70d4ae63fd533b960a3bd3bce7d2606581183`，已按 Runner 原件应用；当前新候选待零生成漂移复检。未推 GitHub/main，未将业务状态标 verified/accepted。

# 2026-10-20 R1恢复候选

用户已授权恢复 R1 源码实施；本目录记录候选状态，不自动推送到任一远端。

## 当前结构/合同 CI 候选（GitLab 隔离分支；不推 GitHub/main）

### pipeline 182/183：结构候选 CI 纠偏

`5e40528eaeee22ac7fcd58597c395b62653b2a52` 已推至 GitLab 隔离分支 `codex/ci-20261020-r1-9b359c0e`，pipeline 182：生成 job 741、content-media job 743、AI job 744、前端 job 745 成功；foundation job 742 失败，精确根因为 `modules/site-config/internal/app/wire.go:257` 引用了不存在的 `platform.SnapshotSwapEvidence`。该树未计源码通过，且没有网络、权限或 Runner 阻塞。

本地独立窄修将实际包限定改为 `lifecycle.SnapshotSwapEvidence`，候选 `90dc3577e02bd93c37ef2579391ecfbd28879b0f` 普通快进推送后触发 pipeline 183：生成 job 746、foundation job 747、content-media job 748、AI job 749、前端 job 750 全部 success。job 746 的 `generated.patch` 为 0 字节，SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`；pipeline 183 的 Go 分片和完整 `make check`均通过。该结果只证明 CI/结构门槛，不标业务 accepted/verified；pipeline 182 的 foundation 失败仍保留为失败证据。

当前远端隔离分支 `codex/ci-20261020-r1-9b359c0e` 为 `90dc3577e02bd93c37ef2579391ecfbd28879b0f`。本地合同修正父提交 `014db6309aad644b556f2b6db1dcad582389abe3` 已将 `site-config.registerActivationAfterSwap` 的签名、输入和生成说明限定为实际 Go 包 `lifecycle.SnapshotSwapEvidence`；`site-config/types.json` 仍准确引用 `libs/platform/contracts/types.json` 的公共 `$defs`。本次状态收尾后形成的新精确 SHA 尚未推送，需在合同变更后用同一隔离分支重跑 CI 并核对零生成漂移。用户已授权自主推进；保留历史失败，未把 pipeline183 扩展为新 SHA 的证据，不标 accepted/完整 Build。

结构修复分支 `codex/r1-structure-fix-20261020` 基于原暂停候选 `6f25dd57386081b5903bc6c02f448ea769466618`；旧失败树与历史记录保持不变。公共 checker、platform、identity、site-config 已顺序处理：identity clean `69f1d6cd52355e2b21e2e299fe35615ba4b19604`，site-config 最终结构合同 clean HEAD `b046e22c6e763c49f301e2c30494b3e0ca939e16`；不要再把 `73e7676e` 写作最新代理 HEAD。机器合同中的 platform ID 使用 `platform.RegisterWorkTable`/`platform.MigrateRestoredDatabase`，实际Go包仍为outbox。

当前本地结构树 `python3 scripts/check_docs.py --summary` 通过，统计为15 modules、2 shared、1060 files、2484 functions、408 operations、2603 types；`make docs`、JSON、`git diff --check`、`make privacy`通过。该本地结构树未运行Go、pnpm或完整`make check`；远端90dc的pipeline183已完成对应整树CI，但合同修正新SHA尚未推送，不能把90dc证据外推到新SHA，也不把本地结构或WIP记为implemented/verified。

集成候选分支为 `codex/resume-20261020-integration`，由 `codex/closeout-20261020-integration` 的 `e85dde0583938bb4207bc3a1117e62549d21018c` 合入 main `a32f3f4c4876f94ce798826e9dee0b170bbb612a` 纯文档基线形成。该基线保留原四路 WIP，不改旧提交。

候选复算确认 pipeline 166 缺真实消费者的 5 个公共函数是：

- `platform.SignActivationReceipt`
- `platform.VerifyActivationReceipt`
- `platform.Store.ObserveTx`
- `platform.Store.SealTx`
- `platform.Store.ResumeTx`

建议接线按既有恢复合同与事务边界分别交 identity、site-config、jobs 处理；接线通过精确 SHA CI 前不得改 implemented/verified。jobs 旧 WIP 新增 web 工程也纳入既有前端任务；lockfile 只能由 CI 中的 pnpm 生成。

## 当前最小公共验签候选

公共候选从 `main=a32f3f4c4876f94ce798826e9dee0b170bbb612a` 独立建立于 `codex/activation-verify-20261020`，已选入公共源 `379f0977277cc2a03f1bde49b5d0f5a6cede4c08`、site-config 源码 `1075bcb8824838874e6a3ed775778f4194fa6969` 和元数据 `e72dd7c15dec4932885c964e83833dcc09fca9bf`。范围含 `ActivationReceiptClaims`、`normalizeActivationReceiptClaims`、`decodeCanonicalReceiptJSON`、`VerifyActivationReceipt` 及 RegisterActivation 可信接线；`recordedAt` 在载荷摘要前统一为 UTC `RFC3339Nano`。首次合成候选 SHA `b5b6a63f1e74d86aeafe8f72523c861a7a2245b5` 的 pipeline 170 在 `make docs` 因 site-config 合同约束字段缺失失败，待 site-config 代理新 SHA。

`decodeCanonicalReceiptJSON` 已登记为实际参考助手，并在合同中登记 `VerifyActivationReceipt → normalizeActivationReceiptClaims/decodeCanonicalReceiptJSON/CanonicalJSON`、助手自身的规范解码→`CanonicalJSON` 依赖边。`SignActivationReceipt` 与 `ObserveTx/SealTx/ResumeTx` 仍只留在 e85 WIP，不纳入本首候选。待 site-config 从 main 独立接通最小 `RegisterActivation` 消费者后，才合成并上传既有 GitLab CI；不推送远端。

本次合入main后保留上述WIP并接回Sign/Tx公共源码；Sign与Verify现都先规范化UTC时间，且签发`key.KeyID`与验签header`kid`均要求合法Identifier。该严格kid校验是本候选相对pipeline173/174的新源码差异，Verify/Sign及本轮平台窄单元均待新的GitLab精确SHA，不把旧验证证据外推。

site-config 已顺序集成 clean HEAD `d51bd3df0efe0c2156b1e0e443a82c6986a43c64`、源码修复 `c3c7c10c11320ebff45c994f8458e54604a75343`；公告 `COMMITTED` 单条隔离修复保留main验签。其合同和实现统一使用 `outbox.MigrateRestoredDatabase`，本次模块源码尚待GitLab CI。

traffic-guard 已按本域静态审查结果接入 clean HEAD `85fc4639b34d0d321c59e59f3b6169319f7dfcba`、源码单元 `0c0e2e17e00adf2bb159de97c31f7bf8b58892b0`；新增 `internal/snapshot_archive.go` 已登记框架。因 `BuildApplication` 尚未装配真实 `SnapshotManager`，本模块新增/受影响项保持 blocked，未运行本机Go或CI。

## 全域封口 `step=observe` 合同决策

已确认非jobs不能等待封口后的最终broker：协调器必须先选择可最终复用的稳定零broker基础proof，13个非jobs使用同一份不含`nonJobsSeals/nonJobsProofHash`的proof执行本域封口；其`VerifyGlobalSeal`只读协调阶段、持久prepare/drain事实和`dataEpoch`，不再等第二次broker回执。jobs最后重新读取broker确认仍等于基础proof，再核13个持久`SnapshotSealReceipt{owner,sealProofHash}`和原`nonJobsProofHash`。`ActivationReceipt`只用于恢复换目录后的独立签名来源验签，不作为barrier封口回执。所有缺证据、epoch/barrier不符、阶段倒退或未知状态都失败关闭；prepare/drain/seal/resume-drain仍由data-protection服务身份加超管委托驱动。

三模块代理的精确可调用合同如下：

- `identity`：在其真实`SnapshotConfig.VerifyGlobalSeal`装配中实现`func(context.Context, SnapshotSealProof, string) error`这一回调边界；非jobs分支只能使用既有服务身份调用jobs `POST /v1/barriers:coordinate` 的`step:"observe"`，请求复用`operationId/barrierId/mode/deadline`，不得带`sealProof`或`expectedSealHash`、用户委托或自报actor。只接受jobs返回的持久`barrierId/state/dataEpoch/revisionWatermark`与原操作一致，并把缺失/不可达映射为失败关闭。不得读取jobs数据库、伪造broker/回执或在observe分支写本域观察。identity已集成源码`5962be0953753b7fc88c990362da60c20260926d`、clean HEAD `f54792d29768a3c223de2b429da1628376aa84cd`；待jobs observe合同后再做源码CI，不把静态审查升为verified。

- `site-config`：复用同一`SnapshotConfig.VerifyGlobalSeal(ctx, proof, proofHash)`只读回调和上述service assertion；RegisterActivation仍只消费已验签的`ActivationReceipt`，不得把`ReadBarrierStatus`或当前runtime自报当13域封口证据。非jobs阶段不等待`nonJobsSeals`或最终broker；缺jobs持久observe证据即返回依赖不可用/完整性错误并保持维护。

- `jobs`：当前`coordinateBarrier`合同把整个operation绑定为data-protection+超管委托，故跨域observe暂阻塞。必须先在jobs OpenAPI/类型/实现合同中把同一路径的`step:"observe"`明确为service-only、caller为已登记有状态域且无userDelegation；prepare/drain/seal/resume-drain继续只接受data-protection+超管委托。`jobs.CoordinateBarrier(ctx, op, in)`的observe分支只读持久barrier/epoch/协调记录，不写stop claims、不发barrier回执、不推进状态；最终jobs `VerifyGlobalSeal`再用同一base proof核13个持久`SnapshotSealReceipt`和broker最终水位。未完成该合同拆分前，identity/site-config不得接线或伪造替代端口。

因此当前阻塞是jobs OpenAPI/实现授权分支缺失，而不是让各域自行实现协调。平台已登记`ReadBarrierStatus`、分阶段`SealBarrier`校验和恢复迁移窄接口，待该合同变更后再顺序吸收identity。

## GitLab pipeline 179（精确候选 `9b359c0e70ffb8857189d77dfa0adaccb00200cd`）

隔离分支：`codex/ci-20261020-r1-9b359c0e`；仅推送GitLab，未推origin/GitHub/main。pipeline 179：生成/格式 job 726 成功，但 `generated.patch` 为 803264 字节（SHA-256 `e56c0714c2f483045d9864e0e8e206145c1557d96bbf8686d8f49df951a93a9e`），前端 job 730、AI 分片 job 729 成功。

失败精确记录：foundation job 727 在 `modules/identity` 报 `ReadBarrierStatus undefined`、broker 字段 `int` 到 `int64` 不兼容、`outbox.RegisterWorkTable undefined`；content-media job 728 在 `modules/traffic-guard/internal/snapshot_archive.go:47` 报 `ModuleSlug undefined`。pipeline 状态为 failed。原始SHA不修改、不重推。

本地窄修已写入工作树但尚未推送：补公共 `ReadBarrierStatus`、`outbox.RegisterWorkTable`，修identity四处类型转换，traffic-guard使用固定本域slug；后续CI需用户另行授权新SHA。本机未运行Go/build/vet/test/pnpm/make check。

job726原始`generated.patch`已重新下载并应用到本地follow-up；仅排除两个已窄修源码路径，其他artifact均由CI原patch写入，包含gofmt/gen/docs/路由生成，无lock或私有路径。应用后JSON与`git diff --check`通过，`make privacy`通过；结构摘要检查失败203项，具体为新增合同类型未登记、重复函数/未来文件、函数可达性与handoff摘要过期，未以本次9b候选名义修复或重推。

候选 `284737bee5c95632ddcce4ce8fb034d72e7f7f96` 的pipeline180结果：生成/格式、content-media、AI、前端成功；foundation job 732 失败，`modules/identity/internal/app/wire.go:199` 调用 `gen.SiteConfigClient.RegisterActivation`，但当前生成client未提供该方法。job731的`generated.patch`大小792字节，SHA-256 `f15530fd247b4c195af91d1e40691a36453e0f6e8a5d3e8949647b2d35de3211`，仅为identity barrier格式生成差异。该原始补丁已在本地按哈希核对后原样应用，未推送第三个SHA。

foundation缺边的合同根因已定位并本地窄修：identity `restoreIdentityAfterSwap` 的唯一跨服务依赖已由 `site-config.RegisterActivation` 改为版本化 operationId `site-config:registerActivation`；文档已由 `make docs` 重生，生成 client 留待后续CI `make gen/docs`。当前仍不推送新SHA。

基于本地依赖修正提交 `7fbd7918535a3c941084a7f2b48d85d8f6d306af` 的候选已应用job731原始生成补丁；`make docs` 未产生额外差异，全仓JSON解析、`git diff --check` 与 `make privacy` 均通过。当前只保留identity barrier的预期格式改动及本次状态记录，未运行Go、测试、pnpm或`make check`，候选仍待主代理复核，不标记为verified/accepted。

精确候选 `2de5fa21d40c07ae80ae69cc7c8fb96a5f8f0a3d` 已普通快进推至 GitLab 隔离分支 `codex/ci-20261020-r1-9b359c0e`，pipeline 181：生成 job736、content-media job738、AI job739、前端 job740成功；foundation job737失败于 `modules/site-config/internal/resource_reference_reader.go:88,91,95` 的 `historyRevision.RevisionID` 字段名错误，生成类型实际为 `RevisionId`，因此 `make check` 未执行。随后本地窄修提交 `9e234cba216beb4fcb027c8a71a301d61bfa7100` 仅修正这三处字段名；job736 `generated.patch` 为829字节，SHA-256 `68a7a99c30d58deebf61e5003301cee95b2d9d54925ca16ee0232158d78bf024`，仅含生成的 `modules/identity/internal/gen/clients.gen.go` RegisterActivation client，已在本地候选提交 `89df86a0c0396d5a496b2afbc7b353fdf4aea040` 中原样应用。当前候选未推送，等待主代理审批。

旧 `9b359c0e` follow-up 的“结构摘要检查失败203项”来自该树直接运行的 `check_docs.py --summary`，不代表当前 `2de5fa21` 的 `make check`；本轮 foundation 在编译阶段失败，不能外推该摘要。

## 下一项：B1资源引用事件合同冻结（仅设计）

`content.ResourceReferencesChangedPayload` 与 `media.ApplyReferenceEventInput` 尚无唯一确定映射：需先冻结 `referenceId` 来源、`draft/published` 到 `current` 的映射、`confirmed`、空集合、撤回及 `sourceRevisionId/sourceVersion` 语义，并明确 `AssetReference` 的 `kind`、期限和摘要绑定。完成该合同前不接消费订阅、不让content或media自行猜字段；本项不改当前CI候选。
