# 2026-10-20 R1恢复候选

用户已授权恢复 R1 源码实施；本目录记录候选状态，不自动推送到任一远端。

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

已确认非jobs不能等待封口后的最终broker：协调器必须先选择可最终复用的稳定零broker基础proof，13个非jobs使用同一份不含`nonJobsSeals/nonJobsProofHash`的proof执行本域封口；其`VerifyGlobalSeal`只读协调阶段、持久prepare/drain事实和`dataEpoch`，不再等第二次broker回执。jobs最后重新读取broker确认仍等于基础proof，再核13个真实激活回执和原`nonJobsProofHash`。所有缺证据、epoch/barrier不符、阶段倒退或未知状态都失败关闭；prepare/drain/seal/resume-drain仍由data-protection服务身份加超管委托驱动。

三模块代理的精确可调用合同如下：

- `identity`：在其真实`SnapshotConfig.VerifyGlobalSeal`装配中实现`func(context.Context, SnapshotSealProof, string) error`这一回调边界；非jobs分支只能使用既有服务身份调用jobs `POST /v1/barriers:coordinate` 的`step:"observe"`，请求复用`operationId/barrierId/mode/deadline`，不得带`sealProof`或`expectedSealHash`、用户委托或自报actor。只接受jobs返回的持久`barrierId/state/dataEpoch/revisionWatermark`与原操作一致，并把缺失/不可达映射为失败关闭。不得读取jobs数据库、伪造broker/回执或在observe分支写本域观察。identity已集成源码`5962be0953753b7fc88c990362da60c20260926d`、clean HEAD `f54792d29768a3c223de2b429da1628376aa84cd`；待jobs observe合同后再做源码CI，不把静态审查升为verified。

- `site-config`：复用同一`SnapshotConfig.VerifyGlobalSeal(ctx, proof, proofHash)`只读回调和上述service assertion；RegisterActivation仍只消费已验签的`ActivationReceipt`，不得把`ReadBarrierStatus`或当前runtime自报当13域封口证据。非jobs阶段不等待`nonJobsSeals`或最终broker；缺jobs持久observe证据即返回依赖不可用/完整性错误并保持维护。

- `jobs`：当前`coordinateBarrier`合同把整个operation绑定为data-protection+超管委托，故跨域observe暂阻塞。必须先在jobs OpenAPI/类型/实现合同中把同一路径的`step:"observe"`明确为service-only、caller为已登记有状态域且无userDelegation；prepare/drain/seal/resume-drain继续只接受data-protection+超管委托。`jobs.CoordinateBarrier(ctx, op, in)`的observe分支只读持久barrier/epoch/协调记录，不写stop claims、不发回执、不推进状态；最终jobs `VerifyGlobalSeal`再用同一base proof核13个真实回执和broker最终水位。未完成该合同拆分前，identity/site-config不得接线或伪造替代端口。

因此当前阻塞是jobs OpenAPI/实现授权分支缺失，而不是让各域自行实现协调。平台已登记`ReadBarrierStatus`、分阶段`SealBarrier`校验和恢复迁移窄接口，待该合同变更后再顺序吸收identity。
