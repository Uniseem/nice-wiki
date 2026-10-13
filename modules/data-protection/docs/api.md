# data-protection HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext和恢复许可由可信控制通道转发，不能由匿名请求自声明。

| 方法与路径 | operationId | handler ID | 权限 | 响应/行为 |
|---|---|---|---|---|
| POST `/v1/backups` | `createBackup` | `M15-F002` | `backup.manage` | 202 backup引用；自动维护屏障到ready或failed |
| GET `/v1/backups` | `listBackups` | `M15-F013` | `backup.manage` | 状态/大小/manifest hash；不泄漏封装密钥 |
| GET `/v1/backups/{backupId}` | `readBackup` | `M15-F014` | `backup.manage` | 完整域列表、schema、水位、校验报告 |
| POST `/v1/backups/{backupId}:download` | `authorizeBackupDownload` | `M15-F015` | `superadmin` | 短期单用途下载授权，解密密钥另管 |
| POST `/v1/restores:validate` | `validateRestore` | `M15-F017` | `superadmin` | 全manifest+跨域引用校验报告；不切代 |
| POST `/v1/restores` | `createRestore` | `M15-F018` | `superadmin` | 202恢复；完整备份指定targetEpoch，不允许部分域恢复 |
| POST `/v1/restores/{restoreId}:continue` | `continueRestore` | `M15-F026` | `superadmin` | 已commit仅继续激活；未commit按持久记录继续prepare |
| POST `/v1/restores/{restoreId}:abort` | `abortRestore` | `M15-F028` | `superadmin` | 仅commit前可abort，需全部stage rollback确认 |
| GET `/v1/restores/{restoreId}/decision` | `getRestoreDecision` | `M15-F025` | `service` | 签名持久决策供各域激活验证 |
| POST `/v1/mirrors` | `buildMirror` | `M15-F032` | `backup.manage` | 从ready backup的允许字段投影建立data git commit |
| POST `/v1/mirrors/{mirrorId}:sync` | `syncMirror` | `M15-F036` | `backup.manage` | 仅显式已配置远程，fast-forward push；冲突409无force |
| PUT `/v1/git-remote` | `configureRemote` | `M15-F034` | `backup.manage` | 只存无credential URL与外部credentialRef，CAS |
| PUT `/v1/local-backup-schedule` | `scheduleLocalBackup` | `M15-F053` | `backup.manage` | 每日本地备份配置，默认02:00站点时区/N=20 |
| POST `/v1/setup:restore` | `restoreDuringSetup` | `M15-F055` | `superadmin` | 初始化受控setup授权入口，完整恢复且不要求AI |
| POST `/v1/backups:import` | `importPortableBackup` | `M15-F058` | `superadmin` | 既有恢复入口上传便携备份，源服务与源运行key不必在线 |
| GET `/v1/backups/{backupId}/download` | `streamPortableBackup` | `M15-F056` | `backup.manage` | 透明可移植ZIP流，原权限和内部单用途token，无额外人工步骤 |
| POST `/v1/barriers` | `acquireBarrier` | `M15-F041` | `service` | 公共BarrierRequest→BarrierStatus；持久draining/safe，等待本域实际安全点 |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `M15-F042` | `service` | 仅全域恢复完成或commit前全部abort确认后释放 |
| POST `/v1/snapshots:export` | `exportSnapshot` | `M15-F043` | `service` | 公共SnapshotExportRequest→SnapshotExportResult；必须safe |
| POST `/v1/snapshots:validate` | `validateSnapshot` | `M15-F044` | `service` | 公共SnapshotValidateRequest→SnapshotValidation；只写隔离报告 |
| POST `/v1/snapshots:prepare` | `prepareSnapshot` | `M15-F045` | `service` | 公共SnapshotPrepareRequest→SnapshotStageResult；不切指针 |
| POST `/v1/snapshots:commit` | `commitSnapshot` | `M15-F046` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；需持久commit决定 |
| POST `/v1/snapshots:rollback` | `rollbackSnapshot` | `M15-F047` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；commit后禁止 |

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；validate只读隔离；prepare不切active；commit校验签名决策；rollback只可commit前。已commit失败必须继续同一targetEpoch并保持维护。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
