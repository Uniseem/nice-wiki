# jobs HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext和恢复许可由可信控制通道转发，不能由匿名请求自声明。

| 方法与路径 | operationId | handler ID | 权限 | 响应/行为 |
|---|---|---|---|---|
| POST `/v1/jobs` | `createJob` | `M13-F002` | `service` | 提交不可变引用与owner/type，202 AcceptedJob，epoch必须当前 |
| POST `/v1/jobs:claim` | `claimJob` | `M13-F003` | `service` | 所属owner worker领取租约与fencingToken |
| POST `/v1/jobs/{jobId}:heartbeat` | `heartbeatJob` | `M13-F005` | `service` | 同worker/fence续租30秒 |
| POST `/v1/jobs/{jobId}:complete` | `completeJob` | `M13-F007` | `service` | 同fence写不可变结果引用，不传正文 |
| POST `/v1/jobs/{jobId}:fail` | `failJob` | `M13-F010` | `service` | 错误分类决定重试或dead-letter |
| POST `/v1/jobs/{jobId}:cancel` | `cancelJob` | `M13-F013` | `job.manage` | 请求取消；正在调用模型由owner/gateway取消并仍结算费用 |
| GET `/v1/jobs/{jobId}` | `readJob` | `M13-F015` | `job.manage` | 状态/引用/安全错误码 |
| GET `/v1/jobs/{jobId}/progress` | `streamProgress` | `M13-F017` | `job.manage` | SSE Last-Event-ID重放，最多7天 |
| POST `/v1/events` | `publishEvent` | `M13-F019` | `service` | 接收已签名版本envelope，durable后202 |
| POST `/v1/events/{eventId}:ack` | `acknowledgeEvent` | `M13-F021` | `service` | 指定consumer的去重确认 |
| POST `/v1/schedules` | `createSchedule` | `M13-F026` | `job.manage` | 5字段cron+IANA时区，允许owner/type白名单 |
| PUT `/v1/schedules/{scheduleId}` | `updateSchedule` | `M13-F027` | `job.manage` | revision CAS |
| POST `/v1/barriers:coordinate` | `coordinateBarrier` | `M13-F029` | `service` | 只允许data-protection+超管委托，协调全部有状态域安全点 |
| POST `/v1/jobs/{jobId}:start` | `startJob` | `M13-F004` | `service` | 所属owner主动领取后确认running，不由jobs执行领域算法 |
| GET `/v1/jobs/{jobId}/lease` | `checkLease` | `M13-F006` | `service` | 所属owner在副作用写回前核对fence/epoch/expiry |
| GET `/v1/events:subscribe` | `subscribeEvents` | `M13-F042` | `service` | 带版本事件SSE，已注册consumer按Last-Event-ID重放再ack |
| POST `/v1/barriers` | `acquireBarrier` | `M13-F030` | `service` | 公共BarrierRequest→BarrierStatus；持久draining/safe，等待本域实际安全点 |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `M13-F031` | `service` | 仅全域恢复完成或commit前全部abort确认后释放 |
| POST `/v1/snapshots:export` | `exportSnapshot` | `M13-F033` | `service` | 公共SnapshotExportRequest→SnapshotExportResult；必须safe |
| POST `/v1/snapshots:validate` | `validateSnapshot` | `M13-F034` | `service` | 公共SnapshotValidateRequest→SnapshotValidation；只写隔离报告 |
| POST `/v1/snapshots:prepare` | `prepareSnapshot` | `M13-F035` | `service` | 公共SnapshotPrepareRequest→SnapshotStageResult；不切指针 |
| POST `/v1/snapshots:commit` | `commitSnapshot` | `M13-F036` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；需持久commit决定 |
| POST `/v1/snapshots:rollback` | `rollbackSnapshot` | `M13-F037` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；commit后禁止 |

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；validate只读隔离；prepare不切active；commit校验签名决策；rollback只可commit前。已commit失败必须继续同一targetEpoch并保持维护。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
