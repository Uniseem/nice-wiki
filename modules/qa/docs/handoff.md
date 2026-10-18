# 2026-10-18 问答第二批交接

- 独占范围仅 `modules/qa/`，分支 `codex/qa-20261018-bulk`。第二批源码 `358fe68`；首批源码 `fea4204`、台账 `489bf1a`。共享公共 cherry-pick `28614ed` / `e7d621d` 已由根持有，不应重复合入。
- 新增原有业务 ready IDs（全部待GitLab，不是implemented）：`qa.ResolveTurnContext`、`qa.BuildAnswerPrompt`、`qa.AcquireQueryEmbedding`、`qa.ValidateCitation`、`qa.RetrieveEvidence`、`qa.PlanQuestion`、`qa.WriteAutomaticCache`、`qa.ReadDebugTurn`、`qa.ListTurnRecords`、`qa.ReadTurnRecord`、`qa.PublishOutbox`、`qa.CommitCompaction`。
- 新增实际助手：`qa.ResolveTaskMemory`、`qa.CursorActor`、`qa.ModelText`、`qa.answerEventStream.Next`、`qa.answerEventStream.Close`。累计29个原业务与48个助手，共77项源码ready，合同114项。
- 核心新文件：turn/context、context/preflight、answer/retrieve/prompt/citation、cache/query-embedding/automatic、admin/records、debug/read、history/commit、events/outbox。源提交内也同步contracts/types/OpenAPI/data/workflows及首批窄修。

## 实际边界

Planner从真实模型结构输出读取resolvedReferences，缺失false；不用关键词扫描。独立读取qa.preflight记忆，失败结构仅一次格式重试且新步骤幂等key，不重试未知付费调用。风险仅发送签名许可既有scopes与真实模型判断，abusive显式转abuse，不伪造similarity/interval；严格allow/deny/challenge。

qa.answer记忆按task与language独立读取，byteSize是完整渲染text的UTF8字节数，允许空文本0B。回答提示只消费上游已按真实token规则裁剪的材料，完整窗口裁剪/模型回答链仍planned，不用字符数伪造tokenizer。

查询向量复用knowledge真实API，传原消费签名permit并严格核对空间、逐维有限、向量hash/问题hash/epoch。检索只接受最多8块及真实token预算，ReadSections对完整来源与偏移/文本hash逐项一致；引用调用content及public-site当前语言权威，不伪造href。knowledge三个内部输入的admissionPermit增补由根负责。

自动cache只来源当前持久completed轮次、可信eligibility、原答案与引用，负反馈后不写；空引用保留公开发布集合摘要，ID确定且不延长已有TTL。读取记录保留引用来源，已知失效显式标false，权威unknown整次失败。压缩提交只接受原固定连续范围/原摘要hash、有效租约与historyGeneration CAS，旧代标superseded，原消息不删。

## CI与后续

本批未执行本机Go/gofmt/gen/build/vet/check/测试/真实接口/模型/服务；只Git审阅和diff检查。全部ready函数blocked待GitLab。生成器需收齐新增运行schema、memory空text合同、guard可选测量字段与严格decision；根集成公共变更后统一生成docs/gen。没有手改生成目录或go.sum，没有新依赖。

已包含根首批CI修复同义改动：gen.TypesQaPolicy、ReadConversation SQL文本expiry转time.Time、内部runtime.QuestionPlan共享，合并冲突保留实际正确版本。契约implementation.phase必须design，不等于进度phase。

下一批继续原业务：会话轮次生产/终态/取消/删除、缓存审查与压缩生产执行等。完整worker需要平台CheckLease在外部调用前后验证，并在本地提交核对epoch/fence；不得把外部模型IO错误绑定到不会续的本地30秒until，也不能自动重派unknown模型调用。本批尚未宣称完整BuildApplication、ExecuteTurn或RunWorker就绪。

---

# 2026-10-18 QA 第一批交接

