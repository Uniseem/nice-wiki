# traffic-guard 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 46 个：手写 33、UI 1、生成 4、配置 6、迁移 2。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/traffic-guard/internal/policy.go` | 手写 | 校验风控参数；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/policy_store.go` | 手写 | 私有策略与登录保护库的打开代次、迁移、control journal和可信上下文注入；策略读写仍待后续 |
| `modules/traffic-guard/internal/subject.go` | 手写 | 只在内存选择可信来源；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/classification.go` | 手写 | 选择可信风控动作；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/signals.go` | 手写 | 读取有限匿名信号；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/risk.go` | 手写 | 确定挑战规则；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/bucket.go` | 手写 | 整数补充令牌；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/bucket_store.go` | 手写 | 原子扣频率令牌；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/permit_store.go` | 手写 | 创建持久并发租约；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/permit_sign.go` | 手写 | 签名准入凭证；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/admission.go` | 手写 | 编排频率/挑战/并发；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/http_admission.go` | 手写 | 准入 HTTP；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/sweeper.go` | 手写 | 回收过期并发；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/challenge.go` | 手写 | 签发挑战；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/pow.go` | 手写 | 验证工作量证明；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/captcha.go` | 手写 | 适配外部验证；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/ban_store.go` | 手写 | 管理员建立临时禁令；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/events.go` | 手写 | 处理权限撤销通知；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/outbox.go` | 手写 | 可靠派发风控引用事件；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/retention.go` | 手写 | 清理匿名风控状态；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/import.go` | 手写 | 校验恢复输入；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/question_signals.go` | 手写 | 接收qa已有检测信号；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/session_queue.go` | 手写 | 每会话1并发其余排队；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/admission_consumption.go` | 手写 | QA绑定准入使用权；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/internal/http_verify.go` | 手写 | QA在线确认准入租约；同文件职责限本域以下登记函数 |
| `modules/traffic-guard/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/traffic-guard/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |
| `modules/traffic-guard/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 |
| `modules/traffic-guard/cmd/traffic-guard/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/traffic-guard/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/traffic-guard/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/traffic-guard/go.mod` | 配置 | 独立module nice-wiki/modules/traffic-guard；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/traffic-guard/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/traffic-guard/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/traffic-guard；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/traffic-guard/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/traffic-guard与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/traffic-guard/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/traffic-guard/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/traffic-guard/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/traffic-guard/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/traffic-guard/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/traffic-guard/internal/queries.go` | 手写 | 查看最近可疑事件 |
| `modules/traffic-guard/migrations/0002_login_protection.sql` | 迁移 | 只追加匿名登录attempt、网络关联、滚动失败、固定锁和Finish幂等回执；既有策略/回执不变 |
| `modules/traffic-guard/internal/guard_runtime.go` | 手写 | 本域可信装配与组合短事务、原安全回执和outbox边界 |
| `modules/traffic-guard/migrations/0003_guard_runtime.sql` | 手写 | 只追加匿名作用域、桶、租约、挑战、禁令与安全回执；不持久token/原IP/正文 |
| `modules/traffic-guard/internal/http_management.go` | 手写 | 只供统一服务边界绑定已完成管理/匿名信号API；内部marker复用该次即时授权 |
| `modules/traffic-guard/web/src/index.tsx` | UI | 防滥用：访客提问准入策略、禁令、可疑事件与一键封禁 |
