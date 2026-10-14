# traffic-guard 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 50 个：手写 32、UI 4、生成 4、配置 9、迁移 1。函数 53 个：约束 31，参考 22。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/traffic-guard/internal/policy.go` | 手写 | 校验风控参数；同文件职责限本域以下登记函数 | `traffic-guard.ValidatePolicy`（参考） |
| `modules/traffic-guard/internal/policy_store.go` | 手写 | 读取生效策略；同文件职责限本域以下登记函数 | `traffic-guard.GetPolicy`、`traffic-guard.UpdatePolicy` |
| `modules/traffic-guard/internal/subject.go` | 手写 | 只在内存选择可信来源；同文件职责限本域以下登记函数 | `traffic-guard.NormalizePeer`（参考）、`traffic-guard.DeriveSubject`（参考） |
| `modules/traffic-guard/internal/classification.go` | 手写 | 选择可信风控动作；同文件职责限本域以下登记函数 | `traffic-guard.ClassifyAction`（参考） |
| `modules/traffic-guard/internal/signals.go` | 手写 | 读取有限匿名信号；同文件职责限本域以下登记函数 | `traffic-guard.LoadSignals`（参考） |
| `modules/traffic-guard/internal/risk.go` | 手写 | 确定挑战规则；同文件职责限本域以下登记函数 | `traffic-guard.ComputeRisk`（参考） |
| `modules/traffic-guard/internal/bucket.go` | 手写 | 整数补充令牌；同文件职责限本域以下登记函数 | `traffic-guard.RefillBucket`（参考） |
| `modules/traffic-guard/internal/bucket_store.go` | 手写 | 原子扣频率令牌；同文件职责限本域以下登记函数 | `traffic-guard.ConsumeBuckets`（参考） |
| `modules/traffic-guard/internal/permit_store.go` | 手写 | 创建持久并发租约；同文件职责限本域以下登记函数 | `traffic-guard.AcquireConcurrency`（参考）、`traffic-guard.RenewPermit`、`traffic-guard.ReleasePermit` |
| `modules/traffic-guard/internal/permit_sign.go` | 手写 | 签名准入凭证；同文件职责限本域以下登记函数 | `traffic-guard.SignPermit`（参考）、`traffic-guard.ValidatePermit`（参考） |
| `modules/traffic-guard/internal/admission.go` | 手写 | 编排频率/挑战/并发；同文件职责限本域以下登记函数 | `traffic-guard.EvaluateAdmission`（参考） |
| `modules/traffic-guard/internal/http_admission.go` | 手写 | 准入 HTTP；同文件职责限本域以下登记函数 | `traffic-guard.HandleAdmission` |
| `modules/traffic-guard/internal/sweeper.go` | 手写 | 回收过期并发；同文件职责限本域以下登记函数 | `traffic-guard.ExpireLeases`（参考）、`traffic-guard.ExpireBans`（参考） |
| `modules/traffic-guard/internal/challenge.go` | 手写 | 签发挑战；同文件职责限本域以下登记函数 | `traffic-guard.IssueChallenge`、`traffic-guard.VerifyChallenge` |
| `modules/traffic-guard/internal/pow.go` | 手写 | 验证工作量证明；同文件职责限本域以下登记函数 | `traffic-guard.VerifyPoW`（参考） |
| `modules/traffic-guard/internal/captcha.go` | 手写 | 适配外部验证；同文件职责限本域以下登记函数 | `traffic-guard.VerifyExternalChallenge`（参考） |
| `modules/traffic-guard/internal/ban.go` | 手写 | 读取作用域禁令；同文件职责限本域以下登记函数 | `traffic-guard.CheckBan`（参考） |
| `modules/traffic-guard/internal/ban_store.go` | 手写 | 管理员建立临时禁令；同文件职责限本域以下登记函数 | `traffic-guard.CreateBan`、`traffic-guard.RevokeBan`、`traffic-guard.ListBans` |
| `modules/traffic-guard/internal/events.go` | 手写 | 处理权限撤销通知；同文件职责限本域以下登记函数 | `traffic-guard.ConsumeIdentityEvent`、`traffic-guard.ConsumeEvents` |
| `modules/traffic-guard/internal/outbox.go` | 手写 | 可靠派发风控引用事件；同文件职责限本域以下登记函数 | `traffic-guard.PublishGuardEvents` |
| `modules/traffic-guard/internal/retention.go` | 手写 | 清理匿名风控状态；同文件职责限本域以下登记函数 | `traffic-guard.PurgeAnonymousSignals`（参考） |
| `modules/traffic-guard/internal/barrier.go` | 手写 | 停止准入并等安全点；同文件职责限本域以下登记函数 | `traffic-guard.EnterBarrier`、`traffic-guard.ReleaseBarrier` |
| `modules/traffic-guard/internal/snapshot.go` | 手写 | 导出风控一致快照；同文件职责限本域以下登记函数 | `traffic-guard.CreateSnapshot` |
| `modules/traffic-guard/internal/import.go` | 手写 | 校验恢复输入；同文件职责限本域以下登记函数 | `traffic-guard.ValidateImport`、`traffic-guard.PrepareImport`、`traffic-guard.ActivateImport`、`traffic-guard.AbortImport` |
| `modules/traffic-guard/internal/recovery_authorization.go` | 手写 | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `traffic-guard.ValidateRecoveryAuthorization`（参考） |
| `modules/traffic-guard/web/src/PolicyEditor.vue` | UI | Vue3模块交互：SaveGuardPolicy；同文件职责限本域以下登记函数 | `traffic-guard.SaveGuardPolicy`（参考） |
| `modules/traffic-guard/web/src/BanList.vue` | UI | Vue3模块交互：SubmitBan；同文件职责限本域以下登记函数 | `traffic-guard.SubmitBan`（参考） |
| `modules/traffic-guard/web/src/client.ts` | UI | Vue3模块交互：RunPoWChallenge；同文件职责限本域以下登记函数 | `traffic-guard.RunPoWChallenge`（参考） |
| `modules/traffic-guard/internal/question_signals.go` | 手写 | 接收qa已有检测信号；同文件职责限本域以下登记函数 | `traffic-guard.ReportQuestionSignals` |
| `modules/traffic-guard/internal/session_queue.go` | 手写 | 每会话1并发其余排队；同文件职责限本域以下登记函数 | `traffic-guard.QueueSessionAdmission` |
| `modules/traffic-guard/internal/admission_consumption.go` | 手写 | QA绑定准入使用权；同文件职责限本域以下登记函数 | `traffic-guard.ConsumeAdmission` |
| `modules/traffic-guard/internal/http_verify.go` | 手写 | QA在线确认准入租约；同文件职责限本域以下登记函数 | `traffic-guard.VerifyAdmissionHTTP` |
| `modules/traffic-guard/internal/types.gen.go` | 生成 | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module traffic-guard --target go-types，输出不得手改 | 无 |
| `modules/traffic-guard/internal/http_bindings.gen.go` | 生成 | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module traffic-guard --target go-http，x-handler映射仅转调手写函数 | 无 |
| `modules/traffic-guard/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/traffic-guard/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 | 无 |
| `modules/traffic-guard/cmd/traffic-guard/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `traffic-guard.main` |
| `modules/traffic-guard/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `traffic-guard.BuildApplication` |
| `modules/traffic-guard/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `traffic-guard.Serve`、`traffic-guard.Shutdown`、`traffic-guard.Health`、`traffic-guard.Ready` |
| `modules/traffic-guard/go.mod` | 配置 | 独立module nice-wiki/modules/traffic-guard；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/traffic-guard/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/traffic-guard/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/traffic-guard；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/traffic-guard/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/traffic-guard与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/traffic-guard/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/traffic-guard/web/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/traffic-guard/web/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/traffic-guard/web/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/traffic-guard/web/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/traffic-guard/web/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/traffic-guard/web/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `traffic-guard.mountModuleUI` |