源码 `fea42043fea9c83681573a1f8ba770ec019a26b9`；分支 `codex/qa-20261018-bulk`；基线 `82179b907d25a546e9f470a1fb9cf1584ab45088`。根公共 GONE/410 的 `fc22dda` 作为基线 cherry-pick 为 `28614ed`，不要当成本域代码再次合入。独占修改仅 `modules/qa/`。

## 代码就绪，待 GitLab

17 个原有业务函数（其中 NormalizeQuestion 是共享迁移并按ADR0006移除通用纯函数取消检查；其余16项新增/解阻）：

- `qa.CreateConversation`
- `qa.GetConversation`
- `qa.ValidateAdmission`
- `qa.NormalizeQuestion`
- `qa.AppendAnswerEvent`
- `qa.ResumeAnswerStream`
- `qa.LookupExactCache`
- `qa.ValidateCachedAnswer`
- `qa.ProposeCacheEntry`
- `qa.DisableCacheEntry`
- `qa.RecordFeedback`
- `qa.ReadQaPolicy`
- `qa.ReadPublicCapability`
- `qa.ListCacheEntries`
- `qa.ReadCacheCalibration`
- `qa.EvaluateCacheEligibility`
- `qa.InvokeModel`

43 个实际助手（全部已登记实际签名与调用边）：

- `qa.NewRuntime`
- `qa.WithRuntime`
- `qa.FromContext`
- `qa.DecodeResponse`
- `qa.StorageError`
- `qa.RequireAdmin`
- `qa.Transaction.Close`
- `qa.Transaction.Commit`
- `qa.BeginRead`
- `qa.BeginWrite`
- `qa.beginTransaction`
- `qa.ReplayReceipt`
- `qa.SaveReceipt`
- `qa.ReadPolicy`
- `qa.ReadConversation`
- `qa.ReadTurn`
- `qa.IsTerminal`
- `qa.CheckTurnLease`
- `qa.SaveTurn`
- `qa.ServiceReadContext`
- `qa.ReadPublishedDocument`
- `qa.ValidateCitations`
- `qa.PublicationFingerprint`
- `qa.validInputText`
- `qa.normalizeWhitespace`
- `qa.foldNaturalCase`
- `qa.isProtectedToken`
- `qa.hasSignificantUppercase`
- `qa.extractAnchors`
- `qa.isAnchorToken`
- `qa.ScopeHash`
- `qa.ReadCache`
- `qa.SaveCache`
- `qa.InsertCache`
- `qa.EmitCacheChanged`
- `qa.SaveEvent`
- `qa.EmitCacheInvalidated`
- `qa.ineligible`
- `qa.NewOperationHandlers`
- `qa.jsonOperation`
- `qa.operationContext`
- `qa.writeResult`
- `qa.serveAnswerStream`

共60项统一blocked待集中GitLab；不记implemented/verified，不代表服务可运行。部分文件尚含planned函数，其文件整体职责继续blocked。完整轮次/模型执行、缓存批准/自动/语义/翻译、压缩和恢复仍未完成。

## 关键事实

