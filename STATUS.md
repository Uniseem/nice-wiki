# 状态（2026-10-21）

## 现在能用什么

15 个服务都能在本地启动；文档、公开站、资源、多语言、AI 问答、知识索引、翻译、账号、统计审计、备份都已可用，并已用阿里云百炼真实模型走通。

本地一键启动（不进 git，数据在 `.local/dev`）：

```bash
python3 scripts/dev.py up
```

然后打开 <http://127.0.0.1:8080/admin/login>（首次是注册页）。其他命令：`dev.py down | reset（清数据）| status | logs <服务>`；`python3 scripts/smoke.py new [img] [trash]` 用接口串一遍主链路。服务日志里有 `问题响应`、`原始错误`、`供应方返回错误` 等调试行（`NW_DEBUG`，dev.py 默认开启）。

前端（React + HeroUI v3：后台单页应用 + 公开页交互岛）只在 GitLab CI 构建，job“前端构建与类型检查”的产物 `web-dist/` 保留 7 天；本地磁盘紧张时把产物解压到 `.local/dev/web`，再用 `NW_SKIP_WEB_BUILD=1 python3 scripts/dev.py up` 跳过本地构建。

- 文档与编辑器：新建、嵌套、移动、编辑锁与自动保存、发布/撤回、回收站、历史与回滚、草稿预览；Tiptap 编辑器含表格、提示框、多栏、画板（Excalidraw）、公式、图片（WebP 变体）、视频、附件。
- 公开站：Go 模板直出 HTML，问答面板和窄屏文档树按需加载；首页主视觉、公告、页脚、站标/分享图、目录、代码高亮、公式、旧路径 301、删除 410、robots/sitemap/llms、访问信标。
- 模型接入（参照 opencode）：“接入供应方”从内置 models.dev 目录选（国内常用的排前，带中文名），只填密钥；“添加模型”向供应方拉取账号实际可用的模型勾选添加，上下文长度跟随供应方/目录给出的最大值（已有模型不一致时可一键更新）；同一 OpenAI 兼容供应方可同时挂对话和向量模型；模型可停用。然后点“一键生成档位和任务分配”。
- 计费暂停：`libs/platform` 的 `BillingEnabled=false`，模型网关不按价目/上限拦调用，知识库不以费用证明阻止重做；后台不显示价格、费用上限和费用统计（账本照常记录）。
- 不花钱联调：`python3 scripts/dev_fake_provider.py`（假的 OpenAI 兼容供应方）+ `python3 scripts/dev_ai_setup.py`（一键配好模型/档位/任务/知识索引）。
- AI：公开问答（准入 → 前置改写 → 检索 → 流式回答 → 引用，含精确缓存、精选缓存、调试台）、知识索引（切块、背景短句、图片/视频封面视觉描述、向量）、文档与界面文字翻译、长期记忆。
- 问答上下文像 ADE：每轮带上摘要和之后的全部消息，快占满回答模型窗口（扣预留比例和输出额度）才用 qa.compact 把早期对话并入增量摘要；提示词按“系统与记忆 → 摘要 → 历史 → 本轮材料与问题”排列，吃供应方前缀缓存。前置改写每轮运行，检索用改写后的独立问题。
- 长期记忆在编辑抽屉里勾选“用于”（问答回答 / 知识索引），保存时写入任务映射；新建默认用于问答回答。真实模型：qwen-plus / qwen-flash / qwen3-vl-plus / text-embedding-v4。
- 账号：超管建子账号并勾选功能权限，首次登录强制改口令；登录设备显示“浏览器 · 系统”，可一键让其他设备退出。
- 统计与审计：访问统计与导出；后台写操作自动记审计（新到旧、显示账号名）。
- 备份：创建/下载完整备份（可设口令，openssl 可解），每日自动备份可开可关；恢复用 `deploy/restore-archive.sh`，本地与 docker compose 都演练过。
- 部署文件（只在本机 docker 演练过）：各域 Dockerfile、`deploy/docker-compose.yml`、Caddyfile、`deploy/init-secrets.sh`，由 `scripts/gen_deploy.py` 生成。

## 下一步（按顺序）

1. 本轮提交只推了 GitLab，确认后双推到 GitHub。
2. 首次发布时 AI 生成路径名和 SEO 描述（任务 content.slug / content.seo）后端还没接。
3. 资源库：确认清理后在 24 小时保护窗口内看不到待删计划、也不能撤销（缺“列出计划”接口）。
4. M3 补完：检索核查、语义缓存、approve-plan 批量翻译执行器、模型判断的语言建议；界面文字翻译只补缺的条目，不会重翻已有的；文档翻译读取长期记忆（目前不读，记忆页也没列“翻译”）。
5. M4/M5：各域屏障与快照、在线恢复/镜像；服务器部署（需要你当次同意）。

## 已知缺口

- 计费相关的 HTTP 接口（账本、对账、价目、报价、费用额度、费用统计）仍在，只是后台不再调用。
- 网关的 token 估算对中文偏保守（约为真实值两倍），问答压缩与窗口校验同口径，所以中文对话约在真实用量一半时压缩；要更贴近需改用供应方返回的真实用量。
- portal 本地路由（bootstrap、events、preferences、屏障）未实现；data-protection 的在线恢复/镜像未实现。
- 各服务用 `lifecycle.SimpleAdmission` 代替快照屏障准入，M4 再换。
- 站点初始化之前，依赖站点身份的服务原地每 3 秒重试，初始化后自动就绪。
- 本地有约 68 个 `codex/*` 分支和约 90 个 worktree 带着 main 没有的旧提交，做到相关模块时按需挑选，不整体回放。

## 常用命令

```bash
make build
```

```bash
make check
```

改了 OpenAPI 或合同之后：

```bash
make gen && make docs
```
