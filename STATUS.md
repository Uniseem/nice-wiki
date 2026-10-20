# 状态（2026-10-21）

## 现在能用什么

M1 主链路已在本地跑通：浏览器里注册超管 → 初始化站点 → 新建文档 → 编辑自动保存 → 发布 → 公开页可见（撤回也可用）。

本地一键启动（不进 git，数据在 `.local/dev`）：

```bash
python3 scripts/dev.py up
```

然后打开 <http://127.0.0.1:8080/admin/login>（首次是注册页）。其他命令：`dev.py down | reset（清数据）| status | logs <服务>`；`python3 scripts/smoke.py` 用接口串一遍整条链路。

已能启动的服务：site-config、identity、jobs、traffic-guard、media、localization、content、public-site、portal。已实现的接口挂载，未实现的返回 DEPENDENCY_UNAVAILABLE。
后台界面：portal/web 是壳（登录、初始化、导航），content/web 是文档树 + Tiptap 编辑器，jobs/web 是任务页。

## 下一步（按顺序）

1. M1 收尾：回收站、历史/回滚、预览、站点设置页、账号页；编辑器补表格、图片、提示块、链接。
2. M2：media 上传与图片/视频、analytics、traffic-guard 的访客侧接入。
3. M3：model-gateway、knowledge、memory、qa（无 cmd 入口）、localization 补全。
4. M4：屏障/快照/恢复换目录、data-protection。M5：部署。

## 已知缺口

- 嵌套文档的公开路径只支持一段 slug；`.md`、分片 sitemap、预览路由未挂。
- portal 本地路由（bootstrap、events、preferences、readiness、屏障）未实现，只有 healthz/readyz。
- 目前各服务用 `lifecycle.SimpleAdmission` 代替快照屏障准入，M4 再换。
- model-gateway、knowledge、memory、analytics、data-protection 还没有在本地启动过；qa 没有 cmd 入口。

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
