# 文件结构：Go 后端（server/）

> 本文件是契约：**只能创建、修改这里列出的文件**。增加、删除、改名、移动任何文件或目录，都必须先询问用户（D45）。标记说明见 [README.md](README.md)。

Go module 路径是 `nice-wiki`，所以包的导入路径形如 `nice-wiki/internal/doc`。

## 包的分层与依赖方向

下层不能导入上层；同一层的包之间，只能按下表"可以导入"一栏的规定互相导入。

| 层 | 包 | 可以导入 |
|---|---|---|
| 0 基础 | `fsutil` `textutil` `doc` `geoip` `prompts` `aitask` `fracindex` | 只能导入标准库和第三方库 |
| 1 基础设施 | `config` `datadir` `store` `jobs` `history` `llm` `httpx` | 第 0 层；`store` 只能被第 2 层及以上导入 |
| 2 业务 | `auth` `settings` `memory` `content` `assets` `videoembed` `i18n/...` `ai/...` `stats` `backup` `setup` | 第 0、1 层；业务包之间需要协作时，由使用方定义接口，在 `app` 里注入实现，不直接互相导入（例外见下） |
| 3 对外 | `api` `site` `webui` | 第 0–2 层 |
| 4 组装 | `app` | 所有包 |

业务层之间允许的直接导入（其他组合一律通过接口）：
- `ai/*` 可以导入 `content`（读文档和块）、`settings`、`assets`（读描述用的文件）、`memory`（取用记忆）。
- `i18n/*` 可以导入 `content`、`settings`、`memory`。
- `ai/qa` 可以导入 `ai/retrieve`、`ai/cache`、`ai/guard`。
- `memory` 可以导入 `content`（生成项目概述时读文档树）、`i18n/translate`（记忆助手写入术语表）。
- `setup` 可以导入 `auth`、`settings`、`backup`。

发布文档之后的连锁动作（建索引、翻译、清缓存）一律通过 `jobs` 投递任务，由 `app` 把任务类型和处理函数对接起来。所以 `content` 不需要知道 `ai`、`i18n` 的存在。

## 根目录

| 文件 | 用途 |
|---|---|
| `AGENTS.md` | Go 端给 AI 助手的说明：分层规则、sqlc 和接口代码生成的流程、错误处理和日志写法 |
| `CLAUDE.md` | 只有一行 `@AGENTS.md` |
| `go.mod` / `go.sum` | Go 模块与依赖 |
| `sqlc.yaml` | sqlc 配置：读取 `internal/store/queries/`，输出到 `internal/store/sqlc/` |
| `oapi-codegen.yaml` | oapi-codegen 配置：读取 `../api/openapi.yaml`，输出到 `internal/api/gen/`，目标是标准库 `net/http` |
| `.golangci.yml` | golangci-lint 配置，其中包括检查包的依赖方向是否符合上表 |

## cmd/ 与 tools/

| 文件 | 用途 |
|---|---|
| `cmd/nice-wiki/main.go` | 程序入口：解析启动参数（`--data`、`--addr`、`--dev`），交给 `app` 启动 |
| `tools/privacycheck/main.go` | 仓库工具：扫描暂存的改动或待推送的提交里的隐私信息（D42），供 git 钩子和 `make check` 调用 |
| `tools/catalogsnap/main.go` | 仓库工具：下载 models.dev 数据，生成 `internal/llm/catalog_snapshot.json` |

## internal/app —— 组装

| 文件 | 用途 |
|---|---|
| `app.go` | 创建所有服务并注入依赖；把任务类型和处理函数对接起来；注册定时任务 |
| `http.go` | 组装 HTTP 路由：公开站点、`/api`、`/admin`、`/static`、`/assets`、`/healthz`，以及全局中间件 |
| `lifecycle.go` | 启动顺序（锁定数据目录 → 迁移 → 检查重置文件 → 启动任务执行器 → 监听端口）和优雅关闭 |

## internal/fsutil —— 文件工具（第 0 层）

| 文件 | 用途 |
|---|---|
| `atomic.go` | 原子写文件：先写临时文件再替换，供所有写 JSON 配置和文档文件的地方使用 |

## internal/fracindex —— 分数索引（第 0 层）

| 文件 | 用途 |
|---|---|
| `fracdex.go` | 从 rocicorp/fracdex 复制的分数索引实现（CC0，公有领域），用于文档树的同级排序。文件开头注明出处 |

## internal/textutil —— 文本工具（第 0 层）

| 文件 | 用途 |
|---|---|
| `bigram.go` | 中日韩文字切成二元组，其他文字按词切分；建索引和查询共用 |
| `normalize.go` | 文本规范化（全半角、空白、大小写），用于查重和计算哈希 |
| `hash.go` | 内容哈希：块、检索单元、文件 |

## internal/doc —— 文档格式与渲染（第 0 层，纯函数，不读写文件）

