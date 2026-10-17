# 2026-10-17 缓存准入交接

- 分支：`codex/qa-20261017-r5`
- 基线：`a3e6af980d5288e42bc8d023410faf8923ff21e8`
- 源码 SHA：`7ef0cfb4f1e695a82dcecb5e84e1b0825a7c62fe`
- 独占范围：`modules/qa/`
- 函数：`qa.EvaluateCacheEligibility` 保持 blocked，验证 not_run。accepted 为 0。
- 未跑 CI：未执行 GitLab CI、go、gofmt、make build、make gen、make check、SSH。本域执行了 `make docs`。源码提交前 `python scripts/check_docs.py --privacy-only` 通过。

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

`modules/qa/internal/cache/eligibility.go` 已登记。GitLab CI 还没覆盖这次提交。指代残留没有可执行判定之前，不要让 `qa.EvaluateCacheEligibility` 返回 eligible=true，也不要扫描问题文本。

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
