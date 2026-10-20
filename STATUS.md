# 状态（2026-10-21）

## 现在能用什么

15 个服务都能在本地启动，M1 主链路（浏览器里注册超管 → 初始化站点 → 新建文档 → 编辑自动保存 → 发布 → 公开页可见）已跑通，M2 的一部分也已可用。

本地一键启动（不进 git，数据在 `.local/dev`）：

```bash
python3 scripts/dev.py up
```

然后打开 <http://127.0.0.1:8080/admin/login>（首次是注册页）。其他命令：`dev.py down | reset（清数据）| status | logs <服务>`；`python3 scripts/smoke.py new [img] [trash]` 用接口串一遍整条链路。排查问题时服务日志里有 `问题响应`、`原始错误`、`未绑定接口` 等调试行（环境变量 `NW_DEBUG`，dev.py 默认开启）。

已完成：
- 文档：新建、嵌套、移动、编辑锁与自动保存草稿、发布/撤回、回收站（删除/恢复/彻底删除，已发布的留 410 或跳转）、发布历史与回滚、草稿预览。
- 编辑器（Tiptap）：标题、列表、任务、引用、代码（含语言）、链接、表格、提示框、公式、图片（上传后浏览器生成 640/1280/1920 的 WebP 变体，公开页用 srcset）、视频（浏览器截帧作封面）、附件上传。
- 公开页：Go 直接输出完整 HTML（首页主视觉按钮与背景色、公告横幅、三栏、目录、代码高亮、公式（浏览器端 KaTeX）、图片、嵌套路径、旧路径 301、删除 410）、首页、导航、robots、sitemap、llms；页面访问信标到 analytics。
- 审计：各服务 HTTP 边界在后台写操作成功后自动向 analytics 追加审计（libs/platform/audit，只覆盖合同枚举内的动作，metadata 为空）。
- 问答（M3 第一版）：访客在文档页右下角提问 → 准入（traffic-guard）→ 检索知识库 → 模型流式回答 → 引用来源。需要先在后台配好模型接入（供应方/模型/档位/任务）、知识库索引、防滥用策略和问答策略；用本机假供应方联调通过，没有用真实模型。简化项：没有前置分类/检索核查/语义缓存/压缩，轮次在 qa 进程内执行（不走 jobs）；人机验证用浏览器里的 PoW（Turnstile 没做）。
- 文档翻译（M3）：后台“语言”页给每篇已发布文档一个“翻译”按钮 → 按块调模型翻译（保留行内格式）→ 访客在 /<语言>/… 看到译文。用假供应方联调通过。简化项：没有术语表/翻译记忆/反思润色，翻译在 localization 进程内执行；语言开了“自动翻译”后，localization 每 10 秒扫一遍公开树，给还没有译文的当前发布修订补译（每轮最多 3 篇，失败的不自动重试）；“启用语言时按估价批量翻译”（approve-plan）批准后会把该语言的自动翻译打开，实际翻译由上面的自动补译完成，它建的 jobs 任务没有执行器、会一直排队（无害）；界面文案翻译、语言建议、缓存回答翻译未接。
- 后台页面：文档、站点设置（含首页主视觉、公告横幅）、资源库、账号与权限、语言、模型接入（供应方/模型/档位/任务）、知识库索引、长期记忆、问答管理、防滥用、任务队列、统计与审计、备份与恢复（界面壳在 portal/web，各域 module.js 由壳挂载）。
- 备份（简易版）：后台“备份与恢复”页可创建/列出/下载完整备份。data-protection 同步把配置里 `backupSources` 列出的各服务数据目录打成一个 zip（SQLite 用 VACUUM INTO 取一致快照，不含密钥），下载用 2 分钟一次性令牌；本地 dev 和 compose 配置都已带 `backupSources`。简化项：没有跨域屏障/水位、没有加密包与 includeSecrets（忽略）、没有定时备份、镜像和恢复仍是 DEPENDENCY_UNAVAILABLE；恢复暂用 `deploy/restore.sh` 一类的手工方式（zip 里是各服务数据目录原样结构）。docker 里没跑过。
- 部署文件（未部署过）：各域 Dockerfile、`deploy/docker-compose.yml`、Caddyfile、`deploy/init-secrets.sh`，由 `scripts/gen_deploy.py` 生成。

## 下一步（按顺序）

1. 用真实模型试一遍 AI 功能（需要你当次同意并提供供应方密钥）：后台“模型接入”配供应方/模型/档位/任务 → “知识库索引”配置并给文档建索引 → “防滥用”保存策略 → “问答管理”开启公开问答 → “语言”启用语言并翻译。本地不花钱联调：`python3 scripts/dev_fake_provider.py`（假的 OpenAI 兼容供应方）+ `python3 scripts/dev_ai_setup.py`（站点初始化后，一键配好模型/档位/任务/知识库索引；防滥用策略、问答策略仍在后台页面里设）。
2. M2 余项：media 消费 content 事件；翻译的自动触发现在是轮询，不是事件。
3. M3 补完：前置分类/检索核查/语义缓存、approve-plan 批量翻译执行器、界面文案翻译、语言建议。
4. M4：各域屏障与快照导出、恢复换目录、恢复/镜像（简易备份已有，见上）。
5. M5：在服务器上部署（需要你当次同意）；15 个镜像都在本机 docker 里构建通过，整套 compose 也在本机起过：通过 Caddy 的 HTTPS 注册超管、初始化站点、新建并发布文档、公开页读取都走通（`NW_PORTAL=https://localhost python3 scripts/smoke.py new`）。这一轮顺带修了：媒体镜像入口被 alpine 自带的 /media 目录挡住、model-gateway 与 data-protection 的主密钥文件要求 0600 而容器普通用户读不到（这两个服务改以 root 运行）、init-secrets 写错目录、构建上下文带宿主机 node_modules。没确认的：初始化之前退出的 analytics/model-gateway/knowledge 在 docker 里多久能自动恢复（我检查时还在重启退避中）；改过入口路径的新 Dockerfile 只有 media 用新方式验证过。

## 已知缺口

- portal 本地路由（bootstrap、events、preferences、屏障）未实现，只有 healthz/readyz；data-protection 的 readyz 仍是 503（恢复/镜像业务未实现，备份接口本身可用）。
- 分片 sitemap 未挂。
- 各服务用 `lifecycle.SimpleAdmission` 代替快照屏障准入，M4 再换。
- media 的 readVideoMetadata（平台视频链接）、localization 的界面文案翻译/语言建议/缓存回答翻译/approve-plan 执行器、qa 的调试台/缓存/前置分类、knowledge 的 inspectEvidence 还没接；它们都依赖模型或后续里程碑。
- 站点初始化之前，依赖站点身份的服务（model-gateway、knowledge、analytics 等）会原地每 3 秒重试，初始化一完成就就绪，不再退出重启。

## 未合并的旧工作

本地有约 68 个 `codex/*` 分支，带着 main 里没有的提交，另有约 90 个 worktree（大多在 /private/tmp 下）。做到相关模块时按需挑选，不要整体回放。

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