| 文件 | 用途 |
|---|---|
| `schema.go` | Tiptap 文档的 Go 结构定义；当前格式版本号 `SchemaVersion` |
| `parse.go` | 解析并校验文档 JSON：未知块类型、缺少块 ID、嵌套规则（如超级块不能嵌套） |
| `migrate.go` | 旧格式版本的文档逐级迁移到当前版本 |
| `walk.go` | 遍历文档树的通用工具 |
| `blocks.go` | 拆块：每个块的 ID、父块、类型、纯文本、标题路径、内容哈希 |
| `marks.go` | 行内格式（加粗、斜体、下划线、删除线、行内代码、高亮、链接、行内公式）在三种输出里的渲染 |
| `links.go` | 站内链接解析接口：文档 ID + 语言 → 地址，由 `site` 提供实现 |
| `safeurl.go` | 链接地址过滤：只允许 http、https、mailto、站内链接 |
| `highlight.go` | 代码块的服务端高亮（chroma） |
| `render_html.go` | HTML 渲染入口：遍历文档，按块类型调用对应的实现 |
| `render_text.go` | 纯文本渲染入口（AI 检索和翻译用） |
| `render_md.go` | Markdown 渲染入口（给 AI 爬虫的 `.md` 版本和 `llms-full.txt`） |

### internal/doc/nodes —— 每种块一个文件

每个文件实现同一个接口：HTML、纯文本、Markdown 三种输出，以及"翻译时要提取哪些文字"。**新增块类型就是在这里新增一个文件并在 `registry.go` 里注册**（按 `docs/playbooks/add-block.md` 操作）。

| 文件 | 块 |
|---|---|
| `registry.go` | 块类型接口定义和注册表 |
| `paragraph.go` | 段落、换行 |
| `heading.go` | 标题（H2–H4），生成锚点 |
| `blockquote.go` | 引用 |
| `divider.go` | 分割线 |
| `list.go` | 无序列表、有序列表、列表项 |
| `tasklist.go` | 任务列表 |
| `callout.go` | 提示框（信息、提示、警告、危险） |
| `table.go` | 表格、行、单元格、表头 |
| `codeblock.go` | 代码块（调用 `highlight.go`） |
| `math.go` | 独立公式块；行内公式在 `marks.go` |
| `image.go` | 图片：`srcset`、替代文字、图注、尺寸 |
| `attachment.go` | 附件下载卡片 |
| `drawing.go` | 画板：亮暗两张 SVG |
| `video.go` | 视频：上传的视频、平台视频的封面加播放按钮 |
| `columns.go` | 超级块和栏 |

## internal/prompts —— 提示词（第 0 层）

所有给模型的提示词都是独立的文本文件，集中放在这里，方便统一审阅和调整。

| 文件 | 用途 |
|---|---|
| `prompts.go` | 嵌入下面的文件，按名称取出并填入变量 |
| `router.md` | 前置模型：问题分类、风险判断、改写成基准语言的完整问题、拆分子问题 |
| `verify.md` | 检索员核查：挑出真正相关的块，判断是否需要补充检索 |
| `rerank.md` | 没有重排档时，由快速档模型代替重排 |
| `answer.md` | 聪明模型：只依据材料回答、标注块引用、拒绝无关请求（记忆由 `memory` 统一注入在最前面） |
| `cache-judge.md` | 缓存的 AI 判断模式：候选回答能不能直接复用 |
| `translate-answer.md` | 把缓存里的回答翻译成访客的语言 |
| `context.md` | 入库时给检索单元补一句背景说明 |
| `describe-image.md` | 图片、画板、视频封面的描述（同时用作默认的替代文字） |
| `describe-attachment.md` | 附件的完整描述 |
| `describe-video.md` | 平台视频：根据公开信息生成描述 |
| `translate-blocks.md` | 文档翻译：整篇原文作上下文，只翻译标签圈出的块，按块 ID 返回译文，保留占位标签，遵守术语表 |
| `extract-terms.md` | 自动术语抽取：从文档中抽取关键术语并给出目标语言的译法 |
| `translate-review.md` | 反思润色：对照原文给出修改意见并改写译文 |
| `translate-strings.md` | 站点设置文字和界面文字的翻译 |
| `site-summary.md` | 生成记忆里的"项目概述"条目 |
| `memory-assistant.md` | 记忆助手：先提炼要点，再逐条判断新增、更新、删除或不变；词条写进术语表；按语言分条目；不改只读条目；不存敏感信息；控制单条和每个任务的大小 |
| `slug.md` | 根据标题生成英文路径名 |
| `seo-description.md` | 根据正文生成 SEO 描述 |
| `language-suggest.md` | 根据访客信号判断建议语言 |
| `compact-conversation.md` | 对话压缩：把较早的对话压成摘要，或在原有摘要上更新 |

## internal/aitask —— AI 任务清单（第 0 层）

