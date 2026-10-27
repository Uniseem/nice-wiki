# site-config 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 42 个：手写 26、UI 1、生成 3、配置 6、迁移 6。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/site-config/internal/config_service.go` | 手写 | 读取后台站点配置 |
| `modules/site-config/internal/config_validator.go` | 手写 | 校验配置所有字段和关联约束 |
| `modules/site-config/internal/language_service.go` | 手写 | 规范基准语言 tag |
| `modules/site-config/internal/home_service.go` | 手写 | 保存一屏hero权威配置、公开安全投影和媒体引用持久恢复 |
| `modules/site-config/internal/home_validator.go` | 手写 | 验证首页目标与引用 |
| `modules/site-config/internal/runtime_state_service.go` | 手写 | 读取独立控制库权威运行状态 |
| `modules/site-config/internal/runtime_store.go` | 手写 | 打开独立control.db与active/site-config.db并持久化运行状态、激活回执、业务索引与操作幂等记录 |
| `modules/site-config/internal/write_epoch.go` | 手写 | 所有业务写入执行epoch与屏障检查 |
| `modules/site-config/internal/migrations/migrations.go` | 手写 | 内嵌本域control.db只追加SQL迁移并校验编号连续 |
| `modules/site-config/internal/migrations/0001_control.sql` | 迁移 | 初始control.db表：runtime_state、activation_receipts、runtime_operation_receipts与schema_migrations |
| `modules/site-config/internal/migrations/active/0001_active.sql` | 迁移 | 初始active业务索引表：schema_migrations、site_config_state、config_revision_index与announcement_heads |
| `modules/site-config/internal/initialization_service.go` | 手写 | 无需AI完成首站点初始化 |
| `modules/site-config/internal/initialization_defaults.go` | 手写 | 确定性生成首次基础配置 |
| `modules/site-config/internal/initialization_repository.go` | 手写 | 持久化不可反悔的初始化决定 |
| `modules/site-config/internal/announcement_validator.go` | 手写 | 校验公告纯文本正文并规范序列化 |
| `modules/site-config/internal/announcement_files.go` | 手写 | 持久写不可变公告正文文件 |
| `modules/site-config/internal/operation_repository.go` | 手写 | 查询配置幂等回执 |
| `modules/site-config/internal/outbox_repository.go` | 手写 | 将配置事件与业务提交原子登记，并把真实outbox单批转发装配委托给平台 |
| `modules/site-config/internal/object_reader.go` | 手写 | 经协议读取本域正文或快照对象 |
| `modules/site-config/internal/branding_service.go` | 手写 | 更新站名/logo/图标/主题色/默认分享图 |
| `modules/site-config/internal/footer_service.go` | 手写 | 更新页脚版权/自定义链接/ICP备案/源码链接 |
| `modules/site-config/internal/banner_service.go` | 手写 | 保存单条可关闭公告横幅 |
| `modules/site-config/internal/config_files.go` | 手写 | 写权威版本化站点设置文件 |
| `modules/site-config/internal/config_history.go` | 手写 | 读取可回滚的站点设置历史 |
| `modules/site-config/internal/resource_reference_reader.go` | 手写 | 向媒体返回站点设置权威资源引用快照 |
| `modules/site-config/cmd/site-config/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/site-config/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/site-config/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/site-config/go.mod` | 配置 | 独立module nice-wiki/modules/site-config；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/site-config/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/site-config/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/site-config；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/site-config/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/site-config与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/site-config/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/site-config/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/site-config/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/site-config/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/site-config/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/site-config/internal/migrations/active/0002_initialization.sql` | 迁移 | 初始化不可反悔决定、完整准备配置及SHA256、幂等回执和当前协调状态；不替代settings权威文件 |
| `modules/site-config/internal/migrations/active/0003_config_commit.sql` | 迁移 | 只追加配置操作原始身份、摘要和不可重建结果回执；outbox结构复用platform |
| `modules/site-config/internal/migrations/active/0004_config_operations.sql` | 迁移 | 一般设置操作的完整候选配置规范JSON/hash，只是重试恢复材料；正式配置仍以settings文件为权威 |
| `modules/site-config/internal/migrations/active/0005_home_reference_decisions.sql` | 迁移 | 追加首页资源pin intent、持久终局、原pin及确认/释放续办阶段，不替代settings文件权威 |
| `modules/site-config/web/src/index.tsx` | UI | 站点设置：基本信息、品牌图片、首页主视觉（含预览）、公告横幅、页脚 |
