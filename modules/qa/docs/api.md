# 问答、引用与回答缓存 HTTP契约

| 方法/路径 | operationId | 处理函数 | 权限 | 输入/输出schema | 成功状态 |
|---|---|---|---|---|---|
| POST /v1/conversations | createConversation | qa.CreateConversation | anonymous | CreateConversationInput → CreateConversationResult | 201 |
| GET /v1/conversations/{conversationId} | getConversation | qa.GetConversation | anonymous | GetConversationInput → GetConversationResult | 200 |
| DELETE /v1/conversations/{conversationId} | deleteConversation | qa.DeleteConversation | anonymous | DeleteConversationInput → DeleteConversationResult | 200 |
| POST /v1/turns | startTurn | qa.StartTurn | anonymous | StartTurnInput → StartTurnResult | 202 |
| GET /v1/turns/{turnId}/events | resumeAnswerStream | qa.ResumeAnswerStream | anonymous | ResumeAnswerStreamInput → ResumeAnswerStreamResult | 200 |
| POST /v1/turns/{turnId}/cancel | cancelTurn | qa.CancelTurn | anonymous | CancelTurnInput → CancelTurnResult | 200 |
| PUT /v1/turns/{turnId}/feedback | recordFeedback | qa.RecordFeedback | anonymous | RecordFeedbackInput → RecordFeedbackResult | 200 |
| PUT /v1/admin/policy | putQaPolicy | qa.PutQaPolicy | qa.manage | PutQaPolicyInput → PutQaPolicyResult | 200 |
| POST /v1/admin/cache | proposeCacheEntry | qa.ProposeCacheEntry | qa.manage | ProposeCacheEntryInput → ProposeCacheEntryResult | 201 |
| POST /v1/admin/cache/{cacheId}/approve | approveCacheEntry | qa.ApproveCacheEntry | qa.manage | ApproveCacheEntryInput → ApproveCacheEntryResult | 200 |
| POST /v1/admin/cache/{cacheId}/disable | disableCacheEntry | qa.DisableCacheEntry | qa.manage | DisableCacheEntryInput → DisableCacheEntryResult | 200 |
| GET /v1/admin/cache | listCacheEntries | qa.ListCacheEntries | qa.manage | ListCacheEntriesInput → ListCacheEntriesResult | 200 |
| POST /v1/admin/cache/{cacheId}/translations | translateCacheEntry | qa.TranslateCacheEntry | service | TranslateCacheEntryInput → TranslateCacheEntryResult | 202 |
| POST /v1/admin/debug-turns | createDebugTurn | qa.CreateDebugTurn | qa.debug | CreateDebugTurnInput → CreateDebugTurnResult | 202 |
| GET /v1/admin/debug-turns/{debugId} | readDebugTurn | qa.ReadDebugTurn | qa.debug | ReadDebugTurnInput → ReadDebugTurnResult | 200 |
| POST /v1/barriers | acquireBarrier | qa.AcquireBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/barriers/{barrierId}:release | releaseBarrier | qa.ReleaseBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/snapshots:export | exportSnapshot | qa.ExportSnapshot | superadmin | SnapshotExportRequest → SnapshotExportResult | 200 |
| POST /v1/snapshots:validate | validateSnapshot | qa.ValidateSnapshot | superadmin | SnapshotValidateRequest → SnapshotValidation | 200 |
| POST /v1/snapshots:prepare | prepareSnapshot | qa.PrepareSnapshot | superadmin | SnapshotPrepareRequest → SnapshotStageResult | 200 |
| POST /v1/snapshots:commit | commitSnapshot | qa.CommitSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/snapshots:rollback | rollbackSnapshot | qa.RollbackSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| GET /v1/admin/cache-calibration | readCacheCalibration | qa.ReadCacheCalibration | qa.manage | ReadCacheCalibrationInput → ReadCacheCalibrationResult | 200 |

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；专用恢复许可只限restore/barrier/epoch/audience/期限，不赋予普通业务权限。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
