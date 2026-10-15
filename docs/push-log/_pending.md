# 待整理会话记录

## 2026-10-15 实现尺度约束（未提交、未推送）

- 新增 ADR0006：边界一次校验、内部信任输入、合同通用条文按调用链整体保证解释、不写防御性冗余与重复工具、默认不写测试与临时验证程序；真实安全与一致性边界照旧失败关闭。
- 同步：根 AGENTS.md 增“实现尺度”节并收紧测试规则，`docs/playbooks/implement-module.md` 增实施尺度，`scripts/render_docs.py` 在各域 functions.md 页首写明通用条文解释并经 make docs 重新生成，ADR 索引、framework.json、plan、STATUS 同步。
- 未改合同条文与业务代码；统一请求校验中间件列为首个 HTTP 绑定工作包前置项。
- 待推送的还有本地提交 `2300d7a`（推送0003远端核对结果）。

## 2026-10-15 编译门槛与公共机制（未提交、未推送）

- 规则：根 AGENTS 与 15 域 AGENTS 去掉过时的“设计阶段/不读vps.txt”表述，写入编译门槛（make build；优先VPS，不可用时本机；go.sum 只由工具生成）；实施手册与并行开发合并流程写明编译与逐个逻辑审查。
- 工具：Makefile 新增 `make build`（Go 模块 build/vet/gofmt），`make test` 改为说明默认无测试；check_docs 新增 go.sum 存在与格式检查。
- 平台：新增 `lifecycle.NewSiteConfigClient` 并替换 10 域自写运行状态适配；新增 `apivalidate.NewRequestValidator`（kin-openapi），平台库新增 kin-openapi 依赖；合同、types、台账与结构登记同步。
