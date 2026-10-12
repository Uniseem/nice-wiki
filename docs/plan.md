# 实施计划

更新于 2026-10-12。已开始实现；P0-01～P0-03 已完成，其余任务按完整验收标准继续核对，不能把部分实现视为完成。

阶段顺序按 [D52](adr/0052-phase-order.md)：工程准备 → 地基 → 后台与编辑器 → 公开站点 → AI 问答与长期记忆 → 多语言 → 收尾。功能依据 [设计文档](../README.md#设计文档)，文件职责依据 [文件结构契约](structure/README.md)。遇到旧草案和后续决策不一致时，以后续 ADR 和已确认的结构约定为准；仍有冲突时先记录并确认，不在实施中悄悄改变设计。

## 执行约定

- 编号 `P阶段-序号` 在提交信息、`STATUS.md`、push-log 中共用。任务完成时把 `[ ]` 改为 `[x]`；正式 push 后补记“完成于 push NNNN”，尚未 push 时不虚填编号。
- 依赖列出直接前置任务，其传递依赖不重复列出。每阶段还依赖上一阶段的全部任务完成；阶段内按依赖推进。任务按一到两次会话的目标拆分，实际过大时先更新计划再继续。
- 每个任务的完成标准都是：功能达到本任务验收标准、相关文档同步、`make check` 通过。第 0 阶段检查入口尚未建立时，运行当时已有的格式、编译和隐私检查；P0-04 完成后统一使用 `make check`。
- **测试越少越好，非必要不测试。** 每项“建议测试：无”表示不新增测试；少数高风险任务只提出最小建议，均须另获用户同意才编写或修改。验收标准描述应具备的行为，不代表授权运行端到端、真实接口或 AI 评测；不设测试阶段、覆盖率目标或全量快照要求。已有检查和已有普通测试照常运行，不为通过检查而改、删、跳过已有测试（[D38](adr/0038-testing.md)）。
- 普通测试若获准，使用临时目录、SQLite 和外部服务的假实现；不调用真实模型。端到端测试、`LIVE=1`、`make eval` 均另行确认。没有必要时不创建 `fixtures/`、`evals/`、快照和测试专用文件。
- 文件路径均相对仓库根目录；`目录/{a,b}` 表示该目录下分别列出的文件，不授权新增其他文件。结构外的文件、目录及职责变更按 D45 先确认。生成文件只改来源再生成。
- 新增或修改接口的任务同时涉及 `api/openapi.yaml`、`server/internal/api/{server.go,permissions.go,errors.go}`、`server/internal/api/gen/`、`web/src/shared/api/schema.gen.ts`；按需更新 `server/internal/app/{app.go,http.go,lifecycle.go}` 和 `web/src/admin/router.ts`。这些公共文件不在每项重复列出，但属于相应任务范围。每个后台接口必须声明权限，前端隐藏入口不能替代服务端检查。
- 数据库变更同时更新对应 `server/internal/store/queries/*.sql` 并重新生成 `server/internal/store/sqlc/`。迁移只追加，不修改已执行的迁移；包间依赖遵守 `structure/server.md`，跨业务协作通过接口与 `app` 注入。
- 每次会话结束更新任务状态、`STATUS.md` 和 `docs/push-log/_pending.md`。提交署名与隐私检查遵守 D41、D42；代码仓库的 push 仍须用户每次明确触发。

## 阶段衔接

| 阶段 | 任务 | 阶段交付 |
|---|---|---|
| 0 工程准备 | P0-01～P0-04 | 约定、工程骨架、生成流程、统一检查与提交钩子 |
| 1 地基 | P1-01～P1-10 | 可启动的服务、文档格式、基础渲染、数据存储、账号与权限 |
| 2 后台与编辑器 | P2-01～P2-15 | 能编辑、预览、发布和管理完整的第一版块类型 |
| 3 公开站点 | P3-01～P3-06 | 可先上线仅基准语言、未启用 AI 的文档站 |
| 4 AI 问答 | P4-01～P4-22 | 模型接入、长期记忆、基准语言索引、受保护的公开问答 |
| 5 多语言 | P5-01～P5-10 | 全站增量翻译、语言切换、多语言问答与缓存 |
| 6 收尾 | P6-01～P6-07 | 统计、仪表盘、备份恢复、容器与部署说明 |

为避免后续功能阻塞前面的交付，先落实以下边界：

1. 语言代码、块 ID、文档格式版本、语言前缀和逻辑方向 CSS 从地基阶段采用最终结构；第 5 阶段才开放目标语言。
2. 第 4 阶段前，管理员手动填写合法英文路径名及 SEO 描述；缺少路径名时提示补填，不调用尚未实现的模型。P4-09 接入首次发布的自动生成，保留手动内容。
3. 初始化向导仍要求登记至少一个渠道。P2-02 只做首个渠道配置和密钥保存，不发送模型请求；第 4 阶段补齐目录、调用、档位和完整管理界面。快速、精确、嵌入档未就绪时，AI 功能关闭，基础文档管理照常可用。
4. P4-08 为记忆助手提前实现术语表的最小读写，翻译流水线和术语抽取仍在第 5 阶段；初始化向导的备份恢复入口在 P6-04 接通，此前不显示可执行入口。
5. 后台界面固定中文。公开页面以 Go 输出完整 HTML，轻交互用原生 TypeScript；只有问答和语言建议使用 Vue。第 3 阶段不展示尚不可用的 AI 入口。

## 第 0 阶段：工程准备

依据：[engineering.md](design/engineering.md)、D36～D43、D45、D51。

### [x] P0-01 助手约定与会话记录

- **完成记录**：完成于 push 0007。

- **任务**：建立分层助手说明、当前状态和重复操作手册；把测试决定权、文件结构、隐私、署名、push 规则写成硬性约定。
- **依赖**：无。
- **涉及文件**：`AGENTS.md`、`CLAUDE.md`、`STATUS.md`、`README.md`、`THIRD_PARTY_NOTICES.md`；`server/{AGENTS.md,CLAUDE.md}`、`web/{AGENTS.md,CLAUDE.md}`；`docs/playbooks/{add-block.md,add-api.md,add-migration.md,add-language.md,add-video-provider.md,add-llm-protocol.md,add-prompt.md,push.md,change-structure.md}`；`docs/push-log/_pending.md`。
- **验收标准**：三份 `CLAUDE.md` 都只有 `@AGENTS.md`；说明文件能引导助手定位任务，不重复长篇设计；手册不会默认要求新增测试；状态快照控制在一屏内；已有借用许可与新项目需确认的边界清楚。
- **建议测试**：无，检查内容与链接即可。

### [x] P0-02 前后端工程骨架

- **完成记录**：完成于 push 0007。

- **任务**：搭建 Go module、pnpm/Vite 双入口、最小后台入口及公开页面脚本、共用样式，固定已选技术栈。
- **依赖**：P0-01。
- **涉及文件**：`server/{go.mod,go.sum,.golangci.yml}`、`server/cmd/nice-wiki/main.go`、`server/internal/app/{app.go,http.go,lifecycle.go}`、`server/internal/webui/dist/.gitkeep`；`web/{package.json,pnpm-lock.yaml,vite.config.ts,tsconfig.json,tsconfig.app.json,tsconfig.node.json,eslint.config.js,components.json,env.d.ts}`、`web/admin/index.html`、`web/src/admin/{main.ts,App.vue,router.ts,admin.css}`、`web/src/site/{main.ts,site.css}`、`web/src/shared/styles/{tokens.css,base.css,content.css,code-site.css,code-editor.css}`；`Makefile`、`.gitignore`、`.gitattributes`、`.editorconfig`。
- **验收标准**：Go module 为 `nice-wiki`，TypeScript 按 D18 固定 6.0.x；双入口能构建到约定的 Go 嵌入目录并生成 manifest；后台和公开入口互不导入，公开入口不带编辑器依赖；样式从一开始用逻辑方向属性；运行时数据、依赖和构建产物不入库。
- **建议测试**：无，编译、lint、类型检查足够；不为了空工程安装测试工具或新增 shadcn-vue 组件。

### [x] P0-03 OpenAPI 与数据库代码生成

- **完成记录**：完成于 push 0007。

- **任务**：建立接口定义、统一错误类型和生成流程，配置 sqlc；后续任务按领域扩充契约。
- **依赖**：P0-02。
- **涉及文件**：`api/openapi.yaml`、`server/{sqlc.yaml,oapi-codegen.yaml}`、`server/internal/api/{server.go,permissions.go,errors.go}`、`server/internal/api/gen/`、`server/internal/store/sqlc/`、`web/src/shared/api/{schema.gen.ts,client.ts}`、`Makefile`。
- **验收标准**：`make gen` 能生成两端 API 类型；Go 路由目标为标准库 `net/http`；前端通过 openapi-fetch 客户端调用，权限名称来自契约；重复生成无差异；sqlc 配置先就绪，待 P1-02 和首批业务查询存在后启用 SQL 生成，此前明确报告尚未启用，不用忽略错误的方式通过检查，也不创建无用途的 SQL 或测试文件。
- **建议测试**：无，生成一致性和两端编译足够。

### [x] P0-04 隐私扫描、钩子与统一检查

- **完成记录**：已完成，尚未 push。

- **任务**：实现暂存区及待推送提交的隐私扫描，接入 git 钩子，完成 `make check`、开发和构建命令。
- **依赖**：P0-02、P0-03。
- **涉及文件**：`server/tools/privacycheck/main.go`、`.githooks/{pre-commit,commit-msg,pre-push}`、`Makefile`、`README.md`、`docs/playbooks/push.md`。
- **验收标准**：扫描文件、提交信息、作者与提交者信息；遵守 D42 的示例及署名例外；钩子不主动提交或 push；`make check` 覆盖格式、lint、类型、已有普通测试、生成一致性、隐私，不调用 E2E、真实服务或 AI 评测；无测试时能正常通过。
- **建议测试**：无，先使用规则审阅和已有检查；发现漏检后再提出针对性回归建议。

## 第 1 阶段：地基

依据：[architecture.md](design/architecture.md)、[accounts.md](design/accounts.md)、[security-ops.md](design/security-ops.md)、D5、D8、D14～D16、D18、D20、D35。

### [x] P1-01 数据目录、原子写入与配置

- **完成记录**：已完成，尚未 push。

- **任务**：建立数据布局、首次启动初始化、进程锁和配置分层，接入启动与优雅关闭。
- **依赖**：P0-04。
- **涉及文件**：`server/internal/fsutil/atomic.go`、`server/internal/datadir/{layout.go,init.go,lock.go}`、`server/internal/config/{bootstrap.go,runtime.go,secrets.go}`、`server/cmd/nice-wiki/main.go`。
- **验收标准**：所有数据路径由 `layout.go` 定义；数据目录可指定，重复启动不会覆盖现有数据；同目录不能同时启动两个进程；配置与文档原子写入；密钥不能从接口读出；启动参数、环境变量和配置文件的优先级明确。
- **建议测试**：无；原子写与恢复的故障场景集中在 P2-04、P6-04 的少量建议中考虑。

### [x] P1-02 SQLite 与初始迁移

- **完成记录**：已完成，尚未 push。

- **任务**：接入纯 Go SQLite、WAL 和事务，建立结构契约列出的初始表；后续阶段按需补查询。
- **依赖**：P1-01、P0-03。
- **涉及文件**：`server/internal/store/{db.go,migrate.go,tx.go}`、`server/internal/store/migrations/{0001_auth.sql,0002_content.sql,0003_jobs.sql,0004_search.sql,0005_qa.sql,0006_llm.sql,0007_guard.sql,0008_i18n.sql,0009_stats.sql,0010_memory.sql}`。
- **验收标准**：初始迁移按编号执行并记录版本，重复启动不重复执行；失败不带着半完成的迁移继续启动；仅建立表结构，不提前实现后续业务；Backlite 的任务表由库创建；文件内容和可重建索引的边界清楚。
- **建议测试**：无，先依赖迁移执行、生成与编译检查；不为每张表写 CRUD 测试。

### [x] P1-03 文档格式、块 ID 与版本迁移

- **完成记录**：已完成，尚未 push。

- **任务**：定义前后端共用的文档结构、校验、遍历、拆块和内容哈希，预先声明第一版全部节点属性。
- **依赖**：P1-01、P0-02。
- **涉及文件**：`server/internal/doc/{schema.go,parse.go,migrate.go,walk.go,blocks.go}`、`server/internal/doc/nodes/registry.go`、`server/internal/textutil/{normalize.go,hash.go}`；`web/src/shared/tiptap/{schema.ts,extensions.ts}`、`web/src/shared/tiptap/nodes/{callout.ts,columns.ts,image.ts,attachment.ts,drawing.ts,video.ts,heading.ts,internal-link.ts}`。
- **验收标准**：Go 和 TypeScript 的格式版本、节点名、属性一致；块 ID 稳定，复制新块生成新 ID；拒绝缺失或重复 ID、非法嵌套、未知格式版本；正文只允许 H2～H4；旧格式逐级迁移，后续格式变化有明确入口。
- **建议测试**：无，安全渲染与格式反例可合并到 P1-04 的最小建议，避免另建一套样例。

### [x] P1-04 基础块的三种渲染

- **完成记录**：已完成，尚未 push。

- **任务**：完成 HTML、纯文本、Markdown 入口及基础块、行内格式、代码高亮；复杂块在第 2 阶段逐个接入。
- **依赖**：P1-03。
- **涉及文件**：`server/internal/doc/{marks.go,links.go,safeurl.go,highlight.go,render_html.go,render_text.go,render_md.go}`、`server/internal/doc/nodes/{registry.go,paragraph.go,heading.go,blockquote.go,divider.go,list.go,tasklist.go,table.go,codeblock.go,math.go}`；`web/src/shared/styles/{content.css,code-site.css,code-editor.css}`。
- **验收标准**：基础块三种输出齐全，共用 `nw-` 类名；HTML 文本转义、危险 URL 拒绝；标题及块锚点稳定；代码与公式保留源码，翻译提取能识别不可翻译内容；不增加标题折叠。
- **建议测试**：建议仅一组表驱动反例，覆盖文本转义、危险链接、重复 ID 与非法嵌套；暂不做每块快照或全量前后端一致性测试。

### [x] P1-05 正文文件、文档树与路径规则

- **完成记录**：已完成，尚未 push。

- **任务**：实现元数据、正文文件读写、分数索引排序、父子移动、路径名校验与旧路径记录。
- **依赖**：P1-02、P1-03。
- **涉及文件**：`server/internal/fracindex/fracdex.go`、`server/internal/content/{docfile.go,repo.go,tree.go,slugs.go,redirects.go,index.go,generators.go}`、`server/internal/store/queries/content.sql`、`THIRD_PARTY_NOTICES.md`。
- **验收标准**：正文文件是真实数据；同级路径名唯一，完整路径按树拼接且不含语言；禁止循环移动；排序只改被移动文档的排序元数据；父文档移动时记录所有受影响的旧路径，跳转目标存 ID；缺失的文档/块索引可从文件重建，不擅自重算付费 AI 数据。
- **建议测试**：无，发布、移动及跳转的一致性集中在 P2-04 考虑。

### [x] P1-06 站点设置与基准语言

- **完成记录**：已完成，尚未 push。

- **任务**：建立站点、首页、公告、语言和 AI 设置的文件读写及默认值，提供初始化状态判断。
- **依赖**：P1-01、P1-03。
- **涉及文件**：`server/internal/settings/{store.go,site.go,home.go,banner.go,languages.go,ai.go}`、`server/internal/setup/state.go`、`server/internal/i18n/languages.go`、`web/src/shared/i18n/{messages.zh-Hans.json,t.ts}`。
- **验收标准**：设置存入约定文件，密钥和系统配置不混入站点设置；语言使用内置规范代码（如 `zh-Hans`），URL 从一开始保留语言前缀；此时仅基准语言启用；后台不做多语言；模型和翻译未就绪时不投递无处理函数的任务。
- **建议测试**：无，设置表单校验和类型检查足够。

### [x] P1-07 data 版本仓库与任务队列

- **完成记录**：已完成，尚未 push。

- **任务**：初始化 go-git 版本仓库、发布提交入口和 Backlite 队列，建立任务名称、去重、重试、进度及定时投递基础。
- **依赖**：P1-01、P1-02、P1-05、P1-06。
- **涉及文件**：`server/internal/history/{repo.go,commit.go,log.go}`、`server/internal/jobs/{kinds.go,queue.go,worker.go,schedule.go}`、`server/internal/store/queries/jobs.sql`。
- **验收标准**：data 仓库只纳入结构约定的内容文件（含后续记忆），配置、草稿、资源、数据库不进入；不依赖系统 git；队列持久化、同文档重复任务合并、失败有进度与重试信息；未实现任务不投递；`jobs` 通过注入获得数据库能力，不违反包分层。
- **建议测试**：无，队列使用成熟库，不重复测试库的重试实现；业务发布衔接见 P2-04。

### [x] P1-08 注册、登录与会话

- **完成记录**：已完成，尚未 push。

- **任务**：实现首个超管注册、argon2id 密码、登录保护、会话、强制改密码及超管重置文件。
- **依赖**：P1-02、P1-06。
- **涉及文件**：`server/internal/auth/{accounts.go,password.go,register.go,login.go,sessions.go,middleware.go,reset.go}`、`server/internal/store/queries/auth.sql`、`server/internal/api/{admin_setup.go,admin_auth.go}`。
- **验收标准**：首个账号注册原子化，之后关闭注册；唯一超管不能停用或删除，账号名不可修改；初始密码只放行改密码；改密码使其他设备会话失效；失败次数限制同时覆盖账号和 IP；重置文件在启动和运行中被检测，成功后删除；cookie 与 Origin 校验按设计执行。
- **建议测试**：不另建测试，合并到 P1-09 的权限与会话反例建议。

### [x] P1-09 功能权限、账号管理与操作日志

- **完成记录**：已完成，尚未 push。

- **任务**：落实十项权限、超管专属操作、账号管理和操作日志；建立路由权限声明的唯一入口。
- **依赖**：P1-08。
- **涉及文件**：`server/internal/auth/{accounts.go,permissions.go,middleware.go,audit.go}`、`server/internal/api/{permissions.go,admin_accounts.go,admin_audit.go}`、`server/internal/store/queries/auth.sql`。
- **验收标准**：十项权限按功能分配，新账号默认全选；模型渠道、档位/任务分配、系统配置等超管能力不能授权给普通账号；停用/删除立即注销全部会话；操作日志保留历史账号名且只有超管可读；匿名注册、登录等例外也显式声明策略。
- **建议测试**：建议仅一组权限反例：遍历路由查漏声明，并覆盖普通账号访问超管接口、强制改密码绕过、停用后旧会话；不为每个 CRUD 分别写测试。

### [x] P1-10 HTTP 基础与前端嵌入

- **完成记录**：已完成，尚未 push。

- **任务**：组装基础路由、安全头、可信代理、日志、异常恢复、健康接口和前端产物读取。
- **依赖**：P1-01、P1-07、P1-09、P0-02。
- **涉及文件**：`server/internal/httpx/{realip.go,security.go,logging.go,recover.go,errors.go}`、`server/internal/webui/{embed.go,manifest.go,admin.go}`、`server/internal/site/{routes.go,langroute.go}`。
- **验收标准**：`/admin/` 子路径返回后台入口，开发模式正确引用 Vite；生产二进制内嵌前端；只有可信代理的来源头能影响真实 IP；请求日志不记录提问或密钥；公开路由预留最终语言结构；`/healthz` 可供部署检查，关闭流程释放文件锁和队列。
- **建议测试**：无；权限反例复用 P1-09，真实代理部署不在本阶段自动测试。

## 第 2 阶段：后台与编辑器

依据：[editor.md](design/editor.md)、[admin.md](design/admin.md)、[accounts.md](design/accounts.md)、[home.md](design/home.md)、D4、D9、D15、D24、D26、D30、D35、D51、D54。

### [x] P2-01 后台外框、登录与账号页面

- **完成记录**：已完成，尚未 push。

- **任务**：实现后台导航、登录/注册/改密码、个人页、账号与审计页面，按权限控制入口。
- **依赖**：P1-09、P1-10。
- **涉及文件**：`web/src/admin/{App.vue,main.ts,router.ts}`、`web/src/admin/stores/auth.ts`、`web/src/admin/layouts/{AdminLayout.vue,AuthLayout.vue}`、`web/src/admin/composables/usePermission.ts`、`web/src/admin/components/{PageHeader.vue,ConfirmDialog.vue}`、`web/src/admin/views/auth/{RegisterView.vue,LoginView.vue,ChangePasswordView.vue}`、`web/src/admin/views/system/{AccountsView.vue,AuditLogView.vue}`、`web/src/admin/views/{ProfileView.vue,NotFoundView.vue}`、`web/src/shared/{api/client.ts,lib/utils.ts}`。
- **验收标准**：401 与强制改密码正确跳转；无权限菜单隐藏且直接访问仍被后端拒绝；注册只在无账号时显示；个人页可改密码与退出所有设备；文字为中文，组件与样式遵守工程约定。
- **建议测试**：无，类型检查与现有权限检查足够；新增 UI 组件需先按 D45 确认具体文件。

### [x] P2-02 初始化向导与首个渠道登记

- **完成记录**：已完成，尚未 push。

- **任务**：接通注册后的站点信息、基准语言、首个模型渠道配置步骤，使未启用 AI 的基础站能完成初始化。
- **依赖**：P2-01、P1-06。
- **涉及文件**：`server/internal/setup/state.go`、`server/internal/llm/channels.go`、`server/internal/api/{admin_setup.go,admin_models.go}`；`web/src/admin/views/setup/SetupWizardView.vue`、`web/src/admin/views/setup/steps/{SiteStep.vue,LanguageStep.vue,ChannelStep.vue}`。
- **验收标准**：初期可登记自定义兼容 OpenAI 渠道的名称、地址、协议与密钥，配置保存可续接；登记仅超管可操作，不回传密钥，不发送模型请求；至少一个渠道登记后可结束向导；未配置档位时明确显示 AI 未启用，不阻塞文档管理；恢复入口等待 P6-04。
- **建议测试**：无，不运行连接探测；权限沿用 P1-09。

### [x] P2-03 草稿自动保存与编辑锁

- **完成记录**：已完成，尚未 push。

- **任务**：实现共享草稿、版本冲突、锁心跳、超时及超管强制释放，接通编辑器的保存状态。
- **依赖**：P1-05、P1-09、P2-01。
- **涉及文件**：`server/internal/content/{drafts.go,locks.go}`、`server/internal/api/admin_docs.go`、`web/src/admin/composables/{useAutosave.ts,useEditLock.ts}`、`web/src/admin/components/LockNotice.vue`、`web/src/admin/editor/DocEditor.vue`。
- **验收标准**：每文档一份草稿，版本不一致时拒绝覆盖；自动保存不触发翻译或索引模型调用；锁在内存中，超过 2 分钟未心跳释放；长时间无输入先提醒再释放；锁失效或被强制释放后切只读；保存失败显示未保存状态并保留本地编辑内容。
- **建议测试**：建议仅覆盖旧版本保存被拒绝、失锁后不能写入两类反例；锁超时如需测试使用 `testing/synctest`，不增加前端快照。

### [x] P2-04 发布、文档属性与登录预览

- **完成记录**：已完成，尚未 push。

- **任务**：把草稿发布为正文，更新块索引和 data git 历史，提供仅登录可看的草稿预览及元数据编辑。
- **依赖**：P2-03、P1-04、P1-07。
- **涉及文件**：`server/internal/content/{publish.go,index.go,generators.go}`、`server/internal/history/commit.go`、`server/internal/api/admin_docs.go`、`server/internal/site/preview.go`、`server/internal/site/templates/{layout.html,doc.html,partials/head.html}`；`web/src/admin/editor/DocMetaPanel.vue`、`web/src/admin/views/docs/{DocsView.vue,DocPreviewView.vue}`。
- **验收标准**：发布权限独立于编辑权限；手动路径名和 SEO 描述可用，缺少路径名时提示补填；访客只读已发布正文；预览需登录且带 `noindex`；提交作者为发布账号；索引和版本记录与正文一致；失败不假报成功，重试不会造成重复后续任务，恢复办法有记录。
- **建议测试**：建议仅一组临时 data 目录的发布一致性测试，覆盖正文/索引/git 某一步失败后的恢复与重试，以及父文档移动后的旧路径；不写每种发布按钮的 E2E。

### [x] P2-05 文档树与基础编辑交互

- **任务**：接通拖拽排序/移动、基础块、稳定 ID、块菜单、斜杠菜单、浮动格式条、站内链接和粘贴转换。
- **依赖**：P2-03、P2-04、P1-03。
- **涉及文件**：`web/src/admin/stores/docTree.ts`、`web/src/admin/components/{DocTree.vue,DocPicker.vue}`、`web/src/admin/editor/{DocEditor.vue,extensions.ts,paste.ts}`、`web/src/admin/editor/menus/{SlashMenu.vue,slash-items.ts,BubbleMenu.vue,BlockMenu.vue,LinkPopover.vue}`、`web/src/shared/lib/fractional-indexing.ts`、`web/src/shared/tiptap/nodes/internal-link.ts`、`THIRD_PARTY_NOTICES.md`。
- **验收标准**：拖拽结果与 Go 分数索引一致；Markdown 快捷输入可用；粘贴清理多余格式，不构成导入功能；站内链接存文档 ID 与可选块 ID；转换类型保留适当 ID，复制块不重复 ID；不增加块引用、反向链接或实时协作。
- **建议测试**：无，不测 Tiptap 自带输入规则；只有出现具体编辑 bug 时再提回归建议。

### [x] P2-06 提示框、表格、代码与公式

- **任务**：完善对应编辑交互和三种渲染，统一样式及翻译提取行为。
- **依赖**：P2-05、P1-04。
- **涉及文件**：`web/src/shared/tiptap/{extensions.ts,nodes/callout.ts}`、`web/src/admin/editor/{extensions.ts,menus/slash-items.ts}`、`web/src/admin/editor/nodeviews/{CalloutView.vue,CodeBlockView.vue,MathView.vue}`、`server/internal/doc/nodes/{callout.go,table.go,codeblock.go,math.go,registry.go}`、`web/src/shared/styles/{content.css,code-site.css,code-editor.css}`、`THIRD_PARTY_NOTICES.md`。
- **验收标准**：四类提示框可切换，表格可编辑；代码可选语言且公开页面服务端高亮；公式可编辑预览，公开页面只输出待渲染 LaTeX；三种输出齐全，代码/公式不翻译；借用 Docmost 社区版的文件保留出处和版权。
- **建议测试**：无，不为各块追加快照；安全反例复用 P1-04。

### [x] P2-07 资源存储与图片

- **任务**：实现内容寻址上传、引用关系、文件响应及图片多尺寸生成、上传和编辑。
- **依赖**：P2-05、P1-02。
- **涉及文件**：`server/internal/assets/{types.go,store.go,upload.go,serve.go,refs.go}`、`server/internal/store/queries/content.sql`、`server/internal/api/admin_assets.go`、`server/internal/doc/nodes/{image.go,registry.go}`；`web/src/admin/composables/{useUpload.ts,useImageVariants.ts}`、`web/src/admin/editor/{upload.ts,extensions.ts}`、`web/src/admin/editor/nodeviews/ImageView.vue`、`web/src/shared/tiptap/nodes/image.ts`、`web/src/shared/styles/content.css`。
- **验收标准**：服务端识别类型和大小，不相信文件后缀；按 sha256 去重，支持 Range 和永久缓存，SVG 有 CSP sandbox；浏览器在 Worker 中生成多宽度 WebP，保留原图和动图；公开输出 `srcset`、懒加载，手动 alt 优先；AI 默认 alt 等 P4-10。
- **建议测试**：无，不测浏览器压缩库；审阅业务输入校验，出现具体上传故障再提回归。

### [x] P2-08 附件与粘贴/拖入上传

- **任务**：接通附件卡片、下载、剪贴板图片和拖入文件的上传流程。
- **依赖**：P2-07。
- **涉及文件**：`web/src/admin/editor/{upload.ts,paste.ts,extensions.ts,menus/slash-items.ts}`、`web/src/admin/editor/nodeviews/AttachmentView.vue`、`web/src/shared/tiptap/nodes/attachment.ts`、`server/internal/doc/nodes/{attachment.go,registry.go}`、`web/src/shared/styles/content.css`。
- **验收标准**：上传显示进度和失败状态，不保存无效资源引用；附件在三种输出中有可读名称和下载地址；资源引用随草稿与发布状态正确更新；不尝试翻译附件本体。
- **建议测试**：无。

### [x] P2-09 超级块与栏宽

- **任务**：实现 2～4 栏、拖块入栏、栏宽调节及对应 Go 输出。
- **依赖**：P2-05、P2-06。
- **涉及文件**：`web/src/shared/tiptap/nodes/columns.ts`、`web/src/admin/editor/nodeviews/ColumnsView.vue`、`web/src/admin/editor/{extensions.ts,menus/slash-items.ts}`、`server/internal/doc/nodes/{columns.go,registry.go}`、`web/src/shared/styles/content.css`、`THIRD_PARTY_NOTICES.md`。
- **验收标准**：不能嵌套超级块；栏宽比例存入节点；窄屏上下排列；纯文本和 Markdown 按栏顺序展开；不实现拖到块左右边缘自动组栏的后续功能。
- **建议测试**：无，非法嵌套复用 P1-04 的最小反例。

### [x] P2-10 Excalidraw 画板

- **任务**：将画板包装为 Vue 节点视图，按需挂载 React/Excalidraw，保存场景及亮暗 SVG。
- **依赖**：P2-07。
- **涉及文件**：`web/src/admin/editor/nodeviews/DrawingView.vue`、`web/src/admin/editor/drawing/{DrawingDialog.vue,mount-excalidraw.ts}`、`web/src/shared/tiptap/nodes/drawing.ts`、`web/src/admin/editor/{extensions.ts,menus/slash-items.ts}`、`server/internal/doc/nodes/{drawing.go,registry.go}`、`web/src/shared/styles/content.css`、`THIRD_PARTY_NOTICES.md`。
- **验收标准**：只在打开画板时加载依赖，使用 `createElement` 不引入 JSX 构建；场景和两张 SVG 按内容寻址保存，重新编辑可恢复场景；公开页面显示适合主题的 SVG，不加载 React；抽取画板文字，视觉描述等待 P4-10。
- **建议测试**：无，不测试 Excalidraw 本身或追加 SVG 快照。

### [x] P2-11 上传视频与封面

- **任务**：实现视频块的本地上传、浏览器截帧/自选封面和播放输出。
- **依赖**：P2-07。
- **涉及文件**：`web/src/admin/composables/useVideoPoster.ts`、`web/src/admin/editor/nodeviews/VideoView.vue`、`web/src/shared/tiptap/nodes/video.ts`、`web/src/admin/editor/{extensions.ts,menus/slash-items.ts}`、`server/internal/doc/nodes/{video.go,registry.go}`、`server/internal/assets/types.go`、`web/src/shared/styles/content.css`。
- **验收标准**：支持约定的视频格式，不依赖 ffmpeg 或服务端转码；封面可重选；播放器可按 Range 读取，Markdown 有封面与说明；上传限制与失败信息明确，不做语音转文字。
- **建议测试**：无，不安排跨设备播放矩阵或 E2E。

### [x] P2-12 平台视频提供方

- **任务**：接入 YouTube、Bilibili、TikTok、抖音的链接识别、嵌入地址、公开信息和本地封面。
- **依赖**：P2-11、P1-07。
- **涉及文件**：`server/internal/videoembed/{provider.go,registry.go,youtube.go,bilibili.go,tiktok.go,douyin.go,thumbnail.go}`、`server/internal/api/admin_video.go`、`web/src/admin/editor/nodeviews/VideoView.vue`、`web/src/shared/tiptap/nodes/video.ts`、`server/internal/doc/nodes/video.go`、`THIRD_PARTY_NOTICES.md`。
- **验收标准**：只认可白名单平台及播放器域名；公开信息只在插入时抓取并保存，失败能手填标题简介；封面存到本地；平台页面只输出封面和播放按钮；实现前按官方资料核实地址格式，受限平台的问题明确记录，不把抓取失败当发布失败。
- **建议测试**：无，不自动调用真实平台；连接验证如需实际请求，先按 D38 确认。

### [x] P2-13 撤回、回收站与版本历史

- **任务**：补齐撤回、删除/恢复、彻底删除、历史列表、差异和回滚为新草稿。
- **依赖**：P2-04、P2-05。
- **涉及文件**：`server/internal/content/{unpublish.go,trash.go,redirects.go}`、`server/internal/history/{log.go,diff.go,restore.go}`、`server/internal/api/{admin_history.go,admin_trash.go}`、`web/src/admin/components/DiffView.vue`、`web/src/admin/views/docs/{DocHistoryView.vue,TrashView.vue}`。
- **验收标准**：撤回不再公开；删除可选 410 或跳转目标，恢复正确处理路径；彻底删除仅超管；回滚创建草稿，未经发布不改变访客内容；git 历史保留，资源不自动删除；生命周期变化更新索引并能通知后续任务。
- **建议测试**：无，复用 P2-04 的生命周期一致性建议，不再建一套集成测试。

### [x] P2-14 资源库与手动清理

- **任务**：展示资源及引用文档，筛选当前未引用资源并支持手动删除。
- **依赖**：P2-07、P2-08、P2-10、P2-12、P2-13。
- **涉及文件**：`server/internal/assets/{refs.go,cleanup.go}`、`server/internal/api/admin_assets.go`、`web/src/admin/components/AssetPicker.vue`、`web/src/admin/views/assets/AssetsView.vue`。
- **验收标准**：图片、附件、视频、画板均可查看引用；清理要求资源库权限并二次确认，提示历史版本可能缺文件；当前草稿/正文引用的资源不能误删；无自动垃圾回收。
- **建议测试**：无，引用关系沿用发布一致性检查；出现误删 bug 时再提出回归建议。

### [x] P2-15 站点、首页与公告设置页面

- **任务**：接通站点名称、logo、主题色、页脚、分享图、主视觉和公告设置。
- **依赖**：P1-06、P1-07、P2-11、P2-14、P2-01。
- **涉及文件**：`server/internal/settings/{store.go,site.go,home.go,banner.go}`、`server/internal/api/admin_settings.go`、`web/src/admin/views/site/{SiteSettingsView.vue,HomeHeroView.vue,BannerView.vue}`、`web/src/admin/components/{AssetPicker.vue,DocPicker.vue}`。
- **验收标准**：设置需要站点设置权限并纳入 data 历史；主视觉按钮 0～3 个，站内目标存 ID，遮罩与文字颜色可配置；公告支持链接和起止时间；页脚含可配置备案、数据来源及源代码链接；修改能触发公开缓存失效，翻译任务在 P5-05 接入。
- **建议测试**：无，不给普通设置表单写测试。

## 第 3 阶段：公开站点

依据：[site.md](design/site.md)、[home.md](design/home.md)、[security-ops.md](design/security-ops.md)、D12、D14、D17、D19、D20、D23、D32～D34、D43、D52、D54。

### [x] P3-01 语言前缀、文档解析与渲染缓存

- **任务**：实现基准语言首页和文档路由、ID 链接解析、301/410/404、已发布内容读取和内存渲染缓存。
- **依赖**：P2-04、P2-13、P1-10。
- **涉及文件**：`server/internal/site/{routes.go,langroute.go,resolve.go,links.go,render.go,cache.go,errors.go}`、`server/internal/site/templates/error.html`。
- **验收标准**：根路径跳到基准语言，所有文档带最终语言前缀；显式语言页面不强制改语言；草稿不可公开；旧路径跳到当前 ID 对应路径，删除记录返回 410；缓存以文档和语言区分，发布/撤回/删除/设置变更立即失效，进程重启可重新生成。
- **建议测试**：无，路径变更复用 P2-04，不另建路由测试集。

### [x] P3-02 三栏布局与导航

- **任务**：完善文档页的 Go 外框、文档树、本页目录、前后篇、页脚及手机抽屉结构。
- **依赖**：P3-01、P2-15。
- **涉及文件**：`server/internal/site/{page.go,nav.go,toc.go}`、`server/internal/site/templates/{layout.html,doc.html}`、`server/internal/site/templates/partials/{head.html,topbar.html,tree.html,toc.html,prevnext.html,footer.html}`、`web/src/shared/styles/{tokens.css,base.css,content.css}`、`web/src/site/site.css`。
- **验收标准**：无 JavaScript 也有完整正文和导航；当前文档与父分支正确定位，前后篇按文档树排列；移动端有抽屉入口；正文 H2～H4 不折叠；AI 未就绪时没有可点击的 AI 入口；源代码链接可见。
- **建议测试**：无，不做布局快照或浏览器矩阵。

### [x] P3-03 公开页面轻交互

- **任务**：接通主题、抽屉、树展开、目录高亮、代码复制、按需公式、平台播放器和引用锚点高亮。
- **依赖**：P3-02、P2-06、P2-12。
- **涉及文件**：`web/src/site/main.ts`、`web/src/site/enhance/{theme.ts,drawer.ts,tree.ts,toc-spy.ts,code-copy.ts,video-facade.ts,math.ts,citation-highlight.ts}`、`server/internal/site/templates/partials/head.html`。
- **验收标准**：轻交互为原生 TypeScript；主题初始化避免明显闪烁；仅含公式时加载 KaTeX；平台播放器点击后才加载；`#b-<块ID>` 能定位并高亮，关闭脚本也能通过原生锚点访问；公开包不包含 Tiptap、React、Excalidraw。
- **建议测试**：无，类型和构建依赖检查足够。

### [x] P3-04 首页主视觉与公告

- **任务**：实现一屏首页、各种背景、按钮及公告显示和关闭记忆。
- **依赖**：P3-02、P2-15、P2-11。
- **涉及文件**：`server/internal/site/home.go`、`server/internal/site/templates/home.html`、`server/internal/site/templates/partials/banner.html`、`web/src/site/enhance/{hero-video.ts,banner.ts}`、`web/src/site/site.css`。
- **验收标准**：首页只有全屏主视觉；背景由 Go 直接输出，按钮解析站内 ID；视频静音循环且 `playsinline`，减少动态效果时显示封面；公告遵守起止时间，关闭同一公告后不再出现；未启用 AI 的按钮不会指向空功能。
- **建议测试**：无，不为起止时间或样式单独增加测试。

### [x] P3-05 SEO、Markdown 与爬虫入口

- **任务**：生成页面 SEO、Markdown 版本、llms 文件、sitemap、robots 和 HTTP 缓存头，接通复制 Markdown。
- **依赖**：P3-01、P3-02、P3-04、P1-04。
- **涉及文件**：`server/internal/site/{seo.go,markdown.go,llms.go,sitemap.go,robots.go,render.go}`、`server/internal/site/templates/partials/head.html`、`web/src/site/enhance/copy-markdown.ts`。
- **验收标准**：title、描述、Open Graph、canonical、当前语言与 `x-default`、面包屑正确；`.md`、各语言 llms 入口（此时仅基准语言）和 sitemap 只含已发布内容；HTML 头部及响应 `Link` 指向 Markdown；robots 允许 AI 爬虫；HTML 使用 `no-cache`/ETag/304，哈希静态资源永久缓存。
- **建议测试**：无，不创建 SEO 快照或爬虫评测。

### [x] P3-06 基础站交付与运行说明

- **任务**：整理基础站的构建、启动、注册和反向代理说明，形成第一个可用交付点。
- **依赖**：P3-01～P3-05、P2-02。
- **涉及文件**：`README.md`、`deploy/README.md`、`Makefile`、`STATUS.md`、`docs/plan.md`。
- **验收标准**：`make build` 产出单个可执行文件；只需二进制和 data 目录即可运行基准语言文档站；后台可初始化、编辑、发布，公开页与爬虫入口可用；说明醒目提示启动后立即注册超管、HTTPS 交给代理及可信代理配置；AI 和目标语言标明后续开放，不依赖未完成的 Docker/备份功能。
- **建议测试**：无，不在阶段末新增 E2E；运行已有 `make check` 和构建检查即可。

## 第 4 阶段：AI 问答、模型接入与长期记忆

依据：[ai-models.md](design/ai-models.md)、[memory.md](design/memory.md)、[ai-qa.md](design/ai-qa.md)、[ai-guard.md](design/ai-guard.md)、D10、D22、D25、D29、D44、D46、D48～D53。

### [ ] P4-01 AI 任务清单、提示词与流式基础

- **任务**：集中定义所有 AI 任务、默认档位/记忆/输出上限，实现提示词嵌入和 SSE 读写。
- **依赖**：P3-06。
- **涉及文件**：`server/internal/aitask/tasks.go`、`server/internal/prompts/prompts.go`、`server/internal/prompts/{router.md,verify.md,rerank.md,answer.md,cache-judge.md,context.md,describe-image.md,describe-attachment.md,describe-video.md,site-summary.md,memory-assistant.md,slug.md,seo-description.md,compact-conversation.md}`、`server/internal/httpx/sse.go`、`web/src/shared/api/sse.ts`。
- **验收标准**：模型分配和记忆映射共用唯一任务清单，翻译任务可预先声明但未实现时不可调用；记忆注入由业务层处理，不让 `llm` 反向依赖 `memory`；SSE 支持心跳、取消和错误结束，设置 `X-Accel-Buffering: no`；提示词集中可审阅，要求只依据材料回答并带块引用。
- **建议测试**：无，不做提示词快照或 AI 评测。

### [ ] P4-02 完整模型渠道与目录管理

- **任务**：扩展首个渠道登记，接入 models.dev 内置快照、每日更新、手动覆盖及完整渠道页面。
- **依赖**：P4-01、P2-02。
- **涉及文件**：`server/tools/catalogsnap/main.go`、`server/internal/llm/{channels.go,catalog.go,models.go,catalog_snapshot.json}`、`server/internal/store/queries/llm.sql`、`server/internal/api/admin_models.go`、`web/src/admin/views/ai/ChannelsView.vue`、`web/src/admin/views/setup/steps/ChannelStep.vue`、`Makefile`。
- **验收标准**：离线有编译内置目录，更新失败仍可用旧目录；手动资料优先；同厂商可多渠道，自定义地址、额外请求头、超时、并发、启停可配置；仅启用模型可被选择；密钥只显示约定的掩码信息；目录刷新不发送站点数据，尚未实现的连接探测不可点击。
- **建议测试**：无，不自动请求真实模型；目录工具是公开数据更新，生成结果检查即可。

### [ ] P4-03 OpenAI 兼容对话协议

- **任务**：建立统一协议请求/响应和 OpenAI Chat Completions 的普通、流式调用。
- **依赖**：P4-02。
- **涉及文件**：`server/internal/llm/protocol/{protocol.go,openai.go}`。
- **验收标准**：适配自定义兼容地址和请求头；统一文本、结构化请求、用量、错误与流式结束；取消/超时终止请求，不泄露密钥；接口细节实现前核对官方资料，不直接复制不兼容许可代码。
- **建议测试**：无，先做编译和协议映射审阅；真实连接按钮在 P4-06 接通后由用户决定使用。

### [ ] P4-04 Anthropic 与 Gemini 对话协议

- **任务**：分别实现 Messages 和 Gemini 协议，复用统一响应、流式事件与错误结构。
- **依赖**：P4-03。
- **涉及文件**：`server/internal/llm/protocol/{anthropic.go,gemini.go,protocol.go}`。
- **验收标准**：各自的认证、系统提示、消息组织、流式片段、用量和错误正确映射；目录预设能选择相应协议；不新增 Bedrock、Vertex 或 Azure 专用认证适配器。
- **建议测试**：无，不做供应商调用矩阵；遇协议兼容 bug 时再提最小回归。

### [ ] P4-05 嵌入与重排协议

- **任务**：实现 OpenAI/Gemini 嵌入及 Cohere/Jina 风格重排，统一向量、维度和排名结果。
- **依赖**：P4-03、P4-04。
- **涉及文件**：`server/internal/llm/protocol/{embed_openai.go,embed_gemini.go,rerank.go,protocol.go}`。
- **验收标准**：批量输入与输出次序一致，维度及无效向量被校验；排名可映射回原候选；错误与计费信息统一；对话模型不能被当成嵌入或专用重排模型调用。
- **建议测试**：无，不测试供应商服务自身的效果。

### [ ] P4-06 档位、统一调用、计费与连接探测

- **任务**：实现任务→档位→有序模型解析、能力检查、结构化输出降级、重试/备用、并发限制和用量记录。
- **依赖**：P4-01～P4-05。
- **涉及文件**：`server/internal/llm/{tiers.go,client.go,structured.go,cost.go,probe.go}`、`server/internal/settings/ai.go`、`server/internal/store/queries/llm.sql`、`server/internal/api/admin_models.go`；`web/src/admin/components/ModelPicker.vue`、`web/src/admin/views/ai/{ChannelsView.vue,TiersView.vue}`。
- **验收标准**：档位与任务分配只允许超管；模型按能力过滤；视觉未配时取快速档看图模型，重排未配时留给快速档兜底；429/5xx 退避后可切备用，普通/流式超时分开；JSON 校验失败重试一次再备用；实际金额优先，否则用量乘单价，无单价明确警告；探测仅用户点击才发送小请求。
- **建议测试**：无，不自动运行连接探测；调用次数与花费边界集中到 P4-15 的建议。

### [ ] P4-07 记忆文件、注入与任务映射

- **任务**：建立内置记忆条目、手动编辑、任务取用、目标语言过滤、大小校验及版本号。
- **依赖**：P4-01、P4-06、P1-07。
- **涉及文件**：`server/internal/memory/{items.go,store.go,mapping.go,inject.go,budget.go}`、`server/internal/api/admin_memory.go`；`web/src/admin/views/memory/{MemoryView.vue,MemoryTasksView.vue}`。
- **验收标准**：记忆存 Markdown 并纳入 data git；默认单条 1.5KB、任务总量 5KB（UTF-8），保存条目或改映射都校验受影响任务；注入置于系统提示最前，包含说明且只取适用语言；新请求立即使用新版本；显示可被公开的安全提醒；编辑需要 AI 记忆权限。
- **建议测试**：不另建测试，大小、版本与只读反例合并到 P4-08 的建议。

### [ ] P4-08 记忆助手与确认提议

- **任务**：实现记忆助手对话、逐条提议/差异、确认或拒绝，并提前提供术语表最小读写。
- **依赖**：P4-07。
- **涉及文件**：`server/internal/memory/{assistant.go,proposals.go}`、`server/internal/i18n/translate/glossary.go`、`server/internal/store/queries/memory.sql`、`server/internal/prompts/memory-assistant.md`、`server/internal/api/admin_memory.go`、`web/src/admin/views/memory/MemoryView.vue`。
- **验收标准**：精确档先提炼，再逐项判新增/更新/删除/不变；未经确认不写入，确认时重新校验当前版本和注入量；助手不能改只读条目；敏感内容拒绝保存；术语写术语表并标手动来源，不塞记忆；对话可续接且按配置清理。
- **建议测试**：建议仅一组提议写入反例，覆盖未确认、过期提议、只读修改及单条/任务超限；不评测助手文风。

### [ ] P4-09 项目概述与发布元数据生成

- **任务**：实现自动项目概述、路径名及 SEO 描述生成，接入原先预留的发布接口。
- **依赖**：P4-06、P4-07、P2-04。
- **涉及文件**：`server/internal/memory/overview.go`、`server/internal/ai/meta/{slug.go,seo.go}`、`server/internal/content/{generators.go,publish.go}`、`server/internal/prompts/{site-summary.md,slug.md,seo-description.md}`、`server/internal/jobs/{kinds.go,schedule.go}`。
- **验收标准**：首次发布缺省元数据可自动生成，路径仍经合法性与唯一性校验；手动内容不被覆盖；概述每天或发布累计达条件更新，手改后停止自动覆盖；向导/模型未就绪仍允许手动填写发布；自动生成失败不假报发布成功。
- **建议测试**：无，不为生成文字做 AI 评测。

### [ ] P4-10 图片、画板、视频与附件描述

- **任务**：实现视觉/文件描述、文字抽取与 sha256 缓存，给检索和默认 alt 提供材料。
- **依赖**：P4-06、P4-07、P2-08、P2-10、P2-12。
- **涉及文件**：`server/internal/ai/ingest/{describe.go,extract.go}`、`server/internal/store/queries/search.sql`、`server/internal/prompts/{describe-image.md,describe-attachment.md,describe-video.md}`、`server/internal/doc/nodes/{image.go,drawing.go,attachment.go,video.go}`。
- **验收标准**：同文件只生成一次描述，不同出现位置各自保留上下文；纯文本附件直接抽取，PDF 等仅交给支持文件输入的模型；视频用标题/公开信息/封面和相邻块，不做转录；画板兼用场景文字；手写 alt 不被替换，描述更新使相关公开缓存失效。
- **建议测试**：无，不用真实图片、PDF、视频建立评测集。

### [ ] P4-11 增量入库与向量索引

- **任务**：从已发布基准语言文档组装检索单元，补上下文、写关键词和向量索引，维护内存向量表。
- **依赖**：P4-10、P4-09、P1-03。
- **涉及文件**：`server/internal/ai/ingest/{pipeline.go,units.go,contextual.go,embed.go,keyword.go}`、`server/internal/textutil/bigram.go`、`server/internal/ai/retrieve/vectors.go`、`server/internal/store/queries/search.sql`、`server/internal/prompts/context.md`。
- **验收标准**：文本按小节约 300～800 字合并；媒体单元带上下块且保存块映射；哈希未变不重新付费处理；只索引基准语言已发布内容，撤回/删除移除检索数据；CJK 二元组和其他语言按词切分供入库/查询共用；向量记录模型，启动加载、更新同步，维度不兼容不能混用。
- **建议测试**：无，不创建完整检索样例或评测题集。

### [ ] P4-12 混合召回、小节扩展与重排

- **任务**：实现向量/FTS5 两路召回、RRF、重排及命中块所在小节读取。
- **依赖**：P4-11、P4-05、P4-06。
- **涉及文件**：`server/internal/ai/retrieve/{vectors.go,keyword.go,fuse.go,rerank.go,section.go}`、`server/internal/store/queries/search.sql`、`server/internal/prompts/rerank.md`。
- **验收标准**：精确词可通过关键词召回，语义结果可通过向量召回；合并按排名处理，参数可配置；有重排档优先使用，没有则快速档替代；返回结果包含有效块 ID，不包含已下线文档，后续能按文档顺序扩展材料。
- **建议测试**：无，不自动做检索效果评测；参数效果由后续调试台观察。

### [ ] P4-13 检索员与有界工具调用

- **任务**：接通快速模型核查、补充检索、读小节和读文档树工具，支持子问题并行。
- **依赖**：P4-12、P4-07。
- **涉及文件**：`server/internal/ai/retrieve/{retriever.go,tools.go}`、`server/internal/prompts/verify.md`。
- **验收标准**：输出真正相关的块和理由，资料不足可以再检索；默认最多 3 轮/5 次工具调用，超限用已有材料；工具参数经校验、仅读已发布内容；并行度受渠道限制，可取消；失败可解释且不暴露服务技术细节。
- **建议测试**：无，轮次/调用上限复用 P4-15 的建议，不用真实模型评测检索员。

### [ ] P4-14 问答会话、人机验证与备用 PoW

- **任务**：提供验证换会话、Turnstile 校验及 PoW 出题/验证，建立签名会话 cookie。
- **依赖**：P4-06、P1-10。
- **涉及文件**：`server/internal/ai/guard/{turnstile.go,pow.go,session.go}`、`server/internal/api/public_session.go`、`server/internal/config/{runtime.go,secrets.go}`。
- **验收标准**：只有有效验证结果可换有时效的签名会话；挑战有有效期且不能重复使用；伪造/过期 cookie 被拒绝；无关站点不能换会话；Turnstile 不可达时可以使用自托管 PoW，风险升高时提高难度，前端降级由 P4-20 接通。
- **建议测试**：不另建测试，会话伪造/过期、挑战重放合并到 P4-15 的最小防滥用建议。

### [ ] P4-15 上限、风险处置与预算

- **任务**：实现问题/检索/并发上限、频率与重复检查、风险判定、封禁、会话/IP 花费及每日问答预算。
- **依赖**：P4-14、P4-06、P4-13。
- **涉及文件**：`server/internal/ai/guard/{limits.go,checks.go,risk.go,bans.go,budget.go,events.go}`、`server/internal/settings/ai.go`、`server/internal/store/queries/{guard.sql,llm.sql}`、`server/internal/api/admin_guard.go`、`web/src/admin/views/ai/GuardView.vue`。
- **验收标准**：默认 500 字、4 子问题、3 轮/5 工具调用，同会话一次只处理一问；输出长度按任务上限；可疑请求要求再验证，确认刷流封禁 10 分钟，IPv6 按 /64，后台可解封；每小时会话/IP 预算可关闭，IP 默认为会话的 5 倍；每日问答预算 0 为无限，后台翻译/描述不占此额度；并发请求不能绕过预算检查。
- **建议测试**：建议仅一组防滥用反例，覆盖无效会话、过期/重放挑战、调用上限和并发预算扣算、时间窗口恢复；使用假验证及 `testing/synctest`，不攻击真实接口。

### [ ] P4-16 对话存储、前置判断与自动压缩

- **任务**：保存/清空对话与问答记录，接通问题分类、风险、完整改写、子问题和 opencode 压缩机制。
- **依赖**：P4-07、P4-15、P4-01。
- **涉及文件**：`server/internal/ai/qa/{conversations.go,records.go,router.go,compact.go}`、`server/internal/store/queries/qa.sql`、`server/internal/prompts/{router.md,compact-conversation.md}`。
- **验收标准**：浏览器只持会话标识，刷新可恢复；记录不保存 IP，按保留期清理；追问补成独立问题，标记能否使用缓存，无关请求不进入精确档；压缩按前置/回答模型较小上下文上限，默认预留 10%、保留最近约 15000 token、摘要增量更新，输入过长时压缩后只重试一次；参数可配置，无法继续时返回普通失败提示。
- **建议测试**：无，不评测摘要质量；上限执行复用 P4-15，不单独写压缩快照。

### [ ] P4-17 问答编排、材料与 SSE 回答

- **任务**：串接防滥用、前置判断、并行检索、去重/小节组装、回答、记录和流式事件，预留缓存调用点。
- **依赖**：P4-13、P4-15、P4-16。
- **涉及文件**：`server/internal/ai/qa/{service.go,pagecontext.go,assemble.go,answer.go,events.go}`、`server/internal/api/public_ask.go`、`server/internal/prompts/answer.md`。
- **验收标准**：文档页问题优先参考当前已发布文档，不能借上下文读取草稿；材料保留有效块 ID 并按文档顺序组装；回答只依据材料，无材料明确告知；进度、文字、引用、完成/错误事件可读取且取消可释放调用；问答和花费均记录；缓存接通前保持关闭，公开启用等待 P4-22。
- **建议测试**：无，不自动调用真实模型或运行整条问答 E2E。

### [ ] P4-18 精确/语义缓存与失效

- **任务**：实现两层匹配、完整问题判断、缓存存储、记忆/嵌入分区和发布联动失效，接入问答流程。
- **依赖**：P4-17、P4-11、P4-07。
- **涉及文件**：`server/internal/ai/cache/{lookup.go,store.go,invalidate.go}`、`server/internal/ai/qa/{service.go,router.go}`、`server/internal/store/queries/qa.sql`、`server/internal/prompts/cache-judge.md`、`server/internal/content/{publish.go,unpublish.go,trash.go}`。
- **验收标准**：第一问程序检查后、前置模型前可精确命中，追问必须先改写；不完整问题不查不写；语义默认快速模型判断最多 3 候选，阈值模式可选；向量复用检索向量；引用块变化/下线、无引用回答遇任意发布、到期、问答记忆版本变化均正确失效；换嵌入模型整体清空，换回答模型默认不清空；语言不作分区键。
- **建议测试**：建议仅一组缓存失效表驱动测试，覆盖引用块变化、无引用发布、记忆版本、嵌入切换，以及不完整追问不能复用；不评测语义相似度质量。

### [ ] P4-19 反馈、精选回答与缓存管理

- **任务**：接通反馈、精选回答、记录筛选、缓存设置及命中/误命中指标与阈值建议。
- **依赖**：P4-18。
- **涉及文件**：`server/internal/ai/qa/feedback.go`、`server/internal/ai/cache/{curated.go,calibrate.go}`、`server/internal/api/{public_feedback.go,admin_qa.go}`；`web/src/admin/views/ai/{QaRecordsView.vue,QaCacheView.vue}`。
- **验收标准**：反馈只能关联本会话的回答；“没帮助”回答不继续进入自动缓存；记录可筛未答出/负反馈；精选可编辑、优先命中、不过期且不受记忆变化影响，但不能引用已失效内容；缓存关闭仍保留记录；精确/语义命中分别统计，阈值建议只作参考。
- **建议测试**：无，缓存内容失效复用 P4-18，不增加管理页测试。

### [ ] P4-20 公开问答浮窗与独立页面

- **任务**：实现共享对话 UI、SSE 进度、Markdown 引用、反馈、人机验证/PoW 和封禁提示，接通 AI 入口。
- **依赖**：P4-14、P4-17、P4-19、P3-03。
- **涉及文件**：`server/internal/site/{ask.go,islands.go}`、`server/internal/site/templates/{ask.html,partials/island.html,partials/topbar.html,home.html,error.html}`、`web/src/site/main.ts`、`web/src/site/islands/{AskWidget.vue,AskPage.vue}`、`web/src/site/chat/{useChat.ts,markdown.ts,ChatPanel.vue,MessageList.vue,MessageItem.vue,CitationChip.vue,ProgressSteps.vue,ChatInput.vue,FeedbackButtons.vue,TurnstileGate.vue,pow-worker.ts,BanNotice.vue}`、`web/src/shared/i18n/messages.zh-Hans.json`。
- **验收标准**：浮窗与 `/<lang>/ask` 共享服务端对话，展开和刷新不丢内容；引用链接到原文块并高亮；回答 Markdown 禁止原始 HTML，链接遵守安全规则；验证组件加载失败后 Worker 算 PoW，不阻塞页面；封禁/超预算提示恢复时间；未配置 AI 时仍不展示入口，公开页仅按需加载 Vue。
- **建议测试**：无，不新增聊天 UI 快照或 E2E；Markdown 安全要求按 P1-04 的反例思路在出现必要性时另提。

### [ ] P4-21 问答调试台与重建索引

- **任务**：展示中间步骤、耗时/花费，提供更换嵌入模型后的费用预估和全量重建。
- **依赖**：P4-17、P4-18、P4-06。
- **涉及文件**：`server/internal/ai/qa/debug.go`、`server/internal/ai/ingest/reindex.go`、`server/internal/api/admin_qa.go`、`server/internal/store/queries/llm.sql`、`web/src/admin/views/ai/{QaDebugView.vue,TiersView.vue}`、`web/src/admin/composables/{useCostEstimate.ts,useJobProgress.ts}`、`web/src/admin/components/CostEstimateDialog.vue`。
- **验收标准**：调试展示分类/改写、候选/排序、核查、最终材料、每步成本和耗时，需 AI 设置权限；修改档位仍仅超管；重建先预估再确认，进度与失败可见；切换期间不混用新旧维度，正确清空回答缓存；调试点击才运行模型，敏感中间结果不写请求日志。
- **建议测试**：无，不自动运行调试或评测题集。

### [ ] P4-22 AI 系统设置与启用条件

- **任务**：接通超管系统设置、后台任务页面和 AI 可用性判断，完成基准语言问答的交付边界。
- **依赖**：P4-01～P4-21。
- **涉及文件**：`server/internal/api/{admin_system.go,admin_jobs.go}`、`server/internal/config/{runtime.go,secrets.go}`、`server/internal/settings/ai.go`、`web/src/admin/views/system/{SystemSettingsView.vue,JobsView.vue}`、`server/internal/setup/state.go`、`README.md`、`deploy/README.md`。
- **验收标准**：快速/精确/嵌入档、索引和验证条件未就绪时关闭问答并列出缺项；停用渠道能及时影响可用性；防滥用不能被缓存命中绕过；超管可设 Turnstile、可信代理、每日上限，任务可查看/重试；反向代理说明补齐 SSE 不缓冲；无目标语言时全部 AI 功能可独立工作。
- **建议测试**：无，不设阶段结束评测；仅运行已有检查，任何真实调用另经用户确认。

## 第 5 阶段：多语言

依据：[i18n.md](design/i18n.md)、[memory.md](design/memory.md)、[ai-qa.md](design/ai-qa.md)、D11～D13、D47～D49、D51、D52。

### [ ] P5-01 译文文件与增量状态

- **任务**：实现按文档/语言/块 ID 保存译文、原文哈希、机器/人工来源和待复核状态。
- **依赖**：P4-22、P1-03。
- **涉及文件**：`server/internal/i18n/translate/{files.go,status.go}`、`server/internal/settings/languages.go`、`server/internal/store/queries/i18n.sql`。
- **验收标准**：译文文件是真实数据并沿用原文块 ID/结构；数据库状态可从文件重建；原文变化机器块进入待翻译，人工块只标待复核不被覆盖；旧译文在新译文生成前继续可读；删除块不留无效译文引用。
- **建议测试**：无，人工保护和写入一致性合并到 P5-04 的建议。

### [ ] P5-02 翻译记忆与术语抽取

- **任务**：接通同哈希长块译文复用、首次翻译术语抽取，以及手动/自动来源管理。
- **依赖**：P5-01、P4-08。
- **涉及文件**：`server/internal/i18n/translate/{reuse.go,terms.go,glossary.go}`、`server/internal/prompts/extract-terms.md`。
- **验收标准**：默认至少 20 字的相同原文块可复用，短块仍结合上下文；每文档首次翻译到目标语言前抽词；自动词条不覆盖手动词条，可按来源筛选和整批撤销；只注入当前原文实际出现的术语，规则与术语分开存储。
- **建议测试**：无，不测试术语提取质量或为复用写独立测试。

### [ ] P5-03 行内占位、分批与降级校验

- **任务**：保护格式/地址/代码，按预估输出量分批，校验返回块 ID 和占位标签并逐级缩小重试。
- **依赖**：P5-01、P4-06。
- **涉及文件**：`server/internal/i18n/translate/{inline.go,batch.go,validate.go}`、`server/internal/prompts/translate-blocks.md`、`server/internal/doc/{marks.go,nodes/registry.go}`。
- **验收标准**：默认约 1000 token、块边界切分、均衡且优先标题，超大表格按行拆；链接 URL/行内代码不交模型，代码块/公式/嵌入不翻；块 ID/占位缺失或长原文照抄触发对半重试→逐块，长度超限减少上下文；最终失败显式标记并保留原文。
- **建议测试**：不另建测试，格式保护、漏块和降级反例合并到 P5-04 的一组建议。

### [ ] P5-04 完整翻译流水线与发布联动

- **任务**：串接复用、抽词、顺序批次、术语检查/重译、可选反思及译文写入、git 和公开缓存刷新。
- **依赖**：P5-02、P5-03、P4-07。
- **涉及文件**：`server/internal/i18n/translate/{pipeline.go,check.go,review.go,files.go}`、`server/internal/prompts/{translate-blocks.md,translate-review.md}`、`server/internal/content/publish.go`、`server/internal/jobs/{kinds.go,worker.go}`。
- **验收标准**：同文档批次顺序执行、不同文档受渠道并发控制；提示包含整篇原文、本批标签、前批/相邻译文、标题、匹配术语与任务记忆；术语不符合只重译一次，仍不符标待复核；反思按语言启用且默认关闭；人工修改或源文在排队期间变化时不会被陈旧任务覆盖；仅发布触发付费翻译，失败可追踪重试。
- **建议测试**：建议仅一组假模型驱动的反例，覆盖占位/块 ID 缺失后的降级、人工译文保护和源文变化后的陈旧结果拒写；不测试语言质量、不跑全站真实翻译。

### [ ] P5-05 站点文字与公开 UI 翻译

- **任务**：翻译文档标题/SEO/图注/alt、站点设置和公开界面文字，接入设置变更后的异步更新。
- **依赖**：P5-04、P2-15。
- **涉及文件**：`server/internal/i18n/translate/strings.go`、`server/internal/prompts/translate-strings.md`、`server/internal/settings/store.go`、`web/src/shared/i18n/{messages.zh-Hans.json,t.ts}`、`server/internal/site/page.go`。
- **验收标准**：`_site.json`、`_ui.json` 按语言保存；翻译只改文字，不改资源 sha256、链接 ID 和程序键；背景字按开关翻译；后台文字保持中文；图片内文字和附件本体不翻；公开页面得到当前语言的按钮/错误/未翻译提示。
- **建议测试**：无，不为每个界面字符串写快照。

### [ ] P5-06 语言启停与费用预估

- **任务**：提供语言列表、全站翻译估价、确认后投递、翻译参数及按语言重新翻译。
- **依赖**：P5-04、P5-05、P4-21。
- **涉及文件**：`server/internal/i18n/{languages.go,translate/estimate.go}`、`server/internal/settings/languages.go`、`server/internal/api/admin_i18n.go`、`web/src/admin/views/i18n/LanguagesView.vue`、`web/src/admin/composables/{useCostEstimate.ts,useJobProgress.ts}`、`web/src/admin/components/CostEstimateDialog.vue`。
- **验收标准**：启用语言需管理语言权限，估价展示后确认才开始；批次大小、复用长度门槛和反思可配置；停用后公共入口及索引目录不再列该语言；翻译规则记忆变化只提示重译与估价，不自动花费；不默认开放 RTL 语言。
- **建议测试**：无，不调用真实全站翻译测估价误差。

### [ ] P5-07 翻译进度、人工编辑与术语页面

- **任务**：完成按语言进度、失败/待复核队列、块对齐编辑、术语来源筛选和整批撤销。
- **依赖**：P5-06、P5-01、P5-02。
- **涉及文件**：`server/internal/api/admin_i18n.go`、`server/internal/i18n/translate/status.go`、`web/src/admin/components/BlockSideBySide.vue`、`web/src/admin/views/i18n/{TranslationProgressView.vue,TranslationEditView.vue,GlossaryView.vue}`。
- **验收标准**：待翻译、失败、待复核数量可追踪；按原文块 ID 对齐，不把多栏打乱；人工编辑标明来源并纳入历史；管理译文与管理语言权限分开；重试/重译明确费用与范围，不覆盖人工内容。
- **建议测试**：无，人工内容保护复用 P5-04，不加对照页面 E2E。

### [ ] P5-08 多语言公开页、路由与 SEO

- **任务**：让正文/标题/导航读取译文，接通根路径语言匹配、手动切换、部分原文提示和多语言爬虫入口。
- **依赖**：P5-05、P5-06、P3-05。
- **涉及文件**：`server/internal/i18n/accept.go`、`server/internal/site/{langroute.go,resolve.go,links.go,page.go,nav.go,seo.go,cache.go,markdown.go,llms.go,sitemap.go}`、`server/internal/site/templates/partials/{topbar.html,untranslated.html,head.html}`、`web/src/site/main.ts`。
- **验收标准**：仅 `/` 按明确偏好 cookie→Accept-Language→基准语言返回 302，考虑 cookie 和语言的缓存差异；用 x/text Matcher 处理地区别名；显式语言路径不强跳；切换同文档保留块锚点；旧译文继续显示，新增未译块回退原文并用访客语言提示；所有启用语言拥有正确 lang/hreflang/sitemap/Markdown，站内链接同语言解析。
- **建议测试**：无，不新增语言组合矩阵，利用类型检查及既有路由检查。

### [ ] P5-09 GeoIP 与语言切换建议

- **任务**：接入本地 DB-IP Lite，收集浏览器语言/时区/国家，快速模型判断、缓存及规则兜底。
- **依赖**：P5-08、P4-06、P1-10。
- **涉及文件**：`server/internal/geoip/{geoip.go,update.go}`、`server/internal/i18n/suggest/{signals.go,decide.go,rules.go}`、`server/internal/prompts/language-suggest.md`、`server/internal/api/public_locale.go`、`server/internal/store/queries/llm.sql`、`web/src/site/islands/LanguageSuggest.vue`、`server/internal/site/templates/partials/footer.html`。
- **验收标准**：归属地离线查，不记录原始 IP，数据库每月更新且页脚署名；模型结果按信号组合缓存，失败/超时规则兜底；只有多个信号支持且目标已启用时建议；提示用建议语言；已明确选择或拒绝后不再弹；不强制跳转，不把国家当决定性偏好。
- **建议测试**：无，不调用真实 GeoIP 下载或模型做地区评测。

### [ ] P5-10 多语言问答材料与缓存回答

- **任务**：把多语言提问、材料、回答、引用及缓存翻译接入现有问答，继续共用基准语言索引。
- **依赖**：P5-08、P5-04、P4-18、P4-20。
- **涉及文件**：`server/internal/ai/qa/{router.go,assemble.go,answer.go}`、`server/internal/ai/cache/{translate.go,store.go,invalidate.go}`、`server/internal/prompts/translate-answer.md`、`web/src/site/chat/{CitationChip.vue,useChat.ts}`。
- **验收标准**：前置改写成基准语言后检索，不为目标语言另建向量；材料优先同块 ID 的目标译文，回答及引用使用访客语言；缓存以基准问题共享，缺少该语言回答才翻译保存；缓存翻译带适用记忆并保留引用；译文与引用变化能清除受影响的语言回答，不能返回已下线材料。
- **建议测试**：无，缓存有效性复用 P4-18，不运行多语言 AI 评测。

## 第 6 阶段：收尾

依据：[site.md](design/site.md)、[admin.md](design/admin.md)、[security-ops.md](design/security-ops.md)、D16、D23、D28、D31、D43、D50。

### [ ] P6-01 访问采集与隐私统计

- **任务**：接收页面 beacon、计算每日匿名访客、过滤爬虫，单独统计 AI 爬虫读取 Markdown。
- **依赖**：P5-09、P3-05。
- **涉及文件**：`server/internal/stats/{beacon.go,visitor.go,bots.go}`、`server/internal/api/public_beacon.go`、`server/internal/store/queries/stats.sql`、`web/src/site/enhance/beacon.ts`、`server/internal/site/{markdown.go,llms.go}`、`web/src/site/main.ts`。
- **验收标准**：统计不用 cookie；访客键由 IP+UA+当天盐计算，原始 IP 不入库，跨日不能关联；来源只存域名；记录文档、语言和国家；人类页面浏览与爬虫读取分开，不把 beacon 与服务器页面访问重复计数。
- **建议测试**：无，不引入流量压测或真实访客数据。

### [ ] P6-02 汇总、仪表盘与管理概览

- **任务**：每日汇总、明细清理、统计页面和仪表盘，组合 AI 花费、问答、缓存、翻译、任务及风险指标。
- **依赖**：P6-01、P4-19、P4-21、P4-22、P5-07。
- **涉及文件**：`server/internal/stats/{rollup.go,query.go}`、`server/internal/store/queries/{stats.sql,llm.sql,qa.sql}`、`server/internal/api/admin_stats.go`、`web/src/admin/views/{DashboardView.vue,stats/StatsView.vue}`。
- **验收标准**：明细按保留期清理、按日汇总长期保留；支持热门文档/来源/国家/语言及 AI 爬虫；花费按渠道、模型、任务区分；命中率分精确与语义，负反馈可见；仪表盘需查看统计权限，超管任务/审计入口仍按独立权限控制；没有数据时正确显示空态。
- **建议测试**：无，不为每种统计图或 SQL 汇总写测试。

### [ ] P6-03 一致备份与定时保留

- **任务**：打包完整 data 目录，生成一致 SQLite 快照，提供是否包含密钥、下载、定时备份和保留份数。
- **依赖**：P6-02、P1-07。
- **涉及文件**：`server/internal/backup/{export.go,schedule.go}`、`server/internal/api/admin_backup.go`、`server/internal/config/runtime.go`、`web/src/admin/views/system/BackupView.vue`。
- **验收标准**：SQLite 使用备份接口，不直接复制正在写入的数据库；备份覆盖正文/译文/记忆/草稿/资源/配置及付费派生数据；密钥是否包含由用户选择且清楚标示；避免递归打包已有 backups、临时文件和运行锁；定时任务保留最近 N 份，仅超管导出下载；失败不删除最后可用备份。
- **建议测试**：不另建测试，备份往返和密钥选项合并到 P6-04 的最小建议。

### [ ] P6-04 初始化恢复与失败保护

- **任务**：校验备份包，在初始化向导接通从备份恢复，并处理错误包和恢复失败。
- **依赖**：P6-03、P2-02。
- **涉及文件**：`server/internal/backup/restore.go`、`server/internal/setup/{restore.go,state.go}`、`server/internal/api/admin_setup.go`、`web/src/admin/views/setup/steps/RestoreStep.vue`、`web/src/admin/views/setup/SetupWizardView.vue`、`deploy/README.md`。
- **验收标准**：拒绝路径穿越、非法链接和不兼容备份；先校验再替换，不覆盖到半途才发现损坏；恢复失败不丢原数据；成功后重新加载配置/账号、迁移数据库、重建缺失的免费索引，不自动重算付费数据；恢复未包含的密钥提示补填；恢复仅按初始化和超管策略开放。
- **建议测试**：建议仅一组临时目录的备份往返/拒绝反例，覆盖正文和资源可恢复、密钥选项、恶意路径及损坏包不破坏原数据；不安排生产恢复演练。

### [ ] P6-05 data git 异地备份

- **任务**：实现 data 仓库远程地址、凭据和定时推送，补齐后台设置及失败状态。
- **依赖**：P6-03、P4-22、P1-07。
- **涉及文件**：`server/internal/history/remote.go`、`server/internal/config/{runtime.go,secrets.go}`、`server/internal/jobs/schedule.go`、`server/internal/api/admin_system.go`、`web/src/admin/views/system/SystemSettingsView.vue`、`deploy/README.md`。
- **验收标准**：仅超管配置，凭据不进入 data git 和日志；按已配置计划推送 data 历史，失败可查看重试；说明远程 git 不含数据库和资源，不能替代完整备份；代码仓库的 push 仍由用户明确触发。
- **建议测试**：无，不向真实远程仓库自动测试推送。

### [ ] P6-06 Docker 与反向代理配置

- **任务**：提供多阶段容器构建、数据卷、Compose 和 Nginx/Caddy 示例。
- **依赖**：P6-04、P6-05、P3-06。
- **涉及文件**：`deploy/{Dockerfile,docker-compose.yml,nginx.conf.example,Caddyfile.example,README.md}`、`Makefile`。
- **验收标准**：先前端后 Go 构建，纯 Go 二进制进最小镜像；只挂一个 `/data` 卷；HTTPS 由代理终止，SSE 不缓冲、正确传递真实 IP 且配置可信代理；带健康检查和优雅关闭说明；示例只使用保留域名、文档用 IP 和占位值；注册超管提醒醒目。
- **建议测试**：无，只做已有构建与配置检查，不跑自动部署 E2E。

### [ ] P6-07 升级说明、许可证与首版交付

- **任务**：整理安装/升级/回退、账号恢复、备份恢复和已知限制，核对借用声明及首版变化。
- **依赖**：P6-01～P6-06。
- **涉及文件**：`README.md`、`deploy/README.md`、`CHANGELOG.md`、`THIRD_PARTY_NOTICES.md`、`STATUS.md`、`docs/plan.md`、`docs/design/*.md`、`docs/playbooks/{add-migration.md,push.md}`。
- **验收标准**：文档与实际实现一致，升级前备份、迁移失败处理及回退限制写清；借用代码出处/版本/版权与汇总一致，公开页提供对应源代码链接；全部任务及遗留项如实记录；已有 `make check` 和最终构建通过；首版不包含传统搜索、导入、块知识管理、实时协作、文档版本树、Ctrl+K、草稿分享、标题折叠或新增模型护栏，也不推荐 CrowdSec/Litestream。
- **建议测试**：无，不设发版前测试阶段；如有实际未解缺陷，只对该缺陷另提最小回归建议。
