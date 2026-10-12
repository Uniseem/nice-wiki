# NeMo Guardrails

> 英伟达的大模型护栏工具包，其中"话题护栏"就是判断问题是否偏题。

| 项目 | 信息 |
|---|---|
| 方向 | 防滥用 |
| 仓库 | NVIDIA-NeMo/Guardrails |
| 许可证 | Apache-2.0 |
| 主要语言 | Python |
| Star | 7.3k |
| 最近更新 | 2026-10 |

## 它是什么

给对话系统加可编程护栏的工具包，用 Colang 语言描述对话流程和限制。

## 和我们相关的部分

- 话题护栏（topical rails）：规定机器人可以谈和不能谈的话题，可以接入专门的话题检测模型。
- OpenAI 的示例同样用一个小模型做话题护栏，检测到偏题就不让主模型回答，用来平衡成本、延迟和准确度。
- 一篇对比文章提醒：**护栏叠得越多，误伤越多**——五个准确率 90% 的护栏叠在一起，约 41% 的正常请求会被误拦。

## 借鉴什么

- 印证了我们的做法：用前置模型（小而快）判断是否偏题，偏题就不调用聪明模型。
- **不再叠加更多的模型护栏**，保持"程序检查 + 前置模型"两层，避免误伤正常访客。

## 注意

- 它是 Python 框架，我们不直接使用。

## 资料

- [仓库](https://github.com/NVIDIA-NeMo/Guardrails)
- [文档](https://docs.nvidia.com/nemo/guardrails/)
- [OpenAI 护栏示例](https://developers.openai.com/cookbook/examples/how_to_use_guardrails)
- [偏题检测论文（arXiv 2411.12946）](https://arxiv.org/html/2411.12946v1)
