# traffic-guard HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext和恢复许可由可信控制通道转发，不能由匿名请求自声明。

| 方法与路径 | operationId | handler ID | 权限 | 响应/行为 |
|---|---|---|---|---|
| POST `/v1/admissions` | `createAdmission` | `M12-F015` | `service` | 准入返回 AdmissionPermit，需挑战时 RATE_LIMITED+challengeId；body action 白名单 |
| POST `/v1/admissions/{admissionId}:renew` | `renewAdmission` | `M12-F016` | `service` | 仅签发者/绑定调用服务可续租，同主体、同 epoch |
| POST `/v1/admissions/{admissionId}:release` | `releaseAdmission` | `M12-F017` | `service` | 幂等释放并发占位 |
| POST `/v1/challenges` | `issueChallenge` | `M12-F019` | `anonymous` | 签发 PoW；限 6 次/分钟/主体且不返回主体 hash |
| POST `/v1/challenges/{challengeId}:verify` | `verifyChallenge` | `M12-F022` | `anonymous` | 验证 proof，成功设置 HttpOnly/Secure/SameSite=Lax 短期签名证明 |
| GET `/v1/policies` | `getPolicy` | `M12-F002` | `guard.manage` | 显示策略与 revision，不显示 pepper |
| PUT `/v1/policies` | `updatePolicy` | `M12-F003` | `guard.manage` | expectedRevision CAS；无钱账字段 |
| POST `/v1/bans` | `createBan` | `M12-F024` | `guard.manage` | 指定匿名主体引用及动作 scope，禁止提交原始 IP |
| DELETE `/v1/bans/{banId}` | `revokeBan` | `M12-F025` | `guard.manage` | 撤销 CAS，并发布审计事件 |
| GET `/v1/bans` | `listBans` | `M12-F026` | `guard.manage` | 游标分页，默认 20 上限100 |
| POST `/v1/admissions/{admissionId}:consume` | `consumeAdmission` | `M12-F044` | `service` | 同一QA请求消费准入，重复同request幂等，不预占费用 |
| POST `/v1/admissions/{admissionId}:verify` | `verifyAdmission` | `M12-F045` | `service` | 仅QA在线确认；gateway本地验签不反向同步依赖 |
| POST `/v1/question-risk` | `reportModelRisk` | `M12-F042` | `service` | 复评已有前置模型风险/匿名重复信号，abuse600秒AI禁令 |
| POST `/v1/session-queue` | `queueSessionAdmission` | `M12-F043` | `service` | 同会话仅1处理，其余由qa持正文FIFO排队 |
| POST `/v1/barriers` | `acquireBarrier` | `M12-F031` | `service` | 公共BarrierRequest→BarrierStatus；持久draining/safe，等待本域实际安全点 |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `M12-F032` | `service` | 仅全域恢复完成或commit前全部abort确认后释放 |
| POST `/v1/snapshots:export` | `exportSnapshot` | `M12-F033` | `service` | 公共SnapshotExportRequest→SnapshotExportResult；必须safe |
| POST `/v1/snapshots:validate` | `validateSnapshot` | `M12-F034` | `service` | 公共SnapshotValidateRequest→SnapshotValidation；只写隔离报告 |
| POST `/v1/snapshots:prepare` | `prepareSnapshot` | `M12-F035` | `service` | 公共SnapshotPrepareRequest→SnapshotStageResult；不切指针 |
| POST `/v1/snapshots:commit` | `commitSnapshot` | `M12-F036` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；需持久commit决定 |
| POST `/v1/snapshots:rollback` | `rollbackSnapshot` | `M12-F037` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；commit后禁止 |

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；validate只读隔离；prepare不切active；commit校验签名决策；rollback只可commit前。已commit失败必须继续同一targetEpoch并保持维护。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
