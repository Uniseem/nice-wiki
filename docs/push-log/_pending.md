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

公共候选从 `main=a32f3f4c4876f94ce798826e9dee0b170bbb612a` 独立建立于 `codex/activation-verify-20261020`，当前提交 `379f0977277cc2a03f1bde49b5d0f5a6cede4c08`（前置源码提交 `7137f9423274f73044fad6f17cb43ecffd4931ee`）。范围仅含 `ActivationReceiptClaims`、`normalizeActivationReceiptClaims`、`decodeCanonicalReceiptJSON` 和 `VerifyActivationReceipt`；`recordedAt` 在载荷摘要前统一为 UTC `RFC3339Nano`，严格头/签名/issuer/payloadHash 校验沿既有回执协议。新增与改动台账均为 `in_progress`，尚无 CI 精确 SHA。

`decodeCanonicalReceiptJSON` 已登记为实际参考助手，并在合同中登记 `VerifyActivationReceipt → normalizeActivationReceiptClaims/decodeCanonicalReceiptJSON/CanonicalJSON`、助手自身的规范解码→`CanonicalJSON` 依赖边。`SignActivationReceipt` 与 `ObserveTx/SealTx/ResumeTx` 仍只留在 e85 WIP，不纳入本首候选。待 site-config 从 main 独立接通最小 `RegisterActivation` 消费者后，才合成并上传既有 GitLab CI；不推送远端。
