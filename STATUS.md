# 状态（2026-10-27）

## 现在能用什么

多用户文档平台（GitBook 式）已在 NL 测试实例和容器部署（不含 Caddy）上跑通；设计见 [docs/architecture-v2.md](docs/architecture-v2.md)，与设计不同的地方在其第 18 节。

- **用户与站点**：邮箱注册（需注册邀请码，不验证邮箱）；GitHub、Google、Linux DO 登录只写了代码，没有联调；每人默认 5 个站点、1GB；成员角色（所有者、编辑、只读）；平台子域名与自定义域名（DNS 验证、Caddy 按需证书）；私有站点（站主之外只有成员能看）；举报与下架；站主后台管理用户、站点、两种邀请码、举报、平台域名与登录方式。
- **站点内容**：沿用原有产品——块编辑器、发布、公开站、多语言自动翻译、AI 问答、知识索引、长期记忆、访问统计、防滥用。统计、媒体、问答、翻译、索引都按站点隔离（一个 PostgreSQL，行级安全）。
- **AI 分层**：自带渠道（OpenAI 兼容地址 + 自己的 key）优先，没有或不可用时用平台额度；AI 邀请码给额度；访客问答每日上限；按用户记账。验证用的是假供应方与 OpenRouter 免费模型。
- **数据**：导入（Markdown 目录、GitBook、Notion、本站导出包，导入的都是草稿）、整站导出、S3、WebDAV 与海外网盘备份目标（手动、每天、每周）、Git 同步（只推已发布内容，可发布后自动推送；拉取生成草稿）。全平台备份用 `deploy/backup.sh`。
- **工程**：account、content、ai、ops 四个宿主进程 + portal、knowledge，Rust 的 search（全文 + 向量）和 edge（页面缓存、限流）；`scripts/gen_deploy.py` 生成各进程镜像与单机编排；浏览器里验证过登录、注册、建站、成员、域名、私有站点、举报、数据与同步等页面。

本地开发与检查（编译、测试、冒烟都在 NL 编译机，见 AGENTS.md；配置、数据和密钥在不进 git 的目录）：

```bash
make build      # 全部 Go 模块 build/vet/gofmt
make check      # 合同、链接、权限、隐私、译文
make gen && make docs
pnpm typecheck
python3 scripts/dev.py up    # 需要 NW_DEV_DATABASE_URL（带 pgvector 的 PostgreSQL，应用角色不能绕过行级安全）
python3 scripts/smoke.py new
```

## 下一步

1. 发布流程（`scripts/release.sh`、GitHub Actions）已改成按进程出镜像、Rust 在原生 amd64 与 arm64 机器上编 musl 静态程序，但只在本地验证了脚本语法和 x86_64 的 musl 编译，没有真正发过版，第一次推 `v*` 标签时要盯一下。
2. edge 已接“发布后立即清缓存”（按站点）和媒体磁盘缓存，人机验证、访问记录攒批仍在 portal。
3. 海外网盘备份（Google Drive、OneDrive、Dropbox）已按官方接口写好并用假网盘服务验证过授权、令牌换发、上传与保留份数，没有真实账号联调；第三方登录和邮件相关（验证、通知）同样只写了代码。用户站点的备份每次是完整导出包，内容没变就不再上传新的一份（没做块级增量）；全平台备份用 `deploy/backup.sh`（增量复制、可选加密、rclone 异地同步）。
4. jobs 的任务表和事件表是各站点共用的（带 `site_id` 列，不是每站点一套）；列表、读取、取消、重放都按当前站点限定，定时调度只有平台管理员能管。
5. 旧 16 服务线路的问答质量、批量翻译计划等遗留事项见 `archive-16-services` 分支。
