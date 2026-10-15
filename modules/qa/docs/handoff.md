# 问答与缓存会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `NormalizeQuestion` 先按 BCP 47 规范化语言标签再计算问题哈希，无法解析的标签按 VALIDATION 拒绝。
- go.mod 补 kin-openapi/oapi-codegen runtime 依赖并生成 go.sum；台账 baseCommit 修正为完整 SHA。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增文件/函数暂记 `in_progress`，待修复提交后回填 SHA。

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
