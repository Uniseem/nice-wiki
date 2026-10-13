# 语言、翻译与术语 HTTP契约

| 方法/路径 | operationId | 处理函数 | 权限 | 输入/输出schema | 成功状态 |
|---|---|---|---|---|---|
| PUT /v1/languages/{tag} | putLanguage | localization.PutLanguage | localization.languages | PutLanguageInput → PutLanguageResult | 200 |
| POST /v1/languages/{tag}/enabled | setLanguageEnabled | localization.SetLanguageEnabled | localization.languages | SetLanguageEnabledInput → SetLanguageEnabledResult | 200 |
| GET /v1/languages | listLanguages | localization.ListLanguages | anonymous | ListLanguagesInput → ListLanguagesResult | 200 |
| POST /v1/translations | createTranslation | localization.CreateTranslation | localization.manage | CreateTranslationInput → CreateTranslationResult | 202 |
| POST /v1/translations:read | readTranslation | localization.ReadTranslation | service | ReadTranslationInput → ReadTranslationResult | 200 |
| PUT /v1/translations/{translationId}/units/{unitId} | editTranslationUnit | localization.EditTranslationUnit | localization.manage | EditTranslationUnitInput → EditTranslationUnitResult | 200 |
| POST /v1/translations/{translationId}/units/{unitId}/protection | setUnitProtection | localization.SetUnitProtection | localization.manage | SetUnitProtectionInput → SetUnitProtectionResult | 200 |
| POST /v1/translations/{translationId}/cancel | cancelTranslation | localization.CancelTranslation | localization.manage | CancelTranslationInput → CancelTranslationResult | 200 |
| PUT /v1/glossary/{termId} | putGlossaryTerm | localization.PutGlossaryTerm | localization.manage | PutGlossaryTermInput → PutGlossaryTermResult | 200 |
| DELETE /v1/glossary/{termId} | deleteGlossaryTerm | localization.DeleteGlossaryTerm | localization.manage | DeleteGlossaryTermInput → DeleteGlossaryTermResult | 200 |
| GET /v1/glossary | listGlossary | localization.ListGlossary | localization.manage | ListGlossaryInput → ListGlossaryResult | 200 |
| POST /v1/glossary:apply-approved-proposal | applyApprovedGlossaryProposal | localization.ApplyApprovedGlossaryProposal | service | ApplyApprovedGlossaryProposalInput → ApplyApprovedGlossaryProposalResult | 202 |
| PUT /v1/interface-strings/{language}/{key} | putInterfaceString | localization.PutInterfaceString | localization.manage | PutInterfaceStringInput → PutInterfaceStringResult | 200 |
| GET /v1/interface-strings/{language} | readInterfaceCatalog | localization.ReadInterfaceCatalog | anonymous | ReadInterfaceCatalogInput → ReadInterfaceCatalogResult | 200 |
| POST /v1/language-suggestions | suggestLanguage | localization.SuggestLanguage | anonymous | SuggestLanguageInput → SuggestLanguageResult | 200 |
| POST /v1/cache-answer-translations | translateCachedAnswer | localization.TranslateCachedAnswer | service | TranslateCachedAnswerInput → TranslateCachedAnswerResult | 202 |
| POST /v1/barriers | acquireBarrier | localization.AcquireBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/barriers/{barrierId}:release | releaseBarrier | localization.ReleaseBarrier | superadmin | BarrierRequest → BarrierStatus | 200 |
| POST /v1/snapshots:export | exportSnapshot | localization.ExportSnapshot | superadmin | SnapshotExportRequest → SnapshotExportResult | 200 |
| POST /v1/snapshots:validate | validateSnapshot | localization.ValidateSnapshot | superadmin | SnapshotValidateRequest → SnapshotValidation | 200 |
| POST /v1/snapshots:prepare | prepareSnapshot | localization.PrepareSnapshot | superadmin | SnapshotPrepareRequest → SnapshotStageResult | 200 |
| POST /v1/snapshots:commit | commitSnapshot | localization.CommitSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/snapshots:rollback | rollbackSnapshot | localization.RollbackSnapshot | superadmin | SnapshotCommitRequest → SnapshotApplyResult | 200 |
| POST /v1/glossary-batches/{batchId}:rollback | rollbackAutomaticGlossaryBatch | localization.RollbackAutomaticGlossaryBatch | localization.manage | RollbackAutomaticGlossaryBatchInput → RollbackAutomaticGlossaryBatchResult | 200 |
| GET /v1/cache-answer-translations/{translationId} | readCacheAnswerTranslation | localization.ReadCacheAnswerTranslation | service | ReadCacheAnswerTranslationInput → ReadCacheAnswerTranslationResult | 200 |

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；专用恢复许可只限restore/barrier/epoch/audience/期限，不赋予普通业务权限。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