- 私有事务核本站与代次，写入使用平台安全窗口；同主体/函数/键七天原回执保留，同键异输入冲突，原结果校验摘要。策略、轮次和缓存的持久JSON边界使用生成schema；无原始SQL错误、IP或凭据日志。
- 会话创建在线验证并消费guard原完整签名许可，精确绑定action/request/subject/epoch/audience；重放先查原回执。GetConversation只按owner读取，签名ordinal分页；过期GONE/410。
- AppendAnswerEvent严格UTF8/16KiB、有效租约、非终态事件，事件与服务端序号同事务。模型来源的内部sourceStep/sourceSequence幂等；异内容拒绝。终态必须另由终态用例同时提交。ResumeAnswerStream仅读持久事件，24小时后410；HTTP回放15秒不占序号心跳、5分钟连接上限，断线不重跑模型。
- 精确cache要求完整问题，优先精选，普通cache必须匹配可信调用链固定的当前qa.answer记忆hash。每次交付经content精确发布/完整正文/块hash及public-site当前语言href权威验证；空引用答案保存并重取整个公开发布引用集合摘要，不靠迟到事件放行。
- 精选候选只来自具有持久上下文和当前引用的completed问答；管理员明确输入清理后的基准语言公开问答，pending不命中，不自动复制私人原文、不自动翻译。反馈精确匹配答案修订，负反馈使来源普通cache stale并移除向量，不自动撤销精选，事件只含ID/评分/版本/状态。
- Planner参考合同追加resolvedReferences，默认缺失false。前置提示规则写入workflows：只有可信模型对实际历史/已核当前页明确完成独立问题及全部指代才可true；仍依赖动态页面/私人上下文或needs_clarification拒绝共享。PlanQuestion本体仍planned，公共入口不接受该字段。
- 模型仅平台NewGatewayClient/InvokeModel；本批真实校验器从生成PayloadValidationSpec建立，不在本机生成。ModelRuntime许可只能按当前可信轮次装配，不能在服务启动塞静态单访客许可。

## 结构及生成

`internal/turn/normalize.go` 已移动为 `internal/normalize/question.go`，纯算法被cache和未来turn共用，避免导入环；根framework需要同步该唯一变更。其余实际文件均为既有未来路径。implementation共109函数，包含43个必要助手，非空壳增量。新增SourceStep/SourceSequence是内部参考DTO，不是公共输入能力。

OpenAPI补三入口GONE/410、流Last-Event-ID与5分钟基础设施截止，保留生产方及私有JSON运行schema引用闭包。根CI须先make gen/docs，得到新gen.PayloadValidationSpec、public-site/content新增读取客户端，再编译。没有手改生成文件或go.sum，没有新库依赖。

## 检查与下一项

本地仅JSON解析/台账同步、Git身份审阅和git diff --check通过；未运行Go/gofmt/gen/build/vet/make check、测试、真实接口、模型或服务；不push。用户已要求及时检查和推送，根统一GitLab执行。

下一项继续本域第二批：缓存批准/自动保存、真实查询嵌入与上下文/模型消费者、记录读取、轮次终态和压缩提交；每批完成即源码加台账提交交根CI，不等待整个QA或75%才检查。

---

以下为旧历史交接；指代设计缺口已经本次根裁定补齐，不再沿用永远拒绝的旧规则。

# 2026-10-17 缓存准入交接

- 分支：`codex/qa-20261017-r5`
- 基线：`a3e6af980d5288e42bc8d023410faf8923ff21e8`
- 源码 SHA：`7ef0cfb4f1e695a82dcecb5e84e1b0825a7c62fe`
- 独占范围：`modules/qa/`
- 函数：`qa.EvaluateCacheEligibility` 保持 blocked，验证 not_run。accepted 为 0。
- 检查：pipeline 48 已编译该包。函数仍 blocked，accepted 为 0。未做业务验收。

## 行为

`EvaluateCacheEligibility` 是纯计算，不读写缓存，不调用嵌入、语义缓存或模型。

按已写明字段拒绝共享：

- `plan.complete` 不为 true：eligible=false，reason=ambiguous
- `privateContextDependent` 为 true：eligible=false，reason=private
- `currentPageDependent` 为 true：eligible=false，reason=ambiguous
- `classification` 为 out_of_scope：eligible=false，reason=out_of_scope
- `plan.cacheEligible` 不为 true：eligible=false，reason=ambiguous

`firstTurn`、风险、子问题和问题原文不参与判定。独立 canonical 问题是否残留这/它或未命名第二步来源没有可执行规则。这些输入即使都通过，仍返回 eligible=false、reason=ambiguous。函数不返回 eligible=true。

入口之后信任已校验的 input 与 op，不重复检查截止、身份、dataEpoch 或屏障。

