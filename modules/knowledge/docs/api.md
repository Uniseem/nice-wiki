# 知识入库与混合检索 HTTP契约

| 方法/路径 | operationId | 处理函数 | 权限 | 输入/输出schema | 成功状态 |
|---|---|---|---|---|---|
| PUT /v1/index-config | putIndexConfig | knowledge.PutIndexConfig | knowledge.manage | PutIndexConfigInput → PutIndexConfigResult | 200 |
| POST /v1/builds | createBuild | knowledge.CreateBuild | knowledge.manage | CreateBuildInput → CreateBuildResult | 202 |
| GET /v1/builds/{buildId} | getBuild | knowledge.GetBuild | knowledge.manage | GetBuildInput → GetBuildResult | 200 |
| GET /v1/builds | listBuilds | knowledge.ListBuilds | knowledge.manage | ListBuildsInput → ListBuildsResult | 200 |
| POST /v1/builds/{buildId}/cancel | cancelBuild | knowledge.CancelBuild | knowledge.manage | CancelBuildInput → CancelBuildResult | 200 |
| POST /v1/retrievals | searchKnowledge | knowledge.Retrieve | service | RetrieveInput → RetrieveResult | 200 |
| GET /v1/builds/{buildId}/diagnostics | redactDiagnostics | knowledge.RedactDiagnostics | knowledge.manage | RedactDiagnosticsInput → RedactDiagnosticsResult | 200 |
| POST /v1/sections:read | readSections | knowledge.ReadSections | service | ReadSectionsInput → ReadSectionsResult | 200 |
| POST /v1/barriers | acquireBarrier | knowledge.AcquireBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/barriers/{barrierId}:release | releaseBarrier | knowledge.ReleaseBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/snapshots:export | exportSnapshot | knowledge.ExportSnapshot | superadmin | SnapshotExportRequest → SnapshotExportResult | 200 |
| POST /v1/snapshots:validate | validateSnapshot | knowledge.ValidateSnapshot | superadmin | SnapshotValidateRequest → SnapshotValidation | 200 |
| POST /v1/snapshots:prepare | prepareSnapshot | knowledge.PrepareSnapshot | superadmin | SnapshotPrepareRequest → SnapshotStageResult | 200 |
| POST /v1/snapshots:commit | commitSnapshot | knowledge.CommitSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/snapshots:rollback | rollbackSnapshot | knowledge.RollbackSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/evidence:inspect | inspectEvidence | knowledge.InspectEvidence | service | InspectEvidenceInput → InspectEvidenceResult | 200 |
| POST /v1/query-embeddings | createQueryEmbedding | knowledge.CreateQueryEmbedding | service | CreateQueryEmbeddingInput → CreateQueryEmbeddingResult | 200 |

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；专用恢复许可只限restore/barrier/epoch/audience/期限，不赋予普通业务权限。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
