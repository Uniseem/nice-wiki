# data-protection HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext由可信控制通道转发，不能由匿名请求自声明。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/backups` | `listBackups` | `data-protection.ListBackups` | `backup.manage` | `—` → 200 `ListBackupsResult` | 分页备份目录 | `FORBIDDEN`、`VALIDATION`、`HTTP default` |
| POST `/v1/backups` | `createBackup` | `data-protection.CreateBackup` | `backup.manage` | `CreateBackupRequestBody` → 202 `CreateBackupResult` | 建立备份操作 | `FORBIDDEN`、`CONFLICT` |
| GET `/v1/backups/{backupId}` | `readBackup` | `data-protection.ReadBackup` | `backup.manage` | `—` → 200 `ReadBackupResult` | 读取备份描述 | `FORBIDDEN`、`NOT_FOUND`、`HTTP default` |
| POST `/v1/backups/{backupId}:download` | `authorizeBackupDownload` | `data-protection.AuthorizeBackupDownload` | `superadmin` | `AuthorizeBackupDownloadRequestBody` → 200 `AuthorizeBackupDownloadResult` | 下载私有备份授权 | `FORBIDDEN`、`LOCKED` |
| POST `/v1/restores:validate` | `validateRestore` | `data-protection.ValidateRestore` | `superadmin` | `ValidateRestoreRequestBody` → 200 `ValidateRestoreResult` | 全域恢复预检 | `FORBIDDEN`、`INTEGRITY_ERROR` |
| POST `/v1/restores` | `createRestore` | `data-protection.CreateRestore` | `superadmin` | `CreateRestoreRequestBody` → 202 `CreateRestoreResult` | 建立完整恢复 | `FORBIDDEN`、`VERSION_MISMATCH` |
| POST `/v1/restores/{restoreId}:continue` | `continueRestore` | `data-protection.ContinueRestore` | `superadmin` | `ContinueRestoreRequestBody` → 200 `ContinueRestoreResult` | 继续中断恢复 | `FORBIDDEN`、`NOT_FOUND` |
| POST `/v1/restores/{restoreId}:abort` | `abortRestore` | `data-protection.AbortRestore` | `superadmin` | `AbortRestoreRequestBody` → 200 `AbortRestoreResult` | 切换前撤销或切换后整体换回 | `FORBIDDEN`、`CONFLICT` |
| GET `/v1/restores/{restoreId}/progress` | `readRestoreProgress` | `data-protection.ReadRestoreProgress` | `anonymous` | `—` → 200 `ReadRestoreProgressResult` | 读取恢复进度 | `FORBIDDEN`、`NOT_FOUND` |
| POST `/v1/mirrors` | `buildMirror` | `data-protection.BuildMirror` | `backup.manage` | `BuildMirrorRequestBody` → 200 `BuildMirrorResult` | 建立data git版本镜像 | `FORBIDDEN`、`VALIDATION` |
| POST `/v1/mirrors/{mirrorId}:sync` | `syncMirror` | `data-protection.SyncMirror` | `backup.manage` | `SyncMirrorRequestBody` → 200 `SyncMirrorResult` | 显式远程fast-forward同步 | `FORBIDDEN`、`CONFLICT`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/git-remote` | `configureRemote` | `data-protection.ConfigureRemote` | `backup.manage` | `ConfigureRemoteRequestBody` → 200 `ConfigureRemoteResult` | 设置远程同步目标 | `FORBIDDEN`、`VALIDATION`、`CONFLICT` |
| PUT `/v1/local-backup-schedule` | `scheduleLocalBackup` | `data-protection.ScheduleLocalBackup` | `backup.manage` | `ScheduleLocalBackupRequestBody` → 200 `ScheduleLocalBackupResult` | 每日本地备份 | `FORBIDDEN`、`VALIDATION` |
| POST `/v1/setup:restore` | `restoreDuringSetup` | `data-protection.RestoreDuringSetup` | `superadmin` | `RestoreDuringSetupRequestBody` → 200 `RestoreDuringSetupResult` | 初始化向导从完整备份恢复 | `FORBIDDEN`、`INTEGRITY_ERROR` |
| POST `/v1/backups:import` | `importPortableBackup` | `data-protection.ImportPortableBackup` | `superadmin` | `ImportPortableBackupRequestBody` → 200 `ImportPortableBackupResult` | 恢复入口接收完整备份 | `FORBIDDEN`、`INTEGRITY_ERROR` |
| GET `/v1/backups/{backupId}/download` | `streamPortableBackup` | `data-protection.StreamPortableBackup` | `backup.manage` | `—` → 200 `application/zip` | 下载可移植完整备份 | `FORBIDDEN`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/barriers` | `acquireBarrier` | `data-protection.EnterBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得本域快照安全截点 | `DEADLINE_EXCEEDED` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `data-protection.ReleaseBarrier` | `service` | `ReleaseBarrierRequestBody` → 200 `ReleaseBarrierResult` | 解除本域维护 | `LOCKED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `data-protection.CreateSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出本域操作目录快照 | `LOCKED`、`INTERNAL` |
| GET `/healthz` | `health` | `data-protection.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `data-protection.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `data-protection.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `data-protection.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| GET `/v1/restores/{restoreId}/archives/{owner}` | `serveRestoreArchive` | `data-protection.ServeRestoreArchive` | `service` | `—` → 200 `application/octet-stream` | 向暂存中的域提供其归档 | `FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`INTEGRITY_ERROR` |
| POST `/v1/restores/{restoreId}:cleanup` | `cleanupRestore` | `data-protection.CleanupRestore` | `superadmin` | `CleanupRestoreInput` → 200 `CleanupRestoreResult` | 清理恢复保留目录 | `FORBIDDEN`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/mirrors/status` | `readMirrorStatus` | `data-protection.ReadMirrorStatus` | `backup.manage` | `—` → 200 `ReadMirrorStatusResult` | 读取镜像与远程同步状态 | `FORBIDDEN`、`HTTP default` |
| GET `/v1/settings` | `readProtectionSettings` | `data-protection.ReadProtectionSettings` | `backup.manage` | `—` → 200 `ReadProtectionSettingsResult` | 读取备份与镜像设置 | `FORBIDDEN` |
<!-- openapi:end -->

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；stage只写restore/<restoreId>/staged，不动active；discard只删本次恢复的暂存或旧目录；切换在进程重启时完成（ADR0003）。
