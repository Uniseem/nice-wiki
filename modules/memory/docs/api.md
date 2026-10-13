# 长期记忆、任务映射与记忆助手 HTTP契约

| 方法/路径 | operationId | 处理函数 | 权限 | 输入/输出schema | 成功状态 |
|---|---|---|---|---|---|
| POST /v1/entries | createEntry | memory.CreateEntry | memory.manage | CreateEntryInput → CreateEntryResult | 201 |
| PUT /v1/entries/{entryId} | updateEntry | memory.UpdateEntry | memory.manage | UpdateEntryInput → UpdateEntryResult | 200 |
| GET /v1/entries/{entryId} | readEntry | memory.ReadEntry | memory.manage | ReadEntryInput → ReadEntryResult | 200 |
| GET /v1/entries | listEntries | memory.ListEntries | memory.manage | ListEntriesInput → ListEntriesResult | 200 |
| POST /v1/entries/{entryId}/archive | archiveEntry | memory.ArchiveEntry | memory.manage | ArchiveEntryInput → ArchiveEntryResult | 200 |
| GET /v1/entries/{entryId}/revisions | listEntryRevisions | memory.ListEntryRevisions | memory.manage | ListEntryRevisionsInput → ListEntryRevisionsResult | 200 |
| PUT /v1/mappings/{mappingId} | putMapping | memory.PutMapping | memory.manage | PutMappingInput → PutMappingResult | 200 |
| DELETE /v1/mappings/{mappingId} | removeMapping | memory.RemoveMapping | memory.manage | RemoveMappingInput → RemoveMappingResult | 200 |
| GET /v1/mappings | listMappings | memory.ListMappings | memory.manage | ListMappingsInput → ListMappingsResult | 200 |
| PUT /v1/task-budgets/{taskId} | putTaskBudget | memory.PutTaskBudget | memory.manage | PutTaskBudgetInput → PutTaskBudgetResult | 200 |
| POST /v1/context:resolve | resolveContext | memory.ResolveContext | service | ResolveContextInput → MemoryContext | 200 |
| POST /v1/assistant-runs | createAssistantRun | memory.CreateAssistantRun | memory.manage | CreateAssistantRunInput → CreateAssistantRunResult | 202 |
| GET /v1/assistant-runs/{runId} | readAssistantRun | memory.ReadAssistantRun | memory.manage | ReadAssistantRunInput → ReadAssistantRunResult | 200 |
| POST /v1/assistant-runs/{runId}/cancel | cancelAssistantRun | memory.CancelAssistantRun | memory.manage | CancelAssistantRunInput → CancelAssistantRunResult | 200 |
| POST /v1/proposals/{proposalId}/approve | approveProposal | memory.ApproveProposal | memory.manage | ApproveProposalInput → ApproveProposalResult | 202 |
| POST /v1/proposals/{proposalId}/reject | rejectProposal | memory.RejectProposal | memory.manage | RejectProposalInput → RejectProposalResult | 200 |
| POST /v1/barriers | acquireBarrier | memory.AcquireBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/barriers/{barrierId}:release | releaseBarrier | memory.ReleaseBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/snapshots:export | exportSnapshot | memory.ExportSnapshot | superadmin | SnapshotExportRequest → SnapshotExportResult | 200 |
| POST /v1/snapshots:validate | validateSnapshot | memory.ValidateSnapshot | superadmin | SnapshotValidateRequest → SnapshotValidation | 200 |
| POST /v1/snapshots:prepare | prepareSnapshot | memory.PrepareSnapshot | superadmin | SnapshotPrepareRequest → SnapshotStageResult | 200 |
| POST /v1/snapshots:commit | commitSnapshot | memory.CommitSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/snapshots:rollback | rollbackSnapshot | memory.RollbackSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/assistant-messages | appendAssistantMessage | memory.AppendAssistantMessage | memory.manage | AppendAssistantMessageInput → AppendAssistantMessageResult | 201 |
| GET /v1/assistant-conversations/{conversationId} | readAssistantConversation | memory.ReadAssistantConversation | memory.manage | ReadAssistantConversationInput → ReadAssistantConversationResult | 200 |
| DELETE /v1/assistant-conversations/{conversationId} | deleteAssistantConversation | memory.DeleteAssistantConversation | memory.manage | DeleteAssistantConversationInput → DeleteAssistantConversationResult | 200 |

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；专用恢复许可只限restore/barrier/epoch/audience/期限，不赋予普通业务权限。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
