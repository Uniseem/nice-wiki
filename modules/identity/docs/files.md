# identity 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 34 个：手写 18、UI 1、生成 4、配置 6、迁移 5。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/identity/internal/registration_service.go` | 手写 | 决定后台显示首次注册或登录 |
| `modules/identity/internal/account_service.go` | 手写 | 超管创建子账号并标记强改 |
| `modules/identity/internal/account_validator.go` | 手写 | 规范不可变账号名 |
| `modules/identity/internal/permission_mapping.go` | 手写 | 将已确认功能勾选映射成服务权限 |
| `modules/identity/internal/credential_service.go` | 手写 | 生成Argon2id口令哈希 |
| `modules/identity/internal/session_service.go` | 手写 | 账号名口令登录 |
| `modules/identity/internal/session_revocation.go` | 手写 | 退出当前或指定自身登录 |
| `modules/identity/internal/csrf_service.go` | 手写 | portal内部核对CSRF |
| `modules/identity/internal/authorization_service.go` | 手写 | 按最新账号功能权限授权 |
| `modules/identity/internal/preference_service.go` | 手写 | 在当前本人授权快照读写后台偏好与完整安全回执 |
| `modules/identity/internal/server_recovery.go` | 手写 | 服务器运维查明文账号索引，并消费固定超管重置文件 |
| `modules/identity/internal/operation_repository.go` | 手写 | 装配受信私有存储上下文与既有仓库调用前置 |
| `modules/identity/internal/outbox_repository.go` | 手写 | 与身份提交原子登记事件并装配可靠转发循环 |
| `modules/identity/internal/storage.go` | 手写 | 打开active区业务库（WAL/foreign_keys/busy_timeout=3000ms）、执行只追加迁移并恢复control区决定日志 |
| `modules/identity/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005），校验编号从1起连续 |
| `modules/identity/migrations/0001_domain.sql` | 迁移 | 初始权威表：accounts（唯一超管部分唯一索引）、credentials、account_permissions、sessions、preferences、operation_receipts、schema_migrations；nw_outbox/nw_inbox由平台EnsureSchema创建不在此登记 |
| `modules/identity/cmd/identity/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/identity/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/identity/internal/transport/server.go` | 手写 | HTTP/worker生命周期、后台任务、优雅关闭、匿名健康与就绪 |
| `modules/identity/go.mod` | 配置 | 独立module nice-wiki/modules/identity；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/identity/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/identity/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/identity；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/identity/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/identity与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/identity/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/identity/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/identity/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/identity/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/identity/internal/gen/permission_groups.gen.go` | 生成 | make gen由根contracts/permission-groups.json生成功能开关到技术权限的固定Go表；MapFunctionGrants只读此表，不手改 |
| `modules/identity/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/identity/migrations/0002_session_revocation.sql` | 迁移 | 为自有撤销事务追加完整结果回执列，并建立自身会话键集分页索引；不变更旧会话或预埋未确认签发协议 |
| `modules/identity/migrations/0003_session_tokens.sql` | 迁移 | 只追加认证代次、会话族及已消费refresh摘要权威表，旧会话不生成可用凭据 |
| `modules/identity/migrations/0004_private_operation_receipts.sql` | 迁移 | 追加私有口令操作HMAC keyId，不保存普通口令SHA或raw cookie |
| `modules/identity/migrations/control/0001_bootstrap.sql` | 迁移 | 独立control schema1与不可逆bootstrap决定；不随active恢复替换 |
| `modules/identity/web/src/index.tsx` | UI | 账号与权限：子账号的新建、功能权限、停用/恢复/删除、重置口令 |
