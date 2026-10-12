# opencode

> 我们模型接入设计的直接参考（D44）。

| 项目 | 信息 |
|---|---|
| 方向 | 模型接入 |
| 仓库 | anomalyco/opencode（原 sst/opencode） |
| 许可证 | MIT |
| 主要语言 | TypeScript |
| Star | 213k |
| 最近更新 | 2026-10 |

## 它是什么

开源的编程智能体，支持 75 个以上的模型供应商。

## 和我们相关的部分

- 每个供应商可配置：使用哪个 AI SDK 包（`npm`）、显示名、接口地址、密钥、请求头、超时（总超时、等响应头超时、流式片段间隔）。
- 自定义供应商：选择兼容 OpenAI 的包，填接口地址和模型列表。
- 用白名单、黑名单控制显示哪些模型。
- `model` 和 `small_model` 区分主模型和轻量任务模型；没设置 `small_model` 时，尝试用同一供应商更便宜的模型，否则退回主模型。
- 密钥存在单独的认证文件里。

## 借鉴什么

- 已经吸收进 D44：渠道配置项、自定义渠道、按档位区分模型。

## 资料

- [仓库](https://github.com/anomalyco/opencode)
- [供应商文档](https://opencode.ai/docs/providers/)
- [配置文档](https://opencode.ai/docs/config/)
