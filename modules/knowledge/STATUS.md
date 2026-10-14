# 索引检索当前进度

更新：2026-10-15。阶段：实施；本域尚未行为验收、尚未部署。

- 本次认领分支：`codex/knowledge-normalize`，基线 `8ae6fcfc6302e2fd41a998faef18be52e2a9714c`，独占范围仅为 `modules/knowledge/`。
- 已实现 2 个文件、1 个函数：`internal/chunk/normalize.go` 的 `knowledge.NormalizeSource`，以及本域 `go.mod`；实现提交为 `77fe57291ab9bcc0f53df2d35018e055e61ca12f`。
- `NormalizeSource` 完成输入大小/UTF-8校验、CRLF/CR→LF、Unicode NFKC 和规范化 UTF-8 到原文偏移映射；无数据库、模型、媒体读取或外部副作用。
- 当前本机无 Go/gofmt；未编译、未运行行为测试、未调用真实接口或模型。实现状态与验证状态分开记录在 [progress.json](progress.json)。
- `SplitChunks` 暂不实现：现有输入只有整段规范文本，输出却要求每块的 `BlockRef`，缺少块边界、blockId 与 blockHash 来源；继续实现会猜测契约。

并行工作只改本域，公共资料交集成会话；根结构登记、公共契约修订和 VPS 检查均未在本次修改。
