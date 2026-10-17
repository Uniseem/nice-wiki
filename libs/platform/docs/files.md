# platform 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 19 个：手写 17、生成 1、配置 1。函数 53 个：约束 19，参考 34。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `libs/platform/serviceauth/assertion.go` | 手写 | 服务断言签发与验证 | `platform.SignAssertion`、`platform.VerifyAssertion`、`platform.validLoginScopes`（参考）、`platform.PrivateNetworkScopes`（参考） |
| `libs/platform/serviceauth/delegation.go` | 手写 | 用户委托验证与作用域绑定的分页游标签名 | `platform.VerifyDelegation`、`platform.SignCursor`（参考）、`platform.VerifyCursor`（参考）、`platform.SignSessionAccess`（参考）、`platform.VerifySessionAccess`（参考）、`platform.GenerateSessionSecrets`（参考）、`platform.SignDelegation`（参考）、`platform.validSessionClaims`（参考）、`platform.PrivateOperationDigest`（参考） |
| `libs/platform/opctx/context.go` | 手写 | 可信OperationContext构建 | `platform.BuildOperationContext` |
| `libs/platform/problem/problem.go` | 手写 | 公共Problem输出 | `platform.WriteProblem`、`platform.FromError`（参考） |
| `libs/platform/outbox/outbox.go` | 手写 | 事务内outbox登记 | `platform.EnqueueEvent` |
| `libs/platform/outbox/relay.go` | 手写 | outbox转发循环 | `platform.RunRelay`、`platform.NewEventPublisher`（参考）、`platform.PublishEvent`（参考）、`platform.relayWriteContext`（参考）、`platform.RunRelayBatch`（参考） |
| `libs/platform/inbox/inbox.go` | 手写 | 事件inbox去重 | `platform.ClaimInbox` |
| `libs/platform/inbox/consumer.go` | 手写 | 事件订阅循环 | `platform.RunConsumer` |
| `libs/platform/worker/loop.go` | 手写 | 任务领取循环 | `platform.RunWorkerLoop`、`platform.workerControlContext`（参考）、`platform.newExecutionLease`（参考）、`platform.expireExecutionLease`（参考）、`platform.executionLeaseContext`（参考）、`platform.renewExecutionLease`（参考） |
| `libs/platform/modelclient/invoke.go` | 手写 | 模型调用助手 | `platform.InvokeModel` |
| `libs/platform/lifecycle/config.go` | 手写 | 服务配置加载 | `platform.LoadServiceConfig`、`platform.NormalizePublicURL`（参考）、`platform.CanonicalPublicOrigin`（参考）、`platform.ValidatePrivatePath`（参考） |
| `libs/platform/lifecycle/runtime.go` | 手写 | 运行状态读取与监视 | `platform.ReadRuntime`、`platform.WatchRuntime`、`platform.NewSiteConfigClient`、`platform.NewWriteContext`（参考）、`platform.RunRecoveryLoop`（参考） |
| `libs/platform/apivalidate/validator.go` | 手写 | HTTP入口统一请求校验中间件（kin-openapi openapi3filter） | `platform.NewRequestValidator`、`platform.NewPayloadValidator`（参考）、`platform.ServeEventStream`（参考）、`platform.NewServiceHandler`（参考）、`platform.NewIdentityAuthorizer`（参考）、`platform.ServeJSONEventStream`（参考）、`platform.envelopeJSONStream.Next`（参考）、`platform.envelopeJSONStream.Close`（参考） |
| `libs/platform/lifecycle/restore_swap.go` | 手写 | 恢复换目录 | `platform.ApplyRestoreSwap` |
| `libs/platform/lifecycle/service.go` | 手写 | 进程模板 | `platform.RunService` |
| `libs/platform/cmd/nw-keys/main.go` | 手写 | 服务密钥生成工具 | `platform.main` |
| `libs/platform/platform.go` | 手写 | 公共契约类型、接口和确定性纯工具，禁止业务规则 | `platform.CanonicalJSON`（参考）、`platform.NormalizeIOError`（参考） |
| `libs/platform/go.mod` | 配置 | Go module nice-wiki/libs/platform；只依赖标准库与ADR0005登记的第三方库；各服务以相对replace引用 | 无 |
| `libs/platform/go.sum` | 生成 | 依赖校验锁；由go mod download生成 | 无 |
