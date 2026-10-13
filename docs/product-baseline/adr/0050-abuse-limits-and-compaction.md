> 产品需求基线快照：来源 old:docs/adr/0050-abuse-limits-and-compaction.md，blob 5f0cd0738bed70629a0c43438b6329b161bb9f3d。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D50. 防滥用的细化：明确上限、按花费限流、验证备用方案、对话自动压缩

- 状态：✅ 已定
- 日期：2026-10-12
- 补充：D22

详见 [design/ai-guard.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/ai-guard.md) 的"上限与限流"、[design/ai-qa.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/ai-qa.md) 的"对话的自动压缩"。依据见 [reference/README.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/reference/README.md) 的防滥用部分。

- 明确的上限（参考 OWASP LLM10）：问题 500 字；每次调用的输出上限；最多 4 个子问题；检索 3 轮、工具调用 5 次；每个会话同时只处理一个问题。默认值可调。
- 按花费限流：每个会话、每个 IP 每小时的花费上限，两层，可以整体关闭。
- Turnstile 加载失败时，自动改用自托管的工作量证明。
- 不再叠加更多的模型护栏。
- 长对话：**完全照抄 opencode 的自动压缩机制和默认值**（用户选择）：接近上下文上限（预留 10%）时，保留最近约 15000 token，更早的压成摘要并增量更新；输入太长被拒绝时压缩后重试一次。上限取前置模型和聪明模型中较小的一个。
- 不采纳：在部署文档里推荐 CrowdSec 和 Litestream。
