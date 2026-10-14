# 索引检索会话交接

## 本次事实

2026-10-15 认领本域文本规范化切片。按本域合同实现了无副作用的 `knowledge.NormalizeSource`：校验封闭输入，统一 CRLF/CR 为 LF，使用已登记的 `golang.org/x/text v0.20.0` 执行 NFKC，并为每个规范化 UTF-8 字节保留原文起点偏移。未读取其他服务私有数据，未调用真实接口、模型、媒体或数据库，未新增测试/样例。

## 基线与交付

- 独占范围：`modules/knowledge/`
- 分支：`codex/knowledge-normalize`
- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`
- 实现提交：`77fe57291ab9bcc0f53df2d35018e055e61ca12f`
- 本次修改：`go.mod`、`internal/chunk/normalize.go`
- 状态记录提交：`808d734bb802514ba606da2f31937baacbd6d48c`；未提交文件应为空。

`NormalizeSource` 已标记 `implemented`，尚未标记 `verified` 或 `accepted`。本机无 Go/gofmt，不能完成编译/格式化/行为验证；`go.sum` 尚未生成。公共 `docs/structure/framework.json` 未在本域修改范围内登记新文件。

## 检查与结果

- `git diff --check`：通过。
- Python JSON 解析 `modules/knowledge/progress.json` 及既有本域实现合同：通过。
- Go 编译、gofmt、VPS `make check`：未运行，本机无 Go/gofmt 且本次不读取私有连接配置。
- 普通测试、E2E、真实接口、真实模型：未运行/未调用，未新增测试授权。

## 阻塞与决策

`knowledge.SplitChunks` 的 `SplitChunksInput` 只有 `buildId/document/normalized/chunkTokens/overlapTokens`，但 `Chunk` 必须带 `BlockRef`（含 `blockId/blockHash`），且步骤要求记录全部 blockIds 与原 UTF-8 偏移。当前契约没有提供块列表、块边界或这些标识的可信来源；本次不生成伪造 block，也不修改公共合同，留待集成会话先修正设计。

## 下一项（一项）

公共契约补齐 `SplitChunks` 的源块边界与稳定 `BlockRef` 输入后，再在 `modules/knowledge/internal/chunk/split.go` 实现 `knowledge.SplitChunks`；开始前复读该函数的 implementation/types/data 条款，完成确定性分块、偏移、hash 和预算校验，并保持不调用模型。
