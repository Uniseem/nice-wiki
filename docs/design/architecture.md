# 整体架构（草案）

> 状态：讨论中。对应决策记录 D2、D14–D16。

## 一个 Go 程序全包

```
一个 Go 程序
├── /                    根据访客语言跳转
├── /<lang>/...          公开页面：Go 输出完整 HTML
│                        └── Vue 局部组件：AI 问答、语言弹窗、目录……
├── /admin/...           后台：Vue 单页应用
├── /api/...             JSON 接口（后台 + 公开组件共用）
└── /assets/...          图片、附件（内容寻址，永久缓存）

部署：一个可执行文件 + data/ 目录
```

## 公开页面渲染

- 文档发布时，Go 把 Tiptap JSON 渲染成 HTML，按"文档 + 语言"缓存。访客请求直接返回缓存。
- 页面外框（导航、目录树、页脚）用 Go 模板（`html/template`）。
- 有交互的部分在页面上留出挂载点，由 Vue 组件接管。

### 块的渲染要保持一致

同一种块在两个地方各渲染一次：编辑器里由 Tiptap 渲染，公开页面上由 Go 渲染。保证两边一致的做法：

1. 共用同一套 CSS 类名和样式表（放在 `web/src/shared/`）。
2. 做对照测试：维护一批示例文档，用 Tiptap 官方的 `generateHTML` 生成标准 HTML，和 Go 渲染出的结果比对。

## 前端：一个 Vite 项目，两个入口

| 入口 | 用途 | 产物 |
|---|---|---|
| `admin` | 后台单页应用 | 带 `index.html` 的完整应用，挂在 `/admin` |
| `site` | 公开页面上的 Vue 组件 + 公共样式 | 只有 JS 和 CSS，由 Go 模板引用 |

- 构建时，Vite 直接把产物输出到 `server/internal/webui/dist/`，Go 用 `go:embed` 打进程序。
- Vite 开启 manifest，Go 读取它来引用带哈希的资源文件名。

### 开发模式

- 后台：Vite 开发服务器提供后台页面，`/api` 请求转发给 Go。
- 公开页面：由 Go 提供，页面里的脚本指向 Vite 开发服务器，这样也有热更新。参考 Vite 官方的"后端集成"做法。

## 草稿与发布

```
编辑 ──自动保存──▶ 草稿（data/drafts/）        访客看不到
         │
         └──点"发布"──▶ 正文（data/docs/）
                          ├─▶ 渲染公开页面 HTML 并缓存
                          ├─▶ git 提交（版本历史）
                          ├─▶ 更新 AI 索引（后台任务）
                          └─▶ 翻译到各目标语言（后台任务）
```

- 翻译和建索引都要花钱，所以只在发布时触发，自动保存时不触发。
- 后台任务需要一个任务队列，存在 SQLite 里，支持失败重试和查看进度。

## 版本历史

`data/` 目录本身是一个 git 仓库，用纯 Go 实现的 go-git 操作，不依赖系统里的 git。

- 每次发布提交一次，提交信息写明发布了哪篇文档，作者就是执行发布的账号。
- 后台可以查看某篇文档的历史版本、对比差异、回滚。
- 可以配置一个远程仓库，定期推送，当作异地备份。

纳入 git 的：`docs/`、`i18n/`、术语表、站点设置。

不纳入 git 的：
- `wiki.db`：派生数据，备份方式另外处理。
- `config/`：配置、密钥、账号。
- `drafts/`：草稿频繁变化，没必要记录。
- `assets/`：文件内容不会变，按 sha256 命名，单独备份更合适。

## 仓库布局（规划）

```
nice-wiki/
├── AGENTS.md                  # 给 AI 编程助手的总说明（见 engineering.md）
├── CLAUDE.md                  # 只有一行：@AGENTS.md
├── STATUS.md                  # 当前状态快照
├── CHANGELOG.md               # 版本更新日志
├── Makefile                   # 统一命令入口：dev / test / check / eval ……
├── api/
│   └── openapi.yaml           # 前后端接口定义，唯一依据（D39）
├── server/                    # Go 后端（一个 Go module，路径 nice-wiki）
│   ├── AGENTS.md / CLAUDE.md
│   ├── cmd/nice-wiki/         # 程序入口
│   └── internal/
│       ├── store/             # 文件存储 + SQLite 索引
│       ├── doc/               # Tiptap JSON：拆块，渲染 HTML、纯文本、Markdown
│       ├── site/              # 公开页面（Go 模板）
│       ├── api/               # JSON 接口（由 openapi.yaml 生成的接口 + 实现）
│       ├── auth/              # 账号、会话、权限、编辑锁
│       ├── ai/                # 问答、检索、缓存、防滥用、OpenRouter 客户端
│       ├── i18n/              # 翻译流水线、语言判断
│       ├── history/           # 基于 go-git 的版本记录
│       ├── jobs/              # 后台任务队列
│       ├── stats/             # 访问统计
│       └── webui/dist/        # 前端构建产物（嵌入程序，不纳入 git）
├── web/                       # Vue 3 前端（一个 Vite 项目）
│   ├── AGENTS.md / CLAUDE.md
│   └── src/
│       ├── admin/             # 后台单页应用
│       ├── site/              # 公开页面上的 Vue 组件
│       └── shared/            # 两边共用：块样式、Tiptap 扩展定义、生成的 API 客户端
├── fixtures/                  # 前后端共用的样例文档和期望输出（D38）
├── evals/                     # AI 问答评测题集与结果
└── docs/
    ├── plan.md                # 实施计划：阶段、任务、验收标准
    ├── adr/                   # 决策记录，一条一个文件
    ├── design/                # 设计文档
    ├── playbooks/             # 操作手册：新增块类型、新增接口、push……
    └── push-log/              # 过程记录，以 push 为单位
```

各个 Go 包是按职责做的初步划分，写代码时再按实际情况调整。
