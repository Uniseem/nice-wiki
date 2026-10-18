# 长期记忆、任务映射与记忆助手 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/entries` | `listEntries` | `memory.ListEntries` | `memory.manage` | `—` → 200 `ListEntriesResult` | 管理条目列表 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/entries` | `createEntry` | `memory.CreateEntry` | `memory.manage` | `CreateEntryInput` → 201 `CreateEntryResult` | 创建人工记忆条目 | `HTTP 409`、`HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/entries/{entryId}` | `readEntry` | `memory.ReadEntry` | `memory.manage` | `—` → 200 `ReadEntryResult` | 读取人工记忆或历史 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/entries/{entryId}` | `updateEntry` | `memory.UpdateEntry` | `memory.manage` | `UpdateEntryInput` → 200 `UpdateEntryResult` | 人工更新记忆 | `HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/entries/{entryId}/archive` | `archiveEntry` | `memory.ArchiveEntry` | `memory.manage` | `ArchiveEntryInput` → 200 `ArchiveEntryResult` | 归档停止未来注入 | `HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/entries/{entryId}/revisions` | `listEntryRevisions` | `memory.ListEntryRevisions` | `memory.manage` | `—` → 200 `ListEntryRevisionsResult` | 读取不可变记忆历史 | `HTTP 403`、`HTTP 422`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/mappings/{mappingId}` | `putMapping` | `memory.PutMapping` | `memory.manage` | `PutMappingInput` → 200 `PutMappingResult` | 维护任务映射 | `HTTP 400`、`HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| DELETE `/v1/mappings/{mappingId}` | `removeMapping` | `memory.RemoveMapping` | `memory.manage` | `—` → 200 `RemoveMappingResult` | 移除任务映射 | `HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/mappings` | `listMappings` | `memory.ListMappings` | `memory.manage` | `—` → 200 `ListMappingsResult` | 读取任务映射 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/task-budgets/{taskId}` | `putTaskBudget` | `memory.PutTaskBudget` | `memory.manage` | `PutTaskBudgetInput` → 200 `PutTaskBudgetResult` | 配置任务注入上限 | `HTTP 400`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/context:resolve` | `resolveContext` | `memory.ResolveContext` | `service` | `ResolveContextInput` → 200 `MemoryContext` | 跨服务取得版本化记忆上下文 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/assistant-runs` | `createAssistantRun` | `memory.CreateAssistantRun` | `memory.manage` | `CreateAssistantRunInput` → 202 `CreateAssistantRunResult` | 显式运行记忆助手 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/assistant-runs/{runId}` | `readAssistantRun` | `memory.ReadAssistantRun` | `memory.manage` | `—` → 200 `ReadAssistantRunResult` | 管理助手提议 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/assistant-runs/{runId}/cancel` | `cancelAssistantRun` | `memory.CancelAssistantRun` | `memory.manage` | `CancelAssistantRunInput` → 200 `CancelAssistantRunResult` | 取消未完成助手调用 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/proposals/{proposalId}/approve` | `approveProposal` | `memory.ApproveProposal` | `memory.manage` | `ApproveProposalInput` → 202 `ApproveProposalResult` | 人工批准具体提议 | `HTTP 403`、`HTTP 409`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/proposals/{proposalId}/reject` | `rejectProposal` | `memory.RejectProposal` | `memory.manage` | `RejectProposalInput` → 200 `RejectProposalResult` | 人工拒绝未应用候选 | `HTTP 403`、`HTTP 409`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers` | `acquireBarrier` | `memory.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得本地维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `memory.ReleaseBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 释放已授权维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `memory.ExportSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出安全watermark快照 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/assistant-messages` | `appendAssistantMessage` | `memory.AppendAssistantMessage` | `memory.manage` | `AppendAssistantMessageInput` → 201 `AppendAssistantMessageResult` | 保存可连续的记忆助手对话 | `HTTP 403`、`HTTP 409`、`HTTP 400`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/assistant-conversations/{conversationId}` | `readAssistantConversation` | `memory.ReadAssistantConversation` | `memory.manage` | `—` → 200 `ReadAssistantConversationResult` | 读取记忆助手连续历史 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| DELETE `/v1/assistant-conversations/{conversationId}` | `deleteAssistantConversation` | `memory.DeleteAssistantConversation` | `memory.manage` | `—` → 200 `DeleteAssistantConversationResult` | 清空助手对话 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/healthz` | `health` | `memory.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `memory.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `memory.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `memory.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| POST `/v1/entries/{entryId}/restore` | `restoreEntry` | `memory.RestoreEntry` | `memory.manage` | `RestoreEntryInput` → 200 `RestoreEntryResult` | 恢复已归档记忆 | `NOT_FOUND`、`VERSION_MISMATCH` |
| PUT `/v1/mappings:reorder` | `reorderMappings` | `memory.ReorderMappings` | `memory.manage` | `ReorderMappingsInput` → 200 `ReorderMappingsResult` | 重排任务记忆顺序 | `NOT_FOUND`、`VERSION_MISMATCH` |
| GET `/v1/context-manifests/{versionHash}` | `readContextManifest` | `memory.ReadContextManifest` | `memory.manage` | `—` → 200 `ReadContextManifestResult` | 查看上下文版本清单 | `NOT_FOUND`、`VERSION_MISMATCH` |
| GET `/v1/task-budgets` | `listTaskBudgets` | `memory.ListTaskBudgets` | `memory.manage` | `—` → 200 `ListTaskBudgetsResult` | 读取各任务记忆注入预算 | `FORBIDDEN` |
| GET `/v1/snapshots/{snapshotId}/archive` | `readSnapshotArchive` | `memory.ReadSnapshotArchive` | `superadmin` | `—` → 200 `application/x-tar` | 受控读取已完成域快照归档 | `HTTP 403`、`HTTP 409`、`HTTP 503` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；备份恢复只用barrier/export/stage/discard，切换在进程重启时完成（ADR0003）。
