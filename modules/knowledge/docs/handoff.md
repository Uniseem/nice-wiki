# 2026-10-18 集成检查通过

pipeline60（生成261、三个Go分片262/263/264、前端265）完整通过，精确生成物已回传。逐项台账只把完整待编译清单转verified；partial、完整启动及既有设计阻塞保持blocked。所有Go模块readonly build/vet/gofmt与go test、完整make check通过；public-site前端类型检查及构建通过。go test没有测试文件，不构成业务验收。accepted=0，未部署。

本批源码汇总d344a37，记录提交等待零生成漂移复核后普通双推。持续向75%推进，用户最新要求及时分批检查/推送，历史停止指令不适用于本次授权。

以下为模块原始交付记录，其中“未CI”只描述交付时。

# 索引检索批次交接（2026-10-18）

本批源码提交 `8632a73c68ac7be79e1cfecfaf8c7a936fe38ce2`，基线 `acc9bbcc1fe0cfbeae91d3c2835b09cea5cbe9bd`，分支 `codex/knowledge-20261018-bulk`，只拥有 `modules/knowledge/`。根授权的公共适配 `8c593f3`、原生重排 query `b05944f` 已作为基线分别 cherry-pick 为 `6701308`、`c042a00`，不是本域交付，不要再次作为本域代码合入。

## 代码就绪，等待编译

主业务 22 项（均加 `knowledge.` 前缀）：

- `knowledge.SplitChunks`
- `knowledge.WriteStagingChunks`
- `knowledge.BuildLexicalIndex`
- `knowledge.RequestEmbeddings`
- `knowledge.StoreEmbeddings`
- `knowledge.ValidateBuild`
- `knowledge.ActivateBuild`
- `knowledge.NormalizeQuery`
- `knowledge.EmbedQuery`
- `knowledge.SearchLexical`
- `knowledge.SearchVector`
- `knowledge.FuseCandidates`
- `knowledge.ValidateCandidates`
- `knowledge.RerankCandidates`
- `knowledge.SelectContext`
- `knowledge.Retrieve`
- `knowledge.ReadSections`
- `knowledge.CreateQueryEmbedding`
- `knowledge.InvokeModel`
- `knowledge.GetBuild`
- `knowledge.ListBuilds`
- `knowledge.RedactDiagnostics`

内部助手 32 项全部在实际代码中被调用并已同步 implementation/progress：NewOperationHandlers、jsonOperation、operationContext、writeResult、NewModelRuntime、WithModelRuntime、CompletedOutput、WithAdminSigningKey、authorizeAdmin、beginAdminRead、validVector、unitVector、acquireQueryFlight、NewSearchRuntime、WithSearchRuntime、searchRuntime、activeSnapshot、publicationKey、sortedSnapshotRows、ReadActiveSnapshot、requestRerank、DecodeVector、TokenizeText、NewIndexRuntime、WithIndexRuntime、RuntimeFromContext、ReadBuildRecord、BeginBuildWrite、storageError、ReadChunks、ReadGeneration、validatePublication。共 54 项统一 `blocked`，原因仅为待集中 GitLab 生成/格式/编译检查；不是设计 blocked，不计 implemented/verified。NormalizeSource 旧实现未改动。

## 实际行为与存储

- 分块不跨稳定 block；按选定模型 tokenizer 提供的 tokenEnds 切分与重叠，逐 UTF8 字节保留源映射；输出 ID 绑定来源、上下文与区间。没有伪造模型 tokenizer。
- 本域独立 SQL 保存完整分块/词项/向量，固定词项算法和规范清单 hash。staging 写入使用平台写窗口并绑定持久租约；active 不被改写。ValidateBuild 前后固定清单并向 content 校验，ActivateBuild 仅接受一秒内证明及活动代数 CAS，同事务写 index.ready outbox；旧代数保留。
- 检索固定一次 SQL 只读快照及完整空间；BM25/cosine→RRF→权威过滤→可选重排→整块预算→最终权威过滤。任意权威失败不当作空成功。ReadSections 只给当前 active 精确引用和完整 sourceOffsets。
- 查询向量按站点/代次/可信根/queryHash/spaceId 持久唯一，本进程按该身份 singleflight。原 operationId 与 MG 幂等键保留，失败/断线不重算。另有调用者幂等键→完整身份摘要映射，同键异输入 CONFLICT；保存前重取平台写窗口。
- 模型仅使用平台 NewGatewayClient/InvokeModel；三个 MG 响应和 content 批量校验响应均用生成 PayloadValidationSpec 的严格生产方 schema。embedding 结果只读完成事件结构化 output；native rerank 显式 query；未配原生时 fast/chat 最多 800 输出 token 严格读既有 ID 排列，格式错误只追加一次独立 attempt。费用未知不重放。
- 六个实际 HTTP 映射已提供，须由完整 app 交给平台 NewServiceHandler；缺其他 handler 时 app 应启动失败。没有给未完成功能加占位成功或把完整服务记 ready。

## 合同与统一生成要求

本域 types 追加 Chunk.sourceOffsets、SourceBlock 与准确 tokenEnds，向量 number 统一 double；OpenAPI 对 sections 明确证据映射，并通过 x-runtime-payload-schemas 保留 CreateInvocationResult、CancelInvocationResult、StreamEvent、ContentValidatePublicationsResult 的生产方引用闭包。源码使用未来生成的 `gen.PayloadValidationSpec`、`gen.Chunk.SourceOffsets` 和 double 向量，根候选 CI 必须先 make gen/docs，再分片编译；生成目录没有手改。

实际 schemaVersion 1 迁移文件已编写，完整启动尚未应用，索引不另写第二份文件。data/workflows 已同步 SQL 清单表示和所有实际规则。实现合同及台账同步到实际 95 个函数，其中新增 32 个必要内部助手；不是增加空函数凑比例。

## 验证事实

仅本地 JSON 解析/状态同步、Git 隐私身份核对与 `git diff --check` 通过。未运行 Go/gofmt/build/vet/gen/docs/check、测试、模型、真实接口、业务服务或部署。作者与提交者保留用户 GitHub 代用邮箱，合著 GPT-6 使用指定 noreply。用户最新授权及时 GitLab 检查和推送，由根会话执行，本域不 push。

## 尚未完成与下一项

完整 ExecuteBuild、SourceBlock 的模型 tokenizer 来源、资源提取、配置/构建创建/事件执行、恢复屏障与完整 BuildApplication 尚未完成。这些原有 planned 项没有计入本批 ready。下一项先由根在 GitLab 对本批统一生成、编译和完整 make check；编译/结构问题先修，再把对应 54 项按真实证据更新。行为验收仍未授权且保持未开始。

---

以下旧交接是历史记录；原 SplitChunks 设计缺口已按本次根裁定修正，不能继续当作新阻塞。

# 索引检索会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `NormalizeSource` 按合同步骤5 施加截止 = min(ctx, op.deadline, 5000ms)。
- 补齐 go.sum。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。

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
