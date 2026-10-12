# RedisVL 的 SemanticCache

> Redis 官方的向量库，其中的语义缓存参数设计可以参考。

| 项目 | 信息 |
|---|---|
| 方向 | 缓存 |
| 仓库 | redis/redis-vl-python |
| 许可证 | MIT |
| 主要语言 | Python |
| Star | 0.4k |
| 最近更新 | 2026-10 |

## 它是什么

Redis 的 Python 向量工具库，内置语义缓存 `SemanticCache`。

## 和我们相关的部分

- `distance_threshold` 默认 0.1（余弦距离，取值 0–2，越小越严格），默认就偏严格，以减少误命中。
- `ttl`：缓存有效期。
- `filterable_fields` 和 `filter_expression`：给缓存加过滤字段，查找时只在同一分区里找，例如同一个模型、同一个提示词版本、同一个租户。

## 借鉴什么

- 默认阈值要严格，宁可少命中也不要答错。
- 缓存分区：按语言、嵌入模型、问答记忆的版本分区，避免不同上下文之间互相误命中（我们已有"记忆版本"，补上语言和嵌入模型）。

## 注意

- 不同版本文档的参数说明略有出入，以实际使用的版本为准。

## 资料

- [仓库](https://github.com/redis/redis-vl-python)
- [缓存接口文档](https://redis.io/docs/latest/develop/ai/redisvl/api/cache/)
