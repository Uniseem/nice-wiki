# translation-agent（吴恩达）

> 用"翻译 → 反思 → 改进"三步工作流做长文翻译的原型项目。

| 项目 | 信息 |
|---|---|
| 方向 | 翻译 |
| 仓库 | andrewyng/translation-agent |
| 许可证 | MIT |
| 主要语言 | Python |
| Star | 5.8k |
| 最近更新 | 2024-08 |

## 它是什么

吴恩达开源的智能体式翻译示例。长文先切块，每块经历三次模型调用：初译、由模型给出修改意见、按意见改进。

## 和我们相关的部分

- 长文切块：每块上限 `MAX_TOKENS_PER_CHUNK = 1000` token；切分时用 `calculate_chunk_size` 让各块大小尽量平均，避免最后剩下一小截。
- 翻译某一块时，把**整篇原文**都发给模型，要翻译的那一段用 `<TRANSLATE_THIS>` 标签圈出，并明确说明"其余部分只作上下文，不要翻译"。
- 反思步骤让模型对照原文给出具体修改建议（准确、流畅、风格、术语），再按建议改写。

## 借鉴什么

- "整篇作上下文、只翻译圈出部分"的提示词写法，直接用于我们的块翻译。
- 小批次（约 1000 token）而不是一次输出整篇。
- 反思改进可以作为**可选**的二次润色，用于重要语言。

## 注意

- 作者自称是周末原型，"不是成熟软件"，最近一次更新在 2024 年。只借鉴思路。
- 三步工作流让成本变成约 3 倍，不能作为默认。

## 资料

- [仓库](https://github.com/andrewyng/translation-agent)
- [核心代码 utils.py](https://github.com/andrewyng/translation-agent/blob/main/src/translation_agent/utils.py)