| 文件 | 用途 |
|---|---|
| `tasks.go` | 所有用到模型的任务清单（唯一定义处）：任务名称、默认档位、默认取用的记忆条目、默认的输出上限。`llm`（选模型）和 `memory`（取记忆）共用 |

## internal/geoip —— IP 归属地（第 0 层）

| 文件 | 用途 |
|---|---|
| `geoip.go` | 查询 IP 所属国家：用 maxminddb-golang 读取 DB-IP Lite 的 mmdb 文件 |
| `update.go` | 每月下载新版数据库到 `data/cache/geoip/`。DB-IP Lite 是 CC BY 4.0 许可，站点页脚需要注明数据来源 |

## internal/config —— 配置（第 1 层）

| 文件 | 用途 |
|---|---|
| `bootstrap.go` | 启动参数、环境变量、`data/config/config.toml`：监听地址、开发模式、日志级别 |
| `runtime.go` | `data/config/runtime.json`：超管在后台修改的系统设置，包括可信代理、Turnstile 站点密钥、每日花费上限、data 仓库的远程地址、备份计划 |
| `secrets.go` | `data/config/secrets.json`：各模型渠道的密钥、Turnstile 私钥、远程仓库凭据。只能写入和使用，不能读出返回给前端 |

## internal/datadir —— 数据目录（第 1 层）

| 文件 | 用途 |
|---|---|
| `layout.go` | `data/` 下所有路径的唯一定义处（见 README 的"数据目录"一节），其他包不能自己拼路径 |
| `init.go` | 首次启动时创建目录结构，写入 `data/.gitignore` |
| `lock.go` | 文件锁：防止两个进程同时使用同一个数据目录 |

## internal/store —— 数据库（第 1 层）

| 文件 | 用途 |
|---|---|
| `db.go` | 打开 SQLite（modernc.org/sqlite），开启 WAL 等参数，配置连接 |
| `migrate.go` | 启动时按顺序执行 `migrations/` 里尚未执行的迁移 |
| `tx.go` | 事务辅助函数 |
| `sqlc/` | **[生成]** sqlc 生成的代码，不能手改 |

### internal/store/migrations —— 数据库迁移（初始版本）

| 文件 | 包含的表 |
|---|---|
| `0001_auth.sql` | 登录会话、登录失败记录、操作日志 |
| `0002_content.sql` | 文档索引、块、资源文件、资源引用关系 |
| `0003_jobs.sql` | 任务合并标记（同一篇文档的重复任务只保留一个）。任务本身的表由 Backlite 自己创建 |
| `0004_search.sql` | 检索单元、单元与块的对应、向量、关键词全文索引（FTS5）、图片和附件描述的缓存 |
| `0005_qa.sql` | 对话、问答记录、反馈、回答缓存、各语言的缓存回答、精选回答 |
| `0006_llm.sql` | 模型调用用量与花费、语言判断结果的缓存、模型目录缓存、调试台的中间结果 |
| `0007_guard.sql` | IP 封禁、可疑事件 |
| `0008_i18n.sql` | 译文状态索引（从译文文件派生，供进度统计用） |
| `0009_stats.sql` | 浏览明细、每日汇总、每日的盐 |
| `0010_memory.sql` | 记忆助手的对话、待确认的修改提议 |

### internal/store/queries —— sqlc 查询

和迁移按同样的领域划分，一个领域一个文件：`auth.sql`、`content.sql`、`jobs.sql`、`search.sql`、`qa.sql`、`llm.sql`、`guard.sql`、`i18n.sql`、`stats.sql`、`memory.sql`。

## internal/jobs —— 后台任务（第 1 层）

| 文件 | 用途 |
|---|---|
| `kinds.go` | 所有任务类型的名称常量和各自的队列配置（重试次数、退避、超时、并发）（唯一定义处）：建索引、翻译、生成描述、抓视频信息、项目概述更新、目录更新、GeoIP 更新、备份、过期清理、统计汇总、全量重建索引、全站翻译 |
| `queue.go` | 封装 Backlite（D51）：启动时安装它的表；投递任务前检查合并标记，同一篇文档的重复任务合并成一个；查询任务进度供后台显示 |
| `worker.go` | 把各任务类型的处理函数注册到 Backlite，并在任务结束时清除合并标记 |
| `schedule.go` | 定时任务：按计划投递任务 |

## internal/history —— 版本历史（第 1 层）

| 文件 | 用途 |
|---|---|
| `repo.go` | 打开或初始化 `data/` 里的 git 仓库（go-git） |
| `commit.go` | 发布时提交，作者为执行发布的后台账号 |
| `log.go` | 某篇文档的历史版本列表；公开页面的"最后更新于" |
| `diff.go` | 两个版本之间的差异 |
| `restore.go` | 取出历史版本的内容（由 `content` 生成新草稿） |
| `remote.go` | 推送到远程仓库做异地备份 |

