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
- 问答（M3 第一版）：访客在文档页右下角提问 → 准入（traffic-guard）→ 检索知识库 → 模型流式回答 → 引用来源。需要先在后台配好模型接入（供应方/模型/档位/任务）、知识库索引、防滥用策略和问答策略；用本机假供应方联调通过，没有用真实模型。简化项：没有前置分类/检索核查/语义缓存/压缩，轮次在 qa 进程内执行（不走 jobs），挑战（人机验证）页面还没做，所以防滥用策略里要先把“PoW 挑战”关掉。
- 后台页面：文档、站点设置、账号与权限、任务队列、统计与审计、备份与恢复（界面壳在 portal/web，各域 module.js 由壳挂载）。
- 部署文件（未部署过）：各域 Dockerfile、`deploy/docker-compose.yml`、Caddyfile、`deploy/init-secrets.sh`，由 `scripts/gen_deploy.py` 生成。

## 下一步（按顺序）

1. M2 余项：图片多宽变体、视频元数据与封面、资源库页面、traffic-guard 的访客侧接入、旧代 outbox 之外的事件订阅（knowledge/media/localization 消费 content 事件）。
2. M3：model-gateway / knowledge / memory / qa / localization 的后台页面与接口补全（真实模型调用需要你当次同意并提供供应方密钥）。
3. M4：各域屏障与快照导出、恢复换目录、data-protection 备份/恢复/镜像。
4. M5：在服务器上部署（需要你当次同意）。

## 已知缺口

- portal 本地路由（bootstrap、events、preferences、屏障）未实现，只有 healthz/readyz；data-protection 的 readyz 仍是 503（备份业务未实现）。
- 分片 sitemap 未挂。
- 各服务用 `lifecycle.SimpleAdmission` 代替快照屏障准入，M4 再换。
- media 的 readVideoMetadata（平台视频链接）、localization 的翻译与界面文案、qa 的问答与调试台、knowledge 的 inspectEvidence 还没接；它们都依赖模型或后续里程碑。
- 后台界面：各域都有页面了（知识库索引页只能配置与查看，没有模型档位时无法实际建索引）。

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
