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

公共候选从 `main=a32f3f4c4876f94ce798826e9dee0b170bbb612a` 独立建立于 `codex/activation-verify-20261020`，已选入公共源 `379f0977277cc2a03f1bde49b5d0f5a6cede4c08`、site-config 源码 `1075bcb8824838874e6a3ed775778f4194fa6969` 和元数据 `e72dd7c15dec4932885c964e83833dcc09fca9bf`。范围含 `ActivationReceiptClaims`、`normalizeActivationReceiptClaims`、`decodeCanonicalReceiptJSON`、`VerifyActivationReceipt` 及 RegisterActivation 可信接线；`recordedAt` 在载荷摘要前统一为 UTC `RFC3339Nano`。

`decodeCanonicalReceiptJSON` 已登记为实际参考助手，并在合同中登记 `VerifyActivationReceipt → normalizeActivationReceiptClaims/decodeCanonicalReceiptJSON/CanonicalJSON`、助手自身的规范解码→`CanonicalJSON` 依赖边。`SignActivationReceipt` 与 `ObserveTx/SealTx/ResumeTx` 仍只留在 e85 WIP，不纳入本候选。合成候选待通过既有 GitLab CI；不推送 GitHub 或 main。

首次合成候选 SHA `b5b6a63f1e74d86aeafe8f72523c861a7a2245b5` 已上传 GitLab 隔离分支并触发 pipeline 170；生成 job 681 在 `make docs` 渲染 site-config 新合同时报 `KeyError: constraints`，三 Go 分片、实际前端和 foundation `make check` 均未启动，未产生可应用的 `generated.patch`。缺口归 site-config 代理修正：`RuntimeStore.activationVerifierAnchors` 的两个输出以及 `verifyActivationReceipt` 的三个输入和一个输出需补合同约束字段。当前无函数/文件转为implemented或verified。

site-config 已提交并选入三项元数据修正：`c56268fbd646cc71e6e829cecdf2d5d6059ef3f5`、`aaa35cbd95cd77e83620f1823dff1f67cf2deed3`、`a935a9284e32657f92fc20b5715166e832777bda`；源码逻辑仍为`1075bcb8824838874e6a3ed775778f4194fa6969`，最终合同摘要为`b3a1647f9ad5fa168bfba5ffeb8701d779d9f683146436c1996ef8407adc91eb`。约束扫描已清零，候选待以新精确SHA重跑完整CI。

pipeline 171 的生成、content-media、ai、实际前端均成功；foundation 的 build/vet/gofmt 与既有普通测试成功，`make check` 发现 site-config 四个新增函数未登记 `TrustAnchors` 类型。已按已授权元数据范围补 platform `TrustAnchors` 跨模块引用，提交 `737fe2d7`；未改变源码逻辑，待以新精确SHA重跑。

pipeline 172 对精确 SHA `1e19a7184500d8956695496aee03ddb243c0b91e` 已全绿：生成/格式、content-media、ai、前端、foundation（含完整 `make check`）均成功；Runner 回传 `generated.patch` 31001 字节，已应用到本地候选，等待以零生成漂移 SHA复跑。

最终候选 `eb96455b2366ac8511c471e7955de41b5ffdab97` 已通过 pipeline173：生成/格式696、foundation697、content-media698、ai699、前端700全部success，`generated.patch`为0字节，远端隔离分支与本地SHA一致。按本轮源码单元范围可复核的8个函数为：`platform.normalizeActivationReceiptClaims`、`platform.decodeCanonicalReceiptJSON`、`platform.VerifyActivationReceipt`、`site-config.ConfigureActivationVerifier`、`site-config.RuntimeStore.ConfigureActivationVerifier`、`site-config.RuntimeStore.activationVerifierAnchors`、`site-config.RegisterActivation`、`site-config.verifyActivationReceipt`；`site-config.BuildApplication`、wire及完整启动继续blocked。当前progress复算为2421函数：verified1611、implemented41、未完成769，verified+implemented=1652（68.2%）。

VerifyGlobalSeal 的 `coordinateBarrier` observe 方案仍待主代理复核：step=observe 只读取 prepare/drain/seal 已持久的 barrier、epoch 和协调事实，不在纯读分支写观察或回执；没有既存证明时明确不可用。14 域服务仅可在既有服务认证下读取当前 barrier/epoch，其他变更步骤仍由 data-protection 加超管授权；当前不改 API、不作为首单元 CI 前置。
