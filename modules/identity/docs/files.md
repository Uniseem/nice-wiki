# identity 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=28；分类={"handwritten": 22, "config": 5, "generated": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/identity/internal/registration_service.go` | handwritten | 决定后台显示首次注册或登录 | `identity.ReadRegistrationState`, `identity.RegisterFirstSuperadmin` |
| `modules/identity/internal/account_service.go` | handwritten | 超管创建子账号并标记强改 | `identity.CreateAccount`, `identity.ReadSelf`, `identity.ListAccounts`, `identity.SetAccountStatus`, `identity.SetAccountPermissions`, `identity.ResetAccountPassword` |
| `modules/identity/internal/account_validator.go` | handwritten | 规范不可变账号名 | `identity.NormalizeAccountName`, `identity.AssertSuperadminImmutable` |
| `modules/identity/internal/permission_mapping.go` | handwritten | 将已确认功能勾选映射成服务权限 | `identity.MapFunctionGrants` |
| `modules/identity/internal/credential_service.go` | handwritten | 生成Argon2id口令哈希 | `identity.HashPassword`, `identity.VerifyPassword`, `identity.ValidatePasswordChange`, `identity.ChangePassword` |
| `modules/identity/internal/session_service.go` | handwritten | 账号名口令登录 | `identity.CreateSession`, `identity.IssueSessionTokens`, `identity.RefreshSession`, `identity.ReadSessions` |
| `modules/identity/internal/session_revocation.go` | handwritten | 退出当前或指定自身登录 | `identity.RevokeSession`, `identity.RevokeSessionFamily`, `identity.RevokeAllSessions`, `identity.RevokeOtherSessions` |
| `modules/identity/internal/csrf_service.go` | handwritten | portal内部核对CSRF | `identity.VerifySessionCSRF` |
| `modules/identity/internal/authorization_service.go` | handwritten | 按最新账号功能权限授权 | `identity.CheckAuthorization` |
| `modules/identity/internal/preference_service.go` | handwritten | 读自身后台偏好 | `identity.ReadPreferences`, `identity.UpdatePreferences` |
| `modules/identity/internal/server_recovery.go` | handwritten | 服务器运维查明文账号索引 | `identity.ReadServerAccountIndex`, `identity.WriteAccountIndex`, `identity.ObserveSuperadminResetFile`, `identity.ApplySuperadminReset` |
| `modules/identity/internal/recovery_authorization.go` | handwritten | 超管批准后签发有限恢复许可 | `identity.IssueRecoveryAuthorization`, `identity.ValidateRecoveryAuthorization`, `identity.RevokeRecoveryAuthorization` |
| `modules/identity/internal/operation_repository.go` | handwritten | 读取幂等回执 | `identity.ResolveIdentityOperation` |
| `modules/identity/internal/outbox_repository.go` | handwritten | 与身份提交原子登记事件 | `identity.PublishIdentityEvent` |
| `modules/identity/internal/restore_security.go` | handwritten | 恢复数据后使备份登录失效 | `identity.InvalidateRestoredSessions` |
| `modules/identity/internal/barrier_service.go` | handwritten | 取得可恢复的数据屏障 | `identity.AcquireBarrier`, `identity.ReleaseBarrier` |
| `modules/identity/internal/snapshot_service.go` | handwritten | 导出屏障水位的一致快照 | `identity.ExportSnapshot`, `identity.PrepareSnapshot` |
| `modules/identity/internal/snapshot_validator.go` | handwritten | 验证快照可恢复性 | `identity.ValidateSnapshot` |
| `modules/identity/internal/snapshot_repository.go` | handwritten | 激活已准备恢复数据 | `identity.CommitSnapshot`, `identity.RollbackSnapshot` |
| `modules/identity/cmd/identity/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M02-F901` |
| `modules/identity/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M02-F902` |
| `modules/identity/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M02-F903`, `M02-F904`, `M02-F905`, `M02-F906` |
| `modules/identity/go.mod` | config | 独立module nice-wiki/modules/identity；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/identity/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/identity/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/identity；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/identity/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/identity与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/identity/config/service.defaults.json` | config | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/identity/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