## internal/llm —— 模型渠道（第 1 层）

| 文件 | 用途 |
|---|---|
| `channels.go` | 渠道配置（`data/config/channels.json`）的增删改查；密钥通过 `config/secrets.go` 存取 |
| `catalog.go` | 模型目录：加载内置快照、每天更新、合并手动覆盖的资料 |
| `catalog_snapshot.json` | **[生成]** models.dev 数据快照，由 `make catalog` 更新，编译时嵌入 |
| `models.go` | 模型资料和能力标签（对话、看图、结构化输出、推理、嵌入、重排） |
| `tiers.go` | 档位和任务的模型解析：任务（来自 `aitask`）→ 档位 → 有序的模型列表；分配时的能力检查 |
| `client.go` | 统一的调用入口：按任务选模型、失败时切换备用模型、重试与退避、每个渠道的并发上限、超时 |
| `structured.go` | 结构化输出：原生支持就直接用，否则提示词要求 JSON，再校验并重试 |
| `cost.go` | 计费：用量 × 单价，或者使用渠道返回的实际金额；写入用量表 |
| `probe.go` | 测试连接；查询渠道的模型列表 |

### internal/llm/protocol —— 协议适配器

| 文件 | 用途 |
|---|---|
| `protocol.go` | 协议接口：对话、流式对话、嵌入、重排；统一的请求、响应、用量结构 |
| `openai.go` | OpenAI Chat Completions 兼容协议 |
| `anthropic.go` | Anthropic Messages 协议 |
| `gemini.go` | Google Gemini 协议 |
| `embed_openai.go` | 嵌入：OpenAI `/embeddings` 兼容 |
| `embed_gemini.go` | 嵌入：Gemini |
| `rerank.go` | 重排：`/rerank` 风格（Cohere、Jina 格式） |

## internal/httpx —— HTTP 通用（第 1 层）

| 文件 | 用途 |
|---|---|
| `realip.go` | 从可信代理的请求头里读取访客真实 IP |
| `security.go` | 安全响应头：CSP 等 |
| `logging.go` | 请求日志（不记录访客提问内容） |
| `recover.go` | 捕获 panic，返回 500 |
| `sse.go` | SSE 流式响应：事件写入、心跳、`X-Accel-Buffering: no` |
| `errors.go` | 统一的错误响应格式 |

## internal/auth —— 账号与权限（第 2 层）

| 文件 | 用途 |
|---|---|
| `accounts.go` | `data/config/accounts.json`：账号的增删改查、停用 |
| `password.go` | argon2id 哈希；密码规则（至少 8 位，不能和旧密码相同） |
| `register.go` | 系统里没有账号时，注册第一个账号为超管 |
| `login.go` | 登录；同一账号或同一 IP 连续失败时临时锁定 |
| `sessions.go` | 登录会话（存在 SQLite）、cookie、退出所有设备 |
| `permissions.go` | 10 项权限的常量、超管专属操作清单、权限检查函数 |
| `middleware.go` | 要求登录、要求权限、"必须改密码"时只放行改密码接口、检查 `Origin` 防 CSRF |
| `reset.go` | 启动时和运行中检测 `reset-superadmin.txt`，重置超管密码后删除该文件 |
| `audit.go` | 写入和查询操作日志 |

## internal/settings —— 站点设置（第 2 层）

存放在 `data/settings/`，纳入 git。

| 文件 | 用途 |
|---|---|
| `store.go` | 读写设置文件；修改后投递翻译任务、清除渲染缓存 |
| `site.go` | 站点名称、logo、网站图标、主题色、页脚（版权、链接、备案号、数据来源署名、源代码链接）、默认分享图 |
| `home.go` | 首页主视觉：主标题、背景字、副标题、按钮、背景类型和文件、遮罩、文字颜色 |
| `banner.go` | 公告横幅：文字、链接、起止时间 |
| `languages.go` | 基准语言、已启用的语言；各语言的反思润色开关；翻译批次大小、翻译记忆的长度门槛 |
| `ai.go` | 档位和任务的模型分配、缓存配置、防滥用配置（各项上限、按花费限流的开关和数值）、对话压缩的设置 |

## internal/setup —— 初始化（第 2 层）

| 文件 | 用途 |
|---|---|
| `state.go` | 判断初始化进行到哪一步：未注册 → 站点信息 → 语言 → 模型渠道 → 完成 |
| `restore.go` | 初始化向导里"从备份恢复" |

## internal/content —— 文档（第 2 层）

