# traffic-guard 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=50；分类={"handwritten": 32, "ui": 4, "generated": 4, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/traffic-guard/internal/policy.go` | handwritten | 校验风控参数；同文件职责限本域以下登记函数 | `M12-F001` |
| `modules/traffic-guard/internal/policy_store.go` | handwritten | 读取生效策略；同文件职责限本域以下登记函数 | `M12-F002`, `M12-F003` |
| `modules/traffic-guard/internal/subject.go` | handwritten | 只在内存选择可信来源；同文件职责限本域以下登记函数 | `M12-F004`, `M12-F005` |
| `modules/traffic-guard/internal/classification.go` | handwritten | 选择可信风控动作；同文件职责限本域以下登记函数 | `M12-F006` |
| `modules/traffic-guard/internal/signals.go` | handwritten | 读取有限匿名信号；同文件职责限本域以下登记函数 | `M12-F007` |
| `modules/traffic-guard/internal/risk.go` | handwritten | 确定挑战规则；同文件职责限本域以下登记函数 | `M12-F008` |
| `modules/traffic-guard/internal/bucket.go` | handwritten | 整数补充令牌；同文件职责限本域以下登记函数 | `M12-F009` |
| `modules/traffic-guard/internal/bucket_store.go` | handwritten | 原子扣频率令牌；同文件职责限本域以下登记函数 | `M12-F010` |
| `modules/traffic-guard/internal/permit_store.go` | handwritten | 创建持久并发租约；同文件职责限本域以下登记函数 | `M12-F011`, `M12-F016`, `M12-F017` |
| `modules/traffic-guard/internal/permit_sign.go` | handwritten | 签名准入凭证；同文件职责限本域以下登记函数 | `M12-F012`, `M12-F013` |
| `modules/traffic-guard/internal/admission.go` | handwritten | 编排频率/挑战/并发；同文件职责限本域以下登记函数 | `M12-F014` |
| `modules/traffic-guard/internal/http_admission.go` | handwritten | 准入 HTTP；同文件职责限本域以下登记函数 | `M12-F015` |
| `modules/traffic-guard/internal/sweeper.go` | handwritten | 回收过期并发；同文件职责限本域以下登记函数 | `M12-F018`, `M12-F027` |
| `modules/traffic-guard/internal/challenge.go` | handwritten | 签发挑战；同文件职责限本域以下登记函数 | `M12-F019`, `M12-F022` |
| `modules/traffic-guard/internal/pow.go` | handwritten | 验证工作量证明；同文件职责限本域以下登记函数 | `M12-F020` |
| `modules/traffic-guard/internal/captcha.go` | handwritten | 适配外部验证；同文件职责限本域以下登记函数 | `M12-F021` |
| `modules/traffic-guard/internal/ban.go` | handwritten | 读取作用域禁令；同文件职责限本域以下登记函数 | `M12-F023` |
| `modules/traffic-guard/internal/ban_store.go` | handwritten | 管理员建立临时禁令；同文件职责限本域以下登记函数 | `M12-F024`, `M12-F025`, `M12-F026` |
| `modules/traffic-guard/internal/events.go` | handwritten | 处理权限撤销通知；同文件职责限本域以下登记函数 | `M12-F028` |
| `modules/traffic-guard/internal/outbox.go` | handwritten | 可靠派发风控引用事件；同文件职责限本域以下登记函数 | `M12-F029` |
| `modules/traffic-guard/internal/retention.go` | handwritten | 清理匿名风控状态；同文件职责限本域以下登记函数 | `M12-F030` |
| `modules/traffic-guard/internal/barrier.go` | handwritten | 停止准入并等安全点；同文件职责限本域以下登记函数 | `M12-F031`, `M12-F032` |
| `modules/traffic-guard/internal/snapshot.go` | handwritten | 导出风控一致快照；同文件职责限本域以下登记函数 | `M12-F033` |
| `modules/traffic-guard/internal/import.go` | handwritten | 校验恢复输入；同文件职责限本域以下登记函数 | `M12-F034`, `M12-F035`, `M12-F036`, `M12-F037` |
| `modules/traffic-guard/internal/recovery_authorization.go` | handwritten | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `M12-F038` |
| `modules/traffic-guard/web/src/PolicyEditor.vue` | ui | Vue3模块交互：SaveGuardPolicy；同文件职责限本域以下登记函数 | `M12-F039` |
| `modules/traffic-guard/web/src/BanList.vue` | ui | Vue3模块交互：SubmitBan；同文件职责限本域以下登记函数 | `M12-F040` |
| `modules/traffic-guard/web/src/client.ts` | ui | Vue3模块交互：RunPoWChallenge；同文件职责限本域以下登记函数 | `M12-F041` |
| `modules/traffic-guard/internal/question_signals.go` | handwritten | 接收qa已有检测信号；同文件职责限本域以下登记函数 | `M12-F042` |
| `modules/traffic-guard/internal/session_queue.go` | handwritten | 每会话1并发其余排队；同文件职责限本域以下登记函数 | `M12-F043` |
| `modules/traffic-guard/internal/admission_consumption.go` | handwritten | QA绑定准入使用权；同文件职责限本域以下登记函数 | `M12-F044` |
| `modules/traffic-guard/internal/http_verify.go` | handwritten | QA在线确认准入租约；同文件职责限本域以下登记函数 | `M12-F045` |
| `modules/traffic-guard/internal/types.gen.go` | generated | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module traffic-guard --target go-types，输出不得手改 |  |
| `modules/traffic-guard/internal/http_bindings.gen.go` | generated | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module traffic-guard --target go-http，x-handler映射仅转调手写函数 |  |
| `modules/traffic-guard/config/defaults.json` | config | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/traffic-guard/migrations/0001_domain.sql` | migration | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |  |
| `modules/traffic-guard/cmd/traffic-guard/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M12-F901` |
| `modules/traffic-guard/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M12-F902` |
| `modules/traffic-guard/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M12-F903`, `M12-F904`, `M12-F905`, `M12-F906` |
| `modules/traffic-guard/go.mod` | config | 独立module nice-wiki/modules/traffic-guard；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/traffic-guard/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/traffic-guard/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/traffic-guard；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/traffic-guard/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/traffic-guard与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/traffic-guard/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/traffic-guard/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/traffic-guard/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/traffic-guard/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/traffic-guard/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/traffic-guard/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/traffic-guard/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M12-F907` |
