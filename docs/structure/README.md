# 文件结构总则

> 对应决策记录 D45。

这一组文档规定了仓库里**每一个文件的用途**，精确到文件，不精确到函数。写代码的 AI 助手只需要按照用途"填空"。

| 文档 | 范围 |
|---|---|
| [root.md](root.md) | 根目录、`.githooks/`、`api/`、`deploy/`、`fixtures/`、`evals/`、`docs/` |
| [server.md](server.md) | Go 后端 `server/` |
| [web.md](web.md) | 前端 `web/` |

## 规则（硬性规定，写进 AGENTS.md）

1. **只能创建、修改这里列出的文件。**
2. **增加、删除、改名、移动任何文件或目录，都必须先询问用户。** 包括：觉得某个文件太大想拆开、想新建一个工具文件、想调整目录层级。询问时说明：要改什么、为什么、影响哪些文件。具体步骤见 `docs/playbooks/change-structure.md`。
3. 用户同意后，在**同一个提交里**同步修改这里的结构文档；影响较大的，另外写一条决策记录。
4. **文件的职责只能按这里写的用途来写。** 发现某段代码不属于任何已有文件的职责时，同样先询问，不能随便找个文件塞进去。
5. 标记为 **[生成]** 的文件和目录，由工具生成，不能手改。
6. 标记为 **[测试]** 的文件，编写前必须征得用户同意（D38）。

### 可以直接新增、不必询问的情况 ✅

以下几类文件天然会不断增加，AI 助手可以**按约定的命名**直接新增，不必每次询问：
- 决策记录 `docs/adr/NNNN-<短名>.md`，以及索引里的对应行；push 记录 `docs/push-log/YYYY-MM-DD-NNNN.md`
- 数据库迁移 `server/internal/store/migrations/NNNN_<短名>.sql`：编号接着上一个，只能追加，不能修改已有的迁移；对应的查询写进 `queries/` 里已有的文件
- 生成目录里的文件：sqlc、oapi-codegen、openapi-typescript 的输出

**不在此列**、仍然要先询问的：shadcn-vue 组件、提示词文件、块类型文件、视频提供方、协议适配器，以及其他任何新文件。

## 标记说明

| 标记 | 含义 |
|---|---|
| **[生成]** | 由工具生成，不能手改。修改的方法是改它的来源，再重新生成 |
| **[生成，不入库]** | 由工具生成，而且不提交进 git |
| **[测试]** | 测试相关的文件，编写或运行前必须征得用户同意 |

## 顶层目录

```
nice-wiki/
├── AGENTS.md  CLAUDE.md  STATUS.md  README.md  CHANGELOG.md  LICENSE
├── Makefile  .gitignore  .gitattributes  .editorconfig
├── .githooks/        git 钩子：隐私扫描、提交信息检查
├── api/              前后端接口定义（openapi.yaml）
├── server/           Go 后端                          → server.md
├── web/              Vue 3 前端                        → web.md
├── deploy/           Docker、反向代理示例、部署指南
├── fixtures/         样例文档 [测试]
├── evals/            AI 质量评测 [测试]
└── docs/             计划、决策记录、设计、文件结构、操作手册、push 记录
```

## 数据目录（运行时生成，不在代码仓库里）

在 D8 的基础上细化。所有路径都只在 `server/internal/datadir/layout.go` 里定义。

```
data/
├── .git/  .gitignore     data 自己的 git 仓库，用于版本历史（D16）
├── wiki.db               SQLite：索引、会话、任务、问答记录、统计……
├── docs/                 [git] 基准语言正文，目录层级即文档树
│   ├── <id>.json
│   └── <id>/             这篇文档的子文档
├── i18n/                 [git]
│   ├── glossary.json     术语表
│   └── <lang>/
│       ├── <id>.json     某篇文档的译文
│       ├── _site.json    站点设置文字的译文
│       └── _ui.json      公开站点界面文字的译文
├── settings/             [git] site.json  home.json  banner.json  languages.json  ai.json
├── memory/               [git] 长期记忆：每个条目一个 .md，_tasks.json 记录各任务取用哪些条目
├── redirects.json        [git] 旧路径的 301 跳转、已删除文档的 410
├── drafts/               草稿
├── trash/                回收站
├── assets/               images/  files/  videos/  drawings/，按 sha256 命名
├── backups/              定时备份
├── cache/                可以随时删除：geoip/、模型目录的更新
└── config/               只有超管能改，绝不进 git
    ├── config.toml       启动配置：监听地址、开发模式、日志级别
    ├── runtime.json      超管在后台修改的系统设置：可信代理、Turnstile 站点密钥、花费上限、远程仓库、备份计划
    ├── secrets.json      密钥：各模型渠道、Turnstile 私钥、远程仓库凭据
    ├── channels.json     模型渠道（不含密钥）
    ├── accounts.json     后台账号
    └── reset-superadmin.txt   只在重置超管密码时临时存在
```

标 [git] 的会纳入 data 的 git 仓库，其余的不纳入。

## 设计文件结构时顺带定下的小决定 ✅

以下几点是在设计文件结构时为了让结构成立而定下的，用户已全部确认：

1. **Go 端的渲染按"块类型一个文件"组织**（`internal/doc/nodes/`），每个文件同时负责 HTML、纯文本、Markdown 三种输出。新增块类型只需要新增一个文件并注册，不会漏改某一种输出。
2. **提示词集中成独立的文本文件**（`internal/prompts/*.md`），方便统一审阅和修改，不散落在代码里。
3. **公开页面的轻交互用原生 TypeScript**，只有问答和语言建议弹窗用 Vue，公开页面保持轻量。
4. **后台界面只有中文**（✅ 用户已确认）。文字直接写在组件里，不做后台的多语言；公开站点的界面文字照常走翻译流程。
5. **配置文件拆成几份**：`config.toml`（启动参数）、`runtime.json`（超管在后台改的系统设置）、`secrets.json`（密钥）、`channels.json`（模型渠道）、`accounts.json`（账号）。站点设置放在 `data/settings/`，纳入 git，可以回滚。
6. **文档的同级排序用"分数索引"**，存在每篇文档自己的元数据里。移动一篇文档只改它自己的文件，git 记录也干净。
7. **渲染缓存只放在内存里**，启动后按需重新生成。几百篇文档乘以几种语言，内存完全放得下。
8. **已删除文档的 410 记录和 301 跳转放在同一个文件里**（`redirects.json`）。
9. **IP 归属地用 DB-IP Lite**。它是 CC BY 4.0 许可，需要在页脚注明数据来源。
10. **仓库工具（隐私扫描、模型目录快照）用 Go 写**，放在 `server/tools/`，不依赖 shell 脚本，各个平台都能运行。
11. **Excalidraw 用 `createElement` 挂载**，前端项目不需要为了它引入 JSX 和 React 的构建插件。
12. **端到端测试放在 `web/e2e/`**，直接用前端的依赖，不单独建一个 Node 项目。