| 文件 | 用途 |
|---|---|
| `docfile.go` | 文档文件的格式：元数据（标题、路径名、SEO 描述、分享图、同级排序位置、格式版本号）+ 正文 |
| `repo.go` | 读写 `data/docs/` 下的文档文件 |
| `tree.go` | 文档树：父子关系、同级排序（用 `fracindex` 的分数索引，移动一篇只改它自己的文件）、移动 |
| `drafts.go` | 草稿：自动保存到 `data/drafts/`，用版本号检测冲突 |
| `publish.go` | 发布：校验 → 首次发布时生成路径名和 SEO 描述 → 写入正文 → 更新索引 → git 提交 → 投递后续任务 |
| `unpublish.go` | 撤回发布 |
| `trash.go` | 回收站：删除、恢复、彻底删除（超管） |
| `slugs.go` | 路径名校验、同级唯一、拼接完整路径 |
| `redirects.go` | `data/redirects.json`：旧路径的 301 跳转、已删除文档的 410 |
| `locks.go` | 编辑锁：获取、心跳、超时释放、超管强制释放（只存在内存里） |
| `index.go` | 把文档和块写进数据库索引；启动时如果发现索引缺失，从文件全量重建 |
| `generators.go` | 本包需要、由外部注入的接口：路径名生成器、SEO 描述生成器、链接解析器 |

## internal/assets —— 资源文件（第 2 层）

| 文件 | 用途 |
|---|---|
| `types.go` | 允许上传的文件类型，以及各类型的大小上限 |
| `store.go` | 内容寻址存储：`data/assets/<类型>/<sha256>.<后缀>` |
| `upload.go` | 上传：识别类型、检查大小、接收浏览器生成的图片多尺寸版本 |
| `serve.go` | 返回文件：支持 Range 请求、永久缓存、SVG 加 CSP sandbox |
| `refs.go` | 引用关系：从块里提取引用了哪些文件，查询某个文件被谁引用 |
| `cleanup.go` | 列出当前没有被引用的文件；手动删除 |

## internal/videoembed —— 平台视频（第 2 层）

| 文件 | 用途 |
|---|---|
| `provider.go` | 提供方接口：识别链接、生成嵌入地址、抓取公开信息 |
| `registry.go` | 注册提供方，按链接匹配 |
| `youtube.go` | YouTube（oEmbed，`youtube-nocookie.com` 播放器） |
| `bilibili.go` | Bilibili（公开视频信息接口，`player.bilibili.com` 播放器） |
| `tiktok.go` | TikTok（oEmbed） |
| `douyin.go` | 抖音（页面公开元信息，开放平台播放器） |
| `thumbnail.go` | 下载封面图存进 `assets/` |

## internal/i18n —— 多语言（第 2 层）

| 文件 | 用途 |
|---|---|
| `languages.go` | 内置语言清单：代码、本地名称、书写方向、匹配别名（如 `zh-TW` → 繁体） |
| `accept.go` | 解析 `Accept-Language`，匹配已启用的语言：用 `golang.org/x/text/language` 的 Matcher，不自己写匹配规则 |

### internal/i18n/translate —— 翻译流水线

| 文件 | 用途 |
|---|---|
| `pipeline.go` | 文档翻译任务：找出要翻译的块 → 翻译记忆复用 → 首次翻译时抽取术语 → 分批 → 逐批翻译 → 校验与降级 → 术语检查 → 可选的反思润色 → 写入（D47） |
| `batch.go` | 按预估输出量分批（默认约 1000 token，可调）：只在块边界切、各批均衡、优先在标题处切；超大的块按行拆开 |
| `reuse.go` | 翻译记忆：原文哈希相同且长度达到门槛（默认 20 字）的块，复用已有译文 |
| `terms.go` | 自动术语抽取：首次翻译某文档到某语言前抽取术语，写入全站术语表（来源标记为自动） |
| `check.go` | 术语表符合度检查：不符合时带提醒重译一次，仍不符合则标记待复核 |
| `review.go` | 可选的反思润色：按语言开启，默认关闭 |
| `inline.go` | 行内格式 ↔ 带编号的占位标签，链接地址和行内代码不交给模型 |
| `validate.go` | 校验译文：块 ID 齐全、占位标签完整、译文不等于原文；不通过时对半切分重试，再逐块翻译；撞到长度上限时减少上下文 |
| `files.go` | 读写 `data/i18n/<语言>/<文档ID>.json`：译文、原文哈希、来源（机器 / 人工）、状态 |
| `glossary.go` | 术语表 `data/i18n/glossary.json`：词条来源（手动 / 自动）、手动优先、按来源筛选和整批撤销、只取原文中出现的词条 |
| `strings.go` | 站点设置文字（`_site.json`）和界面文字（`_ui.json`）的翻译 |
| `status.go` | 每种语言的翻译进度、待复核的人工译文；同步到数据库的状态索引 |
| `estimate.go` | 启用新语言、全站重新翻译之前的费用预估 |

### internal/i18n/suggest —— 语言切换建议

| 文件 | 用途 |
|---|---|
| `signals.go` | 整理访客的信号：浏览器语言列表、时区、IP 所属国家 |
| `decide.go` | 交给快速档模型判断；按信号组合缓存结果；超时或出错时使用兜底规则 |
| `rules.go` | 兜底规则 |

