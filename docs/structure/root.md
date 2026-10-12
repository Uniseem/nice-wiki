# 文件结构：根目录与其他顶层目录

> 本文件是契约：**只能创建、修改这里列出的文件**。增加、删除、改名、移动任何文件或目录，都必须先询问用户（D45）。标记说明见 [README.md](README.md)。

## 根目录

| 文件 | 用途 |
|---|---|
| `AGENTS.md` | 给所有 AI 助手的总说明：项目简介和文档索引、目录地图、常用命令、会话流程、完成的定义、硬性规定（测试、提交署名、隐私、push、文件结构）、不变量 |
| `CLAUDE.md` | 只有一行 `@AGENTS.md` |
| `STATUS.md` | 当前状态快照：当前阶段、进行中、下一步、阻塞与待决、已知问题与技术债。随时覆盖 |
| `README.md` | 给人看的项目介绍：功能、文档索引、本地开发、许可证 |
| `CHANGELOG.md` | 每个版本对使用者来说有什么变化，发布版本时追加 |
| `LICENSE` | AGPL-3.0 全文 |
| `Makefile` | 统一的命令入口，见下表 |
| `.gitignore` | 忽略依赖目录、构建产物、本地的 `data/`、`.env` |
| `.gitattributes` | 统一换行符；生成的代码文件标记为 `linguist-generated`，在 GitHub 上默认折叠它们的 diff |
| `.editorconfig` | 编辑器的缩进和换行设置 |

### Makefile 的命令

| 命令 | 作用 |
|---|---|
| `make dev` | 同时启动 Go 和 Vite 开发服务器 |
| `make dev-server` / `make dev-web` | 只启动其中一个 |
| `make gen` | 运行所有代码生成：sqlc、oapi-codegen、openapi-typescript |
| `make build` | 构建前端，再编译出单个可执行文件 |
| `make check` | 全部检查：格式、lint、类型检查、已有的测试、生成的代码是否最新、隐私扫描。AI 助手可以自行运行 |
| `make test` | 只运行已有的测试 |
| `make test-update` | 重新生成快照测试的期望输出（**[测试]**，需要用户同意） |
| `make e2e` | 端到端测试（**[测试]**，需要用户同意） |
| `make eval` | AI 质量评测，使用真实模型，会花钱（**[测试]**，需要用户同意） |
| `make catalog` | 更新内置的模型目录快照 |
| `make hooks` | 把 git 钩子目录指向 `.githooks/` |
| `make privacy` | 单独运行隐私扫描 |

## .githooks/ —— git 钩子

| 文件 | 用途 |
|---|---|
| `pre-commit` | 提交前：对暂存的改动运行隐私扫描和格式检查 |
| `commit-msg` | 检查提交信息：没有隐私信息；格式符合 Conventional Commits |
| `pre-push` | push 前：对所有待推送的提交运行隐私扫描 |

## api/ —— 前后端接口定义

| 文件 | 用途 |
|---|---|
| `openapi.yaml` | 所有接口的唯一定义（D39）。按标签分组，每个标签对应 `server/internal/api/` 里的一个文件。权限名称也在这里以枚举的形式定义，前后端共用 |

## deploy/ —— 部署

| 文件 | 用途 |
|---|---|
| `README.md` | 部署指南：醒目提示"启动后立即注册超管"；反向代理的配置要点（关闭响应缓冲、传递真实 IP、配置可信代理）；备份与升级 |
| `Dockerfile` | 多阶段构建：构建前端 → 编译 Go → 最小的运行镜像，`/data` 作为数据卷 |
| `docker-compose.yml` | 示例：程序 + 数据卷 |
| `nginx.conf.example` | Nginx 反向代理示例，包括 SSE 需要的设置 |
| `Caddyfile.example` | Caddy 反向代理示例 |

## fixtures/ —— 样例文档 **[测试]**

前后端测试共用（D38）。**这个目录里的文件编写前必须征得用户同意。**

| 文件 | 用途 |
|---|---|
| `README.md` | 样例的命名规则；如何重新生成期望输出 |
| `docs/<名称>.json` | 样例文档，每种块至少一个 |
| `expected/<名称>.html` / `.txt` / `.md` | Go 三个渲染器的期望输出 |
| `expected/<名称>.tiptap.html` | Tiptap `generateHTML` 的输出，用于前后端一致性比对 |

## evals/ —— AI 质量评测 **[测试]**

**这个目录里的文件编写和运行前必须征得用户同意。**

| 文件 | 用途 |
|---|---|
| `README.md` | 评测方法、指标、如何运行 |
| `qa.yaml` | 评测题：问题、应该命中的文档或块、回答要点 |
| `results/<日期>.md` | 每次评测的结果，和上一次对比 |

## docs/ —— 文档

| 路径 | 用途 |
|---|---|
| `plan.md` | 实施计划：阶段、任务编号、依赖、验收标准、建议的测试 |
| `adr/README.md` | 决策记录的索引、规则和模板 |
| `adr/NNNN-<短名>.md` | 每条决策一个文件 |
| `design/*.md` | 各模块的设计文档 |
| `structure/README.md` | 文件结构的总则、标记说明、数据目录 |
| `structure/root.md` | 本文件 |
| `structure/server.md` | Go 后端的文件结构 |
| `structure/web.md` | 前端的文件结构 |
| `playbooks/add-block.md` | 操作手册：新增块类型 |
| `playbooks/add-api.md` | 操作手册：新增接口 |
| `playbooks/add-migration.md` | 操作手册：新增数据库迁移，或者文档格式迁移 |
| `playbooks/add-language.md` | 操作手册：在内置语言清单里新增语言 |
| `playbooks/add-video-provider.md` | 操作手册：新增视频平台 |
| `playbooks/add-llm-protocol.md` | 操作手册：新增模型协议适配器 |
| `playbooks/add-prompt.md` | 操作手册：新增或修改提示词 |
| `playbooks/push.md` | 操作手册：用户要求 push 时的完整步骤 |
| `playbooks/change-structure.md` | 操作手册：需要改文件结构时，怎么向用户提出申请、获批后要同步修改哪些文档 |
| `push-log/INDEX.md` | push 记录索引 |
| `push-log/_pending.md` | 上次 push 之后的会话记录 |
| `push-log/YYYY-MM-DD-NNNN.md` | 每次 push 一份记录 |
