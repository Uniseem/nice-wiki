# 索引检索会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `NormalizeSource` 按合同步骤5 施加截止 = min(ctx, op.deadline, 5000ms)。
- 补齐 go.sum。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增文件/函数暂记 `in_progress`，待修复提交后回填 SHA。

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
- `python scripts/check_docs.py`：失败；根 `framework.json` 未登记本轮及其他并行会话新增文件，并报告已有 `site-config` 台账缺少提交 SHA。本次未修改根文件。
- Go 编译、gofmt、VPS `make check`：未运行，本机无 Go/gofmt 且本次不读取私有连接配置。
- 普通测试、E2E、真实接口、真实模型：未运行/未调用，未新增测试授权。

## 阻塞与决策

`knowledge.SplitChunks` 的 `SplitChunksInput` 只有 `buildId/document/normalized/chunkTokens/overlapTokens`，但 `Chunk` 必须带 `BlockRef`（含 `blockId/blockHash`），且步骤要求记录全部 blockIds 与原 UTF-8 偏移。当前契约没有提供块列表、块边界或这些标识的可信来源；本次不生成伪造 block，也不修改公共合同，留待集成会话先修正设计。

## 下一项（一项）

公共契约补齐 `SplitChunks` 的源块边界与稳定 `BlockRef` 输入后，再在 `modules/knowledge/internal/chunk/split.go` 实现 `knowledge.SplitChunks`；开始前复读该函数的 implementation/types/data 条款，完成确定性分块、偏移、hash 和预算校验，并保持不调用模型。
