# Contextual Retrieval（Anthropic 官方示例）

> 我们"入库时给每个片段补一句背景"这一做法的出处和官方实现示例。

| 项目 | 信息 |
|---|---|
| 方向 | 检索问答 |
| 仓库 | anthropics/claude-cookbooks |
| 许可证 | MIT |
| 主要语言 | Jupyter Notebook |
| Star | 53.4k |
| 最近更新 | 2026-10 |

## 它是什么

Anthropic 提出的检索改进方法：建索引前，让模型根据整篇文档为每个片段写一句背景说明，拼在片段前面，再做向量和关键词索引。官方 cookbook 里有完整示例。

## 和我们相关的部分

- 示例目录：`capabilities/contextual-embeddings/`，包括说明文档和一个部署示例。
- 官方博客给出的效果：只用补背景的向量检索，检索失败率下降 35%；再加上补背景的关键词检索下降 49%；再加重排下降 67%。
- 整篇文档作为每次调用的共同前缀，配合提示缓存，成本很低。

## 借鉴什么

- 已经写进 D10 的检索方案，实现时直接参照示例的提示词和流程。

## 资料

- [示例目录](https://github.com/anthropics/claude-cookbooks/tree/main/capabilities/contextual-embeddings)
- [官方博客](https://www.anthropic.com/news/contextual-retrieval)
