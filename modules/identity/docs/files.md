# identity 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 25 个：手写 19、生成 1、配置 5。函数 46 个：约束 29，参考 17。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/identity/internal/registration_service.go` | 手写 | 决定后台显示首次注册或登录 | `identity.ReadRegistrationState`、`identity.RegisterFirstSuperadmin` |
| `modules/identity/internal/account_service.go` | 手写 | 超管创建子账号并标记强改 | `identity.CreateAccount`、`identity.ReadSelf`、`identity.ListAccounts`、`identity.SetAccountStatus`、`identity.SetAccountPermissions`、`identity.ResetAccountPassword` |
| `modules/identity/internal/account_validator.go` | 手写 | 规范不可变账号名 | `identity.NormalizeAccountName`（参考）、`identity.AssertSuperadminImmutable`（参考） |
| `modules/identity/internal/permission_mapping.go` | 手写 | 将已确认功能勾选映射成服务权限 | `identity.MapFunctionGrants`（参考） |
| `modules/identity/internal/credential_service.go` | 手写 | 生成Argon2id口令哈希 | `identity.HashPassword`（参考）、`identity.VerifyPassword`（参考）、`identity.ValidatePasswordChange`（参考）、`identity.ChangePassword` |
| `modules/identity/internal/session_service.go` | 手写 | 账号名口令登录 | `identity.CreateSession`、`identity.IssueSessionTokens`（参考）、`identity.RefreshSession`、`identity.ReadSessions` |
| `modules/identity/internal/session_revocation.go` | 手写 | 退出当前或指定自身登录 | `identity.RevokeSession`、`identity.RevokeSessionFamily`（参考）、`identity.RevokeAllSessions`（参考）、`identity.RevokeOtherSessions`（参考） |
| `modules/identity/internal/csrf_service.go` | 手写 | portal内部核对CSRF | `identity.VerifySessionCSRF` |
| `modules/identity/internal/authorization_service.go` | 手写 | 按最新账号功能权限授权 | `identity.CheckAuthorization` |
| `modules/identity/internal/preference_service.go` | 手写 | 读自身后台偏好 | `identity.ReadPreferences`、`identity.UpdatePreferences` |
| `modules/identity/internal/server_recovery.go` | 手写 | 服务器运维查明文账号索引 | `identity.ReadServerAccountIndex`（参考）、`identity.WriteAccountIndex`（参考）、`identity.ObserveSuperadminResetFile`（参考）、`identity.ApplySuperadminReset`（参考） |
| `modules/identity/internal/operation_repository.go` | 手写 | 读取幂等回执 | `identity.ResolveIdentityOperation`（参考） |
| `modules/identity/internal/outbox_repository.go` | 手写 | 与身份提交原子登记事件 | `identity.PublishIdentityEvent`（参考）、`identity.RelayOutbox` |
| `modules/identity/internal/restore_security.go` | 手写 | 恢复数据后使备份登录失效 | `identity.InvalidateRestoredSessions`（参考） |
| `modules/identity/internal/barrier_service.go` | 手写 | 取得可恢复的数据屏障 | `identity.AcquireBarrier`、`identity.ReleaseBarrier` |
| `modules/identity/internal/snapshot_service.go` | 手写 | 导出屏障水位的一致快照 | `identity.ExportSnapshot`、`identity.StageSnapshot`、`identity.DiscardSnapshot` |
| `modules/identity/cmd/identity/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `identity.main` |
| `modules/identity/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `identity.BuildApplication` |
| `modules/identity/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `identity.Serve`、`identity.Shutdown`、`identity.Health`、`identity.Ready` |
| `modules/identity/go.mod` | 配置 | 独立module nice-wiki/modules/identity；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/identity/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/identity/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/identity；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/identity/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/identity与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/identity/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/identity/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
