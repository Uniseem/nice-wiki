# New API

> 在 One API 基础上发展的模型网关，支持多种接口格式互相转换。

| 项目 | 信息 |
|---|---|
| 方向 | 模型接入 |
| 仓库 | QuantumNous/new-api |
| 许可证 | AGPL-3.0 |
| 主要语言 | Go |
| Star | 49.8k |
| 最近更新 | 2026-10 |

## 它是什么

统一的模型聚合和分发网关。

## 和我们相关的部分

- 上游支持 OpenAI、Anthropic、Gemini、Azure、Bedrock、Vertex、DeepSeek、通义等。
- 对外同时提供 OpenAI Chat Completions、Responses、Anthropic Messages、Gemini 几种接口格式，可以互相转换。

## 借鉴什么

- **我们 Go 端协议适配器的参考实现**：OpenAI、Anthropic、Gemini 三种协议的请求、流式、工具调用在 Go 里怎么写。
- 同为 AGPL-3.0，许可证上可以借用代码（保留版权声明）。

## 注意

- 它是完整的网关产品，只借用协议转换相关的部分。

## 资料

- [仓库](https://github.com/QuantumNous/new-api)