## 合同

`implementation.json` 的步骤与文件职责已写成上述行为。`types.json`、OpenAPI、events 未改。`make docs` 重写了 `docs/functions.md` 与 `docs/files.md`。

## 需要集成处理

- 把 `modules/qa/internal/cache/eligibility.go` 登记进根 `docs/structure/framework.json`。
- 审核后跑 GitLab CI。不要把本函数记为 verified 或 accepted。

## 下一步（一项）

pipeline 48 已编译 `EvaluateCacheEligibility`。指代残留没有可执行判定之前，不要让它返回 eligible=true，也不要扫描问题文本。

下文是历史交接，其中的下一步不再执行。

---

# 问答与缓存会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `NormalizeQuestion` 先按 BCP 47 规范化语言标签再计算问题哈希，无法解析的标签按 VALIDATION 拒绝。
- go.mod 补 kin-openapi/oapi-codegen runtime 依赖并生成 go.sum；台账 baseCommit 修正为完整 SHA。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。

## 本次事实（R1-16 NormalizeQuestion 纯规则切片，2026-10-15）

- 基线：`8ae6fcf6302e2fd41a998faef18be52e2a9714c`；实际分支：`main`；交付代码提交：`4f4d11a4a6f44f2bd69def6d9a8ee7701a97eb6d`。未 push、未部署。
- 独占范围仅为 `modules/qa/`。本次修改 `internal/turn/normalize.go`、`go.mod`、`STATUS.md`、`progress.json`、`docs/handoff.md`；未修改合同、生成目录、根文件、libs、deploy、其他模块、测试或样例。
- `qa.NormalizeQuestion` 已实现为无副作用纯规则：检查上下文取消/截止、UTF-8和输入上限；用 NFKC 规范化全半角，统一 CRLF/CR/Unicode 空白；折叠自然语言大小写并保留数字、代码/产品标识、否定词；拒绝超过500 Unicode字符或8192字节的问题；按 `language + NUL + normalized` 计算 SHA-256；按稳定规则提取数字、代码/产品标识、否定词和连续汉字锚点，最多256项。
- `OperationContext` 只作为可信入口元数据接收；该纯计算切片不自行解释授权字段，不读写问答记录、缓存或 outbox，不访问 memory/knowledge/content，不产生模型费用。

## 检查与阻塞

- 通过：`python` 解析 `modules/qa/progress.json`；`git diff --check`；提交暂存范围仅为 `modules/qa/`。
- 未运行：`go test/build/vet`、`gofmt`、`go mod tidy/download`、`make check`、普通测试、业务验收和部署。当前环境无 Go、gofmt、make；`go.sum` 需在具备 Go 工具链后由模块工具生成，未手写替代。
- 完整根结构检查仍需集成会话登记新增 `modules/qa/internal/turn/normalize.go` 与 `modules/qa/go.mod`；本模块不修改公共 `framework.json`。
- 工作树中已有并行会话的 `modules/model-gateway/go.mod` 修改和 `modules/model-gateway/migrations/` 未跟踪文件，本次未触碰、未暂存、未提交。
- 未发现本切片需要修订的契约缺口；其余函数的存储、HTTP、跨域客户端、任务循环和真实模型依赖仍未实现，不能用占位值接线。

## 下一步（一项）

实现 `qa.EvaluateCacheEligibility`（`modules/qa/internal/cache/eligibility.go`）：先核对本域 `types.json`、`implementation.json`、`docs/functions.md` 和 `docs/ai-context-and-cache.md`，只实现已补全/非个性化/非敏感动态问题的确定性准入规则，不接 semantic cache、embedding、真实模型或持久化；完成后同步三份本域记录并保留未验证状态。


## 第四轮首页跨域只读协议

新增readPublicCapability免费只读入口配置能力，未知503由首页禁用QA按钮但保留hero。生产方函数planned，不以readyz或无参数假成功充当AI已配置。