## internal/ai —— AI 问答（第 2 层）

### internal/ai/ingest —— 入库

| 文件 | 用途 |
|---|---|
| `pipeline.go` | 建索引任务：对比检索单元的哈希，只处理变化了的部分 |
| `units.go` | 组装检索单元：同一小节的文本块合并；图片、画板、附件、视频各自成为单元，附带上下两个块 |
| `contextual.go` | 给每个单元补一句背景说明 |
| `describe.go` | 生成图片、画板、视频封面、附件的描述，按 sha256 缓存；图片描述同时作为默认替代文字 |
| `extract.go` | 抽取附件的文字：纯文本类直接读取；PDF 等交给视觉档里支持文件输入的模型 |
| `embed.go` | 生成向量并写入数据库 |
| `keyword.go` | 写入关键词全文索引（二元组切分） |
| `reindex.go` | 全量重建索引（更换嵌入模型时），包括费用预估 |

### internal/ai/retrieve —— 检索

| 文件 | 用途 |
|---|---|
| `vectors.go` | 内存向量索引：启动时从数据库加载、增量更新、暴力计算余弦相似度 |
| `keyword.go` | 关键词召回（FTS5 查询） |
| `fuse.go` | RRF 合并两路召回 |
| `rerank.go` | 重排：使用重排档；没有配置时由快速档模型代替 |
| `retriever.go` | 检索员：召回 → 重排 → 核查，最多 2–3 轮 |
| `tools.go` | 检索员可以调用的工具：补充检索、读取所在小节、查看文档目录树（参考 PageIndex，D51） |
| `section.go` | 把命中的块扩展到它所在的小节 |

### internal/ai/qa —— 问答流程

| 文件 | 用途 |
|---|---|
| `service.go` | 问答总流程：防滥用 → 前置模型 → 查缓存 → 并行检索 → 组装 → 回答 → 记录 |
| `router.go` | 前置模型：分类、风险判断、改写、拆分子问题、判断改写后的问题是否意思完整（决定能否使用缓存） |
| `pagecontext.go` | 感知当前页面：从文档页提问时，把当前文档作为优先参考 |
| `assemble.go` | 组装材料：去重、扩展到小节、按文档顺序排列、优先取访客语言的译文 |
| `answer.go` | 聪明模型生成回答，标注块引用 |
| `events.go` | 流式事件的类型：进度、文字片段、引用、完成、错误 |
| `conversations.go` | 服务端保存对话；访客清空；保留期 |
| `compact.go` | 对话的自动压缩：照抄 opencode 的机制和默认值，接近上下文上限时把较早的对话压成摘要并增量更新，输入太长被拒绝时压缩后重试一次 |
| `records.go` | 问答记录；保留期清理 |
| `feedback.go` | "有帮助 / 没帮助" |
| `debug.go` | 调试台：记录并返回每一步的中间结果、花费、耗时 |

### internal/ai/cache —— 回答缓存

| 文件 | 用途 |
|---|---|
| `lookup.go` | 精确匹配（问题规范化后的哈希，对话第一问在前置模型之前就匹配）；语义匹配的两种方式：AI 判断（默认）、相似度阈值；只处理意思完整的问题 |
| `store.go` | 写入缓存，按语言分别保存回答 |
| `invalidate.go` | 失效：过期、引用的块内容变化、无引用的回答在任何发布之后失效、更换嵌入模型时整体清空、记忆版本变化 |
| `calibrate.go` | 指标与校准：命中率、疑似误命中（缓存回答被点"没帮助"）、根据相似度分布给出阈值建议 |
| `translate.go` | 命中时翻译成访客的语言并保存 |
| `curated.go` | 精选回答：不过期，命中时优先 |

### internal/ai/guard —— 防滥用

| 文件 | 用途 |
|---|---|
| `turnstile.go` | 校验 Turnstile：无感验证和交互式挑战 |
| `pow.go` | 工作量证明：Turnstile 加载失败时的备用验证，出题、校验、可疑时提高难度（D50） |
| `limits.go` | 各项上限：问题长度、子问题数量、检索轮数和工具调用次数、每个会话同时只处理一个问题；按次数的频率限制用 go-limiter（D50、D51） |
| `session.go` | 问答会话的签名 cookie |
| `checks.go` | 程序检查：提问频率、和最近问题的重复度、请求间隔是否规律 |
| `risk.go` | 综合程序检查和前置模型的判断，得出风险等级 |
| `bans.go` | IP 封禁 10 分钟（IPv6 按 /64）；手动解封 |
| `budget.go` | 花费限制：整站每日上限；每个会话、每个 IP 每小时的上限（可以整体关闭） |
| `events.go` | 可疑事件记录 |

### internal/ai/meta

