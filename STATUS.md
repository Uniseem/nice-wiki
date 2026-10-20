# 状态（2026-10-21）

## 现在能用什么

15 个服务约有 11 万行手写 Go，本地 `make build` 和 `make check` 都能通过。但还没有任何服务真正启动过，主链路（登录 → 写文档 → 发布 → 公开页可见）还不通。

各服务 `BuildApplication`（`modules/<域>/internal/app/wire.go`）能否返回可运行的 Application：

| 情况 | 服务 |
|---|---|
| 能返回（业务处理器多数不全，未实际启动过） | identity、site-config、jobs、portal、knowledge、analytics、data-protection、media、memory |
| 末尾直接返回"尚未装配"，起不来 | **content**、**public-site**、localization、model-gateway、traffic-guard |
| 还没有 cmd 入口和 internal/app | qa |

## 下一步（按顺序）

1. 让 content、public-site 的 `BuildApplication` 返回可运行的 Application：已实现的接口挂上，未实现的返回 DEPENDENCY_UNAVAILABLE，不阻止启动。
2. 在本地启动 identity、site-config、jobs、content、portal、public-site，打通：初始化站点 → 登录 → 新建并保存文档 → 发布 → 公开页能看到。
3. 主链路通了以后，再按 [docs/plan.md](docs/plan.md) 逐个补其余服务，最后补恢复、封口、快照这类横切机制。

## 未合并的旧工作

本地有约 68 个 `codex/*` 分支，带着 main 里没有的提交，另有约 90 个 worktree（大多在 /private/tmp 下）。例如 `codex/finish-20261018-knowledge` 有 60 个提交，`codex/resume-20261020-jobs` 有 27 个。这些都没合并也没删除；做到相关模块时按需挑选，不要整体回放。

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
