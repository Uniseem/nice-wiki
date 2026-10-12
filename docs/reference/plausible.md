# Plausible

> 注重隐私、不用 cookie 的网站统计，我们"每日盐"做法的出处。

| 项目 | 信息 |
|---|---|
| 方向 | 统计 |
| 仓库 | plausible/analytics |
| 许可证 | AGPL-3.0 |
| 主要语言 | Elixir |
| Star | 29.4k |
| 最近更新 | 2026-10 |

## 它是什么

Google Analytics 的轻量替代，可以自托管。

## 和我们相关的部分

- 独立访客：用"每日的盐 + 网站域名 + IP + User-Agent"算哈希；盐每 24 小时更换，旧盐删除，所以无法跨天关联同一个人。
- 原始 IP 和 User-Agent 不保存。
- 代价是无法区分新老访客、无法做留存分析。

## 借鉴什么

- D31 已经采用同样的做法。

## 注意

- 以上细节来自其数据政策的二手整理，以官方页面为准。

## 资料

- [仓库](https://github.com/plausible/analytics)
- [数据政策](https://plausible.io/data-policy)