| 文件 | 用途 |
|---|---|
| `meta/slug.go` | 实现 `content` 需要的路径名生成器 |
| `meta/seo.go` | 实现 `content` 需要的 SEO 描述生成器 |

## internal/memory —— 长期记忆（第 2 层）

独立模块，详见 `docs/design/memory.md`。记忆条目存放在 `data/memory/`，纳入 git。

| 文件 | 用途 |
|---|---|
| `items.go` | 记忆条目的结构：标题、说明、适用语言、来源（手动 / 助手 / 自动）、单条上限（默认 1.5KB）、是否只读；内置条目清单 |
| `store.go` | 读写 `data/memory/*.md`（含开头的元数据），修改后提交到 data 的 git 仓库 |
| `mapping.go` | 每个任务取用哪些条目（`data/memory/_tasks.json`），默认值来自 `aitask` |
| `inject.go` | 按任务（和目标语言）拼出要注入的记忆文本，每个条目前带上它的说明；给出版本号（供缓存判断用） |
| `budget.go` | 检查单个条目是否超过它的上限、每个任务的注入总量是否超过 5KB，超出时拒绝保存并指出原因 |
| `assistant.go` | 记忆助手：对话、可调用的工具（读写条目、写术语表、查看注入量） |
| `proposals.go` | 助手提出的修改：逐条标记为新增、更新、删除、不变，生成差异，等待确认后写入；只读条目不能被修改 |
| `overview.go` | 定期更新自动维护的"项目概述"条目；手动修改过的不覆盖 |

## internal/stats —— 访问统计（第 2 层）

| 文件 | 用途 |
|---|---|
| `beacon.go` | 接收页面发来的浏览记录 |
| `visitor.go` | 每天更换的盐，以及独立访客哈希 |
| `bots.go` | 按 User-Agent 识别爬虫，单独区分 AI 爬虫 |
| `rollup.go` | 每日汇总；明细的保留期清理 |
| `query.go` | 统计查询：浏览量、访客、热门文档、来源、国家、语言、AI 爬虫读取量 |

## internal/backup —— 备份（第 2 层）

| 文件 | 用途 |
|---|---|
| `export.go` | 打包 `data/`：SQLite 用自带的备份接口生成一致的快照；可以选择是否包含密钥 |
| `schedule.go` | 定时备份到 `data/backups/`，保留最近 N 份 |
| `restore.go` | 校验并解开备份包 |

## internal/api —— JSON 接口（第 3 层）

| 文件 | 用途 |
|---|---|
| `gen/` | **[生成]** oapi-codegen 根据 `api/openapi.yaml` 生成的类型和路由接口，不能手改 |
| `server.go` | 实现生成的接口，把各领域的处理函数组合起来 |
| `permissions.go` | 每个接口需要的权限（唯一定义处）；权限测试遍历它 |
| `errors.go` | 业务错误映射成 HTTP 错误 |

后台接口（`/api/admin/...`），每个文件对应 `openapi.yaml` 里的一个标签：

| 文件 | 接口 |
|---|---|
| `admin_setup.go` | 初始化向导、注册超管、从备份恢复 |
| `admin_auth.go` | 登录、退出、修改密码、退出所有设备、当前账号信息 |
| `admin_accounts.go` | 账号管理（超管） |
| `admin_audit.go` | 操作日志（超管） |
| `admin_docs.go` | 文档树、草稿、发布、撤回、预览、属性、编辑锁 |
| `admin_history.go` | 历史版本、差异、回滚 |
| `admin_trash.go` | 回收站 |
| `admin_assets.go` | 上传、资源库、引用关系、清理 |
| `admin_video.go` | 识别平台视频链接、抓取公开信息 |
| `admin_i18n.go` | 语言启用、翻译进度、对照编辑、术语表、费用预估 |
| `admin_models.go` | 渠道（超管）、模型目录、档位和任务分配、测试连接 |
| `admin_qa.go` | 问答记录、回答缓存设置、精选回答、调试台、重建索引 |
| `admin_guard.go` | 封禁列表、可疑事件 |
| `admin_memory.go` | 长期记忆：条目的增删改、各任务取用的条目和注入量、记忆助手对话（SSE）、确认或拒绝修改提议 |
| `admin_settings.go` | 站点设置、首页主视觉、公告横幅 |
| `admin_system.go` | 系统设置（超管）：可信代理、Turnstile、花费上限、远程仓库 |
| `admin_backup.go` | 备份、下载、定时备份设置（超管） |
| `admin_stats.go` | 仪表盘和访问统计 |
| `admin_jobs.go` | 任务队列：查看、重试 |

公开接口（`/api/public/...`）：

| 文件 | 接口 |
|---|---|
| `public_ask.go` | 问答（SSE 流式）、清空对话、读取对话 |
| `public_session.go` | 用 Turnstile 或工作量证明的结果换取问答会话；下发工作量证明的题目 |
| `public_feedback.go` | 回答反馈 |
| `public_locale.go` | 语言切换建议 |
| `public_beacon.go` | 浏览统计 |

