# 当前工作包：文档树与资源引用纯规则（2026-10-16，已交付）

分支codex/content-persistence-round1，基线170c1dc9057e122e349de43db532b2c48434b6cf，初始工作区干净，独占modules/content。本轮用户要求三个子代理并行一轮，由根统一集成与VPS检查。

- 源码提交f7bb9b8c029f3cc6af996708d28f50492ffb0546：新增既有登记的internal/tree_validator.go与internal/reference_extractor.go，2完整文件/5函数按准确源码SHA记verified。
- 树规则检查当前完整父索引、重复ID、缺父、环和32层；拟移动拒绝自身/后代，按整棵子树高度核移动后上限，后代ID字典序返回，输入不变。
- 资源规则按正文前序和固定角色顺序保留所有嵌套资源、画板3文件、上传视频及平台本地cover/poster；稳定sourceKey去重保留不同block，完整引用冲突失败关闭，水位/当前历史分类保留，最多10000项。
- 原持久化适配切片因本域尚无登记的私有DB schema/迁移文件与启动打开协议而未展开；根同意改做上述既有纯规则。真实准备记录、COMMITTED资格、隔离adapter与BuildApplication/main继续blocked，未猜建表、未放松head/ledger/refs/receipt/outbox同事务。
- 根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、两源码gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（861文件/1185函数/379操作/2269类型）。暂存内容与提交信息静态隐私扫描通过；verified仅表示当前编译与结构检查，不代表业务验收。
- 不改正文schemaVersion、OpenAPI/公共类型或其他域；未本机Go/gofmt/编译/测试，未新增测试/mock/fixture/样例/临时验证程序，未调用真实接口/模型或部署，未push。本域本轮交付后停止，由根集成，不自行启动后续工作包。

---

# 当前工作包：第五轮不可变正文文件核心（2026-10-16，已交付）

分支codex/content-revision-files-r1，基线9ff93ae71e8f262ea0fc057aa3f6eea71ce52f22，独占modules/content。上一public-site纯契约f9aa83f/9567637已独立交付并保留分支，本轮不再改该域或根。

- 已写WriteRevisionFile/ReadRevisionBody及直接要求的CanonicalizeBody/HashBlock、树/行内/表格/分栏语义源码。准备原材料先持久、复用原结果，fsync/rename不推进head；读取仅可信COMMITTED资格，坏文件真实隔离无fallback。
- 新增实际路径仅既有登记的internal/{body_canonicalizer,block_validator,table_validator,columns_validator,revision_reader}.go；原revision_files/revision_repository复用并统一16MiB硬限。接口扩展遵守根裁定，无DB adapter假实现，无新迁移或真实HTTP成功占位。
- 源码提交02ede7bdfd70b263511a1f82343c602a3c73143c，7个完整核心文件、35个本包完成函数及VPS生成客户端按准确源码SHA记verified；check/gen/build证明编译和结构，不代表业务验收。
- 最终隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14现有Go模块readonly build/vet/gofmt及完整make check通过（859文件/1142函数/379操作/2264类型），覆盖最终同一time.Time与contentInteger修订。
- 未本机Go/gofmt/gen/编译/测试，未新增测试/mock/fixture/样例/临时验证程序，未运行真实API/模型/服务，未push或部署。本轮交付后停止实施，由根统一集成、复核与推送，不启动后续包。
- 真实私有DB准备/提交资格/隔离adapter、保存/发布/HTTP/后台仍缺，BuildApplication/main继续blocked；既有fence/稳定ID历史ledger/head/refs/receipt/outbox事务责任未转移到文件准备。

---

# 之前工作包的历史记录

更新：2026-10-15。阶段：R1 实施中；本域业务尚未验收、尚未部署。

- 2026-10-15 集成复核修复：`cmd/content` 补 `Application.Serve/Shutdown` 方法，入口恢复可编译；运行状态适配解开 `state` 外层。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次 `R1-05` 基线为 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`，独立分支为 `codex/content-r1-05-recovery`；前序启动准备仍由 `f62ea3d7c52bc98ebd73d94c1428ca8794caf751` 提供。
- 本次已实现 `content.RecoverPreparedCommits`（提交 `f376a3e779cd7cffc0f1f214a99b8b6ea5090055`）：恢复只能在注入的独占启动/safe 屏障内进行；校验严格 journal、正文哈希与已有 head；有 committed receipt 的 PREPARED 才补记 COMMITTED；缺正文、哈希不符或 head 不一致的提交隔离并关闭文档可见性；函数没有推进 head 的能力。`content.WriteRevisionFile` 及 `BuildApplication` 接线仍按范围保持 planned/blocked。
- 本次实现 `content.CommitDraftJournal`：按 operationId+规范摘要先查已提交 receipt；对 PREPARED 正文拒绝符号链接/非正规文件，复核大小、SHA-256、文件同步与目录同步；校验旧head、草稿版本连续性、journal与draft一致性及站点/dataEpoch；由注入的私有事务在同一边界执行 fence检查、head CAS、稳定ID台账、COMMITTED journal、refs、receipt和outbox。未注入事务存储或发生不确定存储错误时明确返回 `DEPENDENCY_UNAVAILABLE`，不将启动伪装为可用。
- 本次源码与模块台账提交为 `448ad37fbb2980e0eab87845be100d2e731bc409`；该提交只包含 `modules/content/`。
- 状态区分：上述源码为 `implemented`；仅完成 JSON、隐私、差异空白和暂存内容静态检查，Go 编译/gofmt/行为恢复检查未运行，故未标 `verified`；业务验收为 `accepted` 之前的未执行状态。
- 已创建启动与部署准备文件：`cmd/content/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`Makefile`、`config/service.defaults.json`、`deploy/manifest.json`、`Dockerfile`；实现提交为 `f62ea3d7c52bc98ebd73d94c1428ca8794caf751`。
- `content.main`、`content.Serve`、`content.Shutdown`、`content.Health`、`content.Ready` 已写入；`content.BuildApplication` 已接入配置、信任锚、site-config运行权威和恢复换目录前置检查，但因业务存储、journal恢复、全部OpenAPI x-handler、后台函数和activation回执未实现，按合同返回 `DEPENDENCY_UNAVAILABLE`，保持 blocked。
- `go.sum` 未创建：当前环境无Go工具链，VPS不可用，不能用手写内容代替 `go mod download`。Go build、vet、gofmt、make verify、Docker构建和业务验收均未执行。
- 本地Python JSON解析与隐私检查通过；`git diff --check` 通过；根 `scripts/check_docs.py` 因公共结构清单尚未登记本域新增文件而报告未登记项。根结构清单属于集成会话所有权，本次未修改。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用；未push。

逐文件/函数事实见 [progress.json](progress.json)，交接证据见 [docs/handoff.md](docs/handoff.md)。公共资料由集成会话维护，本域只修改 `modules/content/`。

- 本轮集成适配：生成器修正nullable引用后，草稿前序修订按指针空值与expectedHead比较，首次提交和后续CAS语义保持一致；本域build/vet/gofmt通过，业务验收未执行。
