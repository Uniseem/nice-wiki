# 待整理会话记录

## 2026-10-15 实现尺度约束（未提交、未推送）

- 新增 ADR0006：边界一次校验、内部信任输入、合同通用条文按调用链整体保证解释、不写防御性冗余与重复工具、默认不写测试与临时验证程序；真实安全与一致性边界照旧失败关闭。
- 同步：根 AGENTS.md 增“实现尺度”节并收紧测试规则，`docs/playbooks/implement-module.md` 增实施尺度，`scripts/render_docs.py` 在各域 functions.md 页首写明通用条文解释并经 make docs 重新生成，ADR 索引、framework.json、plan、STATUS 同步。
- 未改合同条文与业务代码；统一请求校验中间件列为首个 HTTP 绑定工作包前置项。
- 待推送的还有本地提交 `2300d7a`（推送0003远端核对结果）。