## internal/site —— 公开站点（第 3 层）

| 文件 | 用途 |
|---|---|
| `routes.go` | 公开站点的路由表 |
| `langroute.go` | 根路径 `/` 的语言跳转（cookie → `Accept-Language` → 基准语言）；解析语言前缀 |
| `resolve.go` | 路径解析：文档、首页、问答页，或者 301 跳转、410、404 |
| `links.go` | 实现 `doc` 的站内链接解析：文档 ID + 语言 → 地址 |
| `page.go` | 组装页面数据：公告横幅、顶栏、文档树、本页目录、上一篇和下一篇、页脚 |
| `nav.go` | 每种语言的文档树；上一篇、下一篇 |
| `toc.go` | 本页目录 |
| `seo.go` | title、描述、Open Graph、canonical、hreflang、JSON-LD 面包屑 |
| `islands.go` | 输出 Vue 组件的挂载点和初始数据 |
| `render.go` | 执行模板；ETag 与 304 |
| `cache.go` | 渲染结果的内存缓存，按"文档 + 语言"存放，发布后失效 |
| `home.go` | 首页 |
| `ask.go` | 独立问答页 |
| `errors.go` | 404、410 页面 |
| `preview.go` | 草稿预览（需要登录，页面带 `noindex`） |
| `markdown.go` | 每个页面的 `.md` 版本；HTML 页面的响应头里加上 `Link` 指向它（llms.txt 规范的建议） |
| `llms.go` | `llms.txt`、各语言的 `llms.txt` 和 `llms-full.txt` |
| `sitemap.go` | sitemap 总索引和各语言的 sitemap |
| `robots.go` | `robots.txt`（明确允许 AI 爬虫） |

### internal/site/templates —— Go 模板（嵌入程序）

| 文件 | 用途 |
|---|---|
| `layout.html` | 所有公开页面的外框 |
| `home.html` | 首页主视觉 |
| `doc.html` | 文档页：三栏布局 |
| `ask.html` | 独立问答页 |
| `error.html` | 404 和 410 |
| `partials/head.html` | `<head>`：SEO 信息、样式和脚本、主题初始化 |
| `partials/banner.html` | 公告横幅 |
| `partials/topbar.html` | 顶栏：站点名称、AI 入口、语言切换、亮暗切换、手机端的抽屉按钮 |
| `partials/tree.html` | 文档树 |
| `partials/toc.html` | 本页目录 |
| `partials/prevnext.html` | 上一篇、下一篇 |
| `partials/footer.html` | 页脚 |
| `partials/untranslated.html` | 页面顶部的"部分内容尚未翻译"提示 |
| `partials/island.html` | Vue 组件挂载点的通用写法 |

## internal/webui —— 前端产物（第 3 层）

| 文件 | 用途 |
|---|---|
| `embed.go` | 嵌入 `dist/` |
| `manifest.go` | 读取 Vite 的 manifest，查出带哈希的文件名；开发模式下改为指向 Vite 开发服务器 |
| `admin.go` | 返回后台单页应用的入口页，`/admin/` 下的所有路径都返回它 |
| `dist/` | **[生成，不入库]** 前端构建产物；只提交一个 `.gitkeep`，保证前端没构建时也能编译 |

## 测试文件 **[测试]**

以下文件**编写前必须征得用户同意**（D38），这里只规定放在哪里：
- Go 测试和被测代码放在一起：`xxx.go` 对应 `xxx_test.go`。
- 各包测试用的数据放在本包的 `testdata/` 目录下。
- 外部服务的假实现放在各自包里的 `fake.go`（例如 `llm/fake.go`、`ai/guard/fake.go`），只供测试使用。

### 2026-10-13 已授权的最小离线回归测试

用户已确认以下新增文件；使用临时目录、SQLite 和模型替身，不访问真实模型。

| 文件 | 用途 |
|---|---|
| `internal/ai/guard/budget_test.go` | 预留、结算、并发上限和动态配置 |
| `internal/ai/guard/pow_test.go` | 地址编码、来源绑定、过期和重放 |
| `internal/i18n/translate/files_test.go` | 人工保护、行内格式和属性回填 |
| `internal/i18n/translate/pipeline_test.go` | 人工译文、并发保存和旧版本拒绝 |
| `internal/backup/export_test.go` | 快照往返、运行文件排除、失败保护和非法包拒绝 |
| `internal/setup/restore_test.go` | 数据卷根目录保持、进程锁和失败回滚 |
| `internal/ai/qa/service_test.go` | 开关、预算、缓存事件、上下文、多语言、动态模型和撤回清理 |
| `internal/stats/rollup_test.go` | 人类与爬虫分离、跨语言去重和重复汇总 |
