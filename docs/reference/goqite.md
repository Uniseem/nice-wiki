# goqite

> 基于 SQLite 的 Go 消息队列，仿照 AWS SQS 的设计，但简单得多。

| 项目 | 信息 |
|---|---|
| 方向 | 存储与运维 |
| 仓库 | maragudk/goqite |
| 许可证 | MIT |
| 主要语言 | Go |
| Star | 0.5k |
| 最近更新 | 2026-08 |

## 它是什么

消息存在单张表里；取出的消息在超时之前不会被重复投递；附带一个简单的任务执行层。

## 和我们相关的部分

- 除测试外没有依赖，自己选 SQL 驱动（`database/sql`），所以可以配合 modernc.org/sqlite。
- 也支持 PostgreSQL。

## 借鉴什么

- 和 backlite 二选一，或者参考它"可见性超时"的机制自己写。

## 注意

- 它更底层，重试、定时这些要自己在上面实现。

## 资料

- [仓库](https://github.com/maragudk/goqite)
