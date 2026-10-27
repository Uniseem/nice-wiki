# 语言、翻译与术语 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| PUT `/v1/languages/{tag}` | `putLanguage` | `localization.PutLanguage` | `localization.languages` | `PutLanguageInput` → 200 `PutLanguageResult` | 更新语言启用配置 | `HTTP 400`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/languages/{tag}/enabled` | `setLanguageEnabled` | `localization.SetLanguageEnabled` | `localization.languages` | `SetLanguageEnabledInput` → 200 `SetLanguageEnabledResult` | 启停公开语言 | `HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/languages` | `listLanguages` | `localization.ListLanguages` | `anonymous` | `—` → 200 `ListLanguagesResult` | 读取语言列表 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/translations` | `createTranslation` | `localization.CreateTranslation` | `localization.manage` | `CreateTranslationInput` → 202 `CreateTranslationResult` | 申请源修订翻译 | `HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/translations:read` | `readTranslation` | `localization.ReadTranslation` | `service` | `ReadTranslationInput` → 200 `ReadTranslationResult` | 公开读取当前译文 | `HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/translations/{translationId}/units/{unitId}` | `editTranslationUnit` | `localization.EditTranslationUnit` | `localization.manage` | `EditTranslationUnitInput` → 200 `EditTranslationUnitResult` | 人工编辑并自动保护 | `HTTP 409`、`HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/translations/{translationId}/units/{unitId}/protection` | `setUnitProtection` | `localization.SetUnitProtection` | `localization.manage` | `SetUnitProtectionInput` → 200 `SetUnitProtectionResult` | 显式启停人工保护 | `HTTP 409`、`HTTP 403`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/translations/{translationId}/cancel` | `cancelTranslation` | `localization.CancelTranslation` | `localization.manage` | `CancelTranslationInput` → 200 `CancelTranslationResult` | 取消翻译子链路 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/glossary/{termId}` | `putGlossaryTerm` | `localization.PutGlossaryTerm` | `localization.manage` | `PutGlossaryTermInput` → 200 `PutGlossaryTermResult` | 人工维护术语 | `HTTP 400`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| DELETE `/v1/glossary/{termId}` | `deleteGlossaryTerm` | `localization.DeleteGlossaryTerm` | `localization.manage` | `—` → 200 `DeleteGlossaryTermResult` | 删除术语规则 | `HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/glossary` | `listGlossary` | `localization.ListGlossary` | `localization.manage` | `—` → 200 `ListGlossaryResult` | 管理术语列表 | `HTTP 403`、`HTTP 409`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/glossary:apply-approved-proposal` | `applyApprovedGlossaryProposal` | `localization.ApplyApprovedGlossaryProposal` | `service` | `ApplyApprovedGlossaryProposalInput` → 202 `ApplyApprovedGlossaryProposalResult` | 应用memory已批准提议 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/interface-strings/{language}/{key}` | `putInterfaceString` | `localization.PutInterfaceString` | `localization.manage` | `PutInterfaceStringInput` → 200 `PutInterfaceStringResult` | 人工编辑界面文本 | `HTTP 409`、`HTTP 422`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/interface-strings/{language}` | `readInterfaceCatalog` | `localization.ReadInterfaceCatalog` | `anonymous` | `—` → 200 `ReadInterfaceCatalogResult` | 读取运行界面字典 | `HTTP 409`、`HTTP 422`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/language-suggestions` | `suggestLanguage` | `localization.SuggestLanguage` | `anonymous` | `SuggestLanguageInput` → 200 `SuggestLanguageResult` | 给访客语言建议 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/glossary-batches/{batchId}:rollback` | `rollbackAutomaticGlossaryBatch` | `localization.RollbackAutomaticGlossaryBatch` | `localization.manage` | `RollbackAutomaticGlossaryBatchInput` → 200 `RollbackAutomaticGlossaryBatchResult` | 整批撤销自动术语 | `HTTP 409`、`HTTP 403`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/healthz` | `health` | `localization.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `localization.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/languages/{tag}:estimate` | `estimateLanguagePlan` | `localization.EstimateLanguagePlan` | `localization.languages` | `EstimateLanguagePlanInput` → 200 `EstimateLanguagePlanResult` | 估算启用语言翻译费用 | `VALIDATION`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/languages/{tag}:approve-plan` | `approveLanguagePlan` | `localization.ApproveLanguagePlan` | `localization.languages` | `ApproveLanguagePlanInput` → 200 `ApproveLanguagePlanResult` | 批准估价并启用语言翻译 | `VALIDATION`、`CONFLICT`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| POST `/v1/interface-strings/{language}:translate` | `requestInterfaceTranslation` | `localization.RequestInterfaceTranslation` | `localization.manage` | `RequestInterfaceTranslationInput` → 202 `RequestInterfaceTranslationResult` | 补译界面字符串缺项 | `VALIDATION`、`UNSUPPORTED_CAPABILITY` |
| PUT `/v1/language-preferences` | `saveLanguagePreference` | `localization.SaveLanguagePreference` | `anonymous` | `SaveLanguagePreferenceInput` → 200 `SaveLanguagePreferenceResult` | 保存访客显式语言偏好 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`INTERNAL`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED`、`HTTP default` |
| GET `/v1/translations/progress` | `listTranslationProgress` | `localization.ListTranslationProgress` | `localization.manage` | `—` → 200 `ListTranslationProgressResult` | 查看翻译进度 | `FORBIDDEN` |
| GET `/v1/translations/{translationId}/units` | `readTranslationUnits` | `localization.ReadTranslationUnits` | `localization.manage` | `—` → 200 `ReadTranslationUnitsResult` | 对照编辑读取 | `FORBIDDEN`、`NOT_FOUND` |
| GET `/v1/language-suggestion-policy` | `readLanguageSuggestionPolicy` | `localization.ReadLanguageSuggestionPolicy` | `localization.manage` | `—` → 200 `LanguageSuggestionPolicy` | 读取管理员语言建议付费开关 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 422` |
| PUT `/v1/language-suggestion-policy` | `putLanguageSuggestionPolicy` | `localization.PutLanguageSuggestionPolicy` | `localization.manage` | `PutLanguageSuggestionPolicyInput` → 200 `LanguageSuggestionPolicy` | 管理员明确启停语言建议付费许可 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 422` |
| GET `/v1/language-suggestion-approvals/{approvalId}` | `readLanguageSuggestionApproval` | `localization.ReadLanguageSuggestionApproval` | `service` | `—` → 200 `ReadLanguageSuggestionApprovalResult` | 供网关核对当前语言建议批准 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 422`、`HTTP 404` |
| GET `/v1/translation-titles` | `listTranslationTitles` | `localization.ListTranslationTitles` | `service` | `—` → 200 `ListTranslationTitlesResult` | 列出译文标题 | `FORBIDDEN` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证
