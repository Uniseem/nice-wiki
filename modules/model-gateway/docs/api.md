# 统一模型网关 HTTP契约

| 方法/路径 | operationId | 处理函数 | 权限 | 输入/输出schema | 成功状态 |
|---|---|---|---|---|---|
| PUT /v1/admin/providers/{providerId} | putProvider | model-gateway.PutProvider | superadmin | PutProviderInput → PutProviderResult | 200 |
| POST /v1/admin/providers/{providerId}/credentials | putCredential | model-gateway.PutCredential | superadmin | PutCredentialInput → PutCredentialResult | 201 |
| POST /v1/admin/providers/{providerId}/credentials/rotate | rotateCredential | model-gateway.RotateCredential | superadmin | RotateCredentialInput → RotateCredentialResult | 200 |
| PUT /v1/admin/models/{modelId} | putModel | model-gateway.PutModel | superadmin | PutModelInput → PutModelResult | 200 |
| PUT /v1/admin/profiles/{profileId} | putProfile | model-gateway.PutProfile | superadmin | PutProfileInput → PutProfileResult | 200 |
| POST /v1/inferences | createInference | model-gateway.CreateInvocation | service | CreateInvocationInput → CreateInvocationResult | 202 |
| GET /v1/inferences/{invocationId} | readInference | model-gateway.GetInvocation | service | GetInvocationInput → GetInvocationResult | 200 |
| POST /v1/inferences/{invocationId}/cancel | cancelInference | model-gateway.CancelInvocation | service | CancelInvocationInput → CancelInvocationResult | 200 |
| GET /v1/inferences/{invocationId}/events | resumeInference | model-gateway.ResumeStream | service | ResumeStreamInput → ResumeStreamResult | 200 |
| POST /v1/admin/invocations/{invocationId}/reconcile | reconcileCharge | model-gateway.ReconcileCharge | model.manage | ReconcileChargeInput → ReconcileChargeResult | 200 |
| GET /v1/admin/ledger | getLedger | model-gateway.GetLedger | model.manage | GetLedgerInput → GetLedgerResult | 200 |
| GET /v1/admin/prices/{modelId} | getPriceBook | model-gateway.GetPriceBook | model.manage | GetPriceBookInput → GetPriceBookResult | 200 |
| GET /v1/usage | queryUsage | model-gateway.QueryUsage | service | QueryUsageInput → QueryUsageResult | 200 |
| GET /v1/models | readModels | model-gateway.ReadModels | service | ReadModelsInput → ReadModelsResult | 200 |
| GET /v1/profiles | readProfiles | model-gateway.ReadProfiles | service | ReadProfilesInput → ReadProfilesResult | 200 |
| POST /v1/barriers | acquireBarrier | model-gateway.AcquireBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/barriers/{barrierId}:release | releaseBarrier | model-gateway.ReleaseBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/snapshots:export | exportSnapshot | model-gateway.ExportSnapshot | superadmin | SnapshotExportRequest → SnapshotExportResult | 200 |
| POST /v1/snapshots:validate | validateSnapshot | model-gateway.ValidateSnapshot | superadmin | SnapshotValidateRequest → SnapshotValidation | 200 |
| POST /v1/snapshots:prepare | prepareSnapshot | model-gateway.PrepareSnapshot | superadmin | SnapshotPrepareRequest → SnapshotStageResult | 200 |
| POST /v1/snapshots:commit | commitSnapshot | model-gateway.CommitSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/snapshots:rollback | rollbackSnapshot | model-gateway.RollbackSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| PUT /v1/task-assignments/{taskId} | putTaskAssignment | model-gateway.PutTaskAssignment | superadmin | PutTaskAssignmentInput → PutTaskAssignmentResult | 200 |
| GET /v1/task-assignments | readTaskAssignments | model-gateway.ReadTaskAssignments | service | ReadTaskAssignmentsInput → ReadTaskAssignmentsResult | 200 |
| POST /v1/catalog:refresh | refreshModelCatalog | model-gateway.RefreshModelCatalog | superadmin | RefreshModelCatalogInput → RefreshModelCatalogResult | 200 |
| POST /v1/providers/{providerId}:probe | createConnectionProbe | model-gateway.CreateConnectionProbe | superadmin | CreateConnectionProbeInput → CreateConnectionProbeResult | 202 |
| POST /v1/cost-quotes | quoteCost | model-gateway.QuoteCost | service | QuoteCostInput → QuoteCostResult | 200 |

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；专用恢复许可只限restore/barrier/epoch/audience/期限，不赋予普通业务权限。


| 方法/路径 | operationId | 处理函数 | 权限 | 输入/输出 | 状态 |
|---|---|---|---|---|---|
| GET /v1/budget-policy | readFeeBudgetPolicy | model-gateway.ReadFeeBudgetPolicy | guard.manage | ReadFeeBudgetPolicyInput → ReadFeeBudgetPolicyResult | 200 |
| PUT /v1/budget-policy | putFeeBudgetPolicy | model-gateway.PutFeeBudgetPolicy | guard.manage | PutFeeBudgetPolicyInput → PutFeeBudgetPolicyResult | 200 |

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
