# 当前工作包：第一轮真实私有正文存储（2026-10-16，已交付）

分支codex/content-next-two-r1，基线d2161f41c441b69f080b7d8cf3d40b88afecb357，初始clean；独占modules/content。用户授权三个子代理并行两轮及每轮根统一检查/push，本域本轮只交付真实正文存储，第二轮待根给新基线。

- 新增internal/storage.go、migrations/migrations.go、migrations/0001_domain.sql。独立RevisionFileStore/WithRevisionFileStore兼容既有完整存储注入，不继承或伪装DraftCommitStore；未添加BeginDraftCommit占位。
- 私有SQLite安全打开/只追加连续迁移及已应用SQL哈希核对，首次新空库绑定可信打开epoch，既有未知/异代库失败关闭。首次Prepare在同SQL事务绑定可信站点并持久原材料；准备不创建文档、不推进head、不造COMMITTED资格。
- 完整scope和摘要原结果回放，合并原8字段JSONL终局及真实COMMITTED回执；提交资格只来自精确修订/准备/原回执联查。坏已提交文件持久隔离并关闭文档资格，坏未提交准备只隔离自身。
- BuildApplication实际打开/迁移/独占恢复后，仍因完整草稿事务、全部handler、activation及后台装配未完成而blocked；恢复换目录后的epoch重绑定未交付时仍失败关闭。maxDocumentBytes和公共写窗口只由已授权上游传入，不虚构配置或开放writer。
- 本轮完整文件和函数按源码提交97b31b1919181a4a697c8259c85ea9575f225a4d记verified；根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，go mod tidy、make docs/gen、14模块readonly build/vet/gofmt与完整make check通过（865文件/1258函数/379操作/2277类型）。go.mod/go.sum和格式/生成物均由根VPS回传，未手写锁文件；本域未运行本机Go/编译/测试。未新增测试/fixture/mock/样例/临时业务程序，未运行SQL行为验收、真实API/模型、服务或部署，未自行push。

- 源码提交97b31b1919181a4a697c8259c85ea9575f225a4d；当前记录提交紧随源码。新3文件/31参考函数及修改的正文读取/恢复源码已逐包检查；首个迁移与epoch同事务，journal共享RWMutex+NOFOLLOW/SameFile避免半行/路径替换，启动反向核所有prepared/committed必须有完整匹配JSONL，preparing无日志合法窗口保留。完整DraftCommitStore/SaveDraft、全部HTTP/后台与恢复activation/epoch重绑定未交付，main/Build仍blocked。

---

## 当前下一步（一项）

本轮先由根完成VPS检查、集成和push；第二轮按根提供的新main基线实施“真实内存编辑锁+持久文档/删除/fence权威前置”。先读edit_lock_service、document_service、draft_service与CommitDraftJournal合同；现有CommitDraftJournalInput还缺leaseId/fencingToken/serverBootId及refs/pin材料，必须先同步内部授权材料，再由真实锁管理器和持久文档状态闭环，不能注入恒成功ValidateFence。能在同一紧密工作包真实接上保存事务时继续，否则保持完整提交链partial；不从本轮空提交表推定保存链可运行。

---

# 本轮交接：文档树与资源引用纯规则（2026-10-16，已交付）

- 基线170c1dc9057e122e349de43db532b2c48434b6cf，分支codex/content-persistence-round1，初始clean；仅拥有modules/content。源码提交f7bb9b8c029f3cc6af996708d28f50492ffb0546；本记录提交紧随源码，台账deliveryCommit和2文件/5函数implementationCommit均指向该源码SHA。
- 先核持久化适配可交付性：本域只有DraftCommitStore/RevisionFileStore窄接口，尚无私有DB schema、migrations路径登记及启动打开/迁移协议；完整准备/COMMITTED资格/隔离适配必须先由集成会话批准结构并收口设计。本轮未悄悄建表或新增路径，根同意转做既有登记的纯规则切片。
- 新增实际文件仅internal/tree_validator.go、internal/reference_extractor.go；EnsureTreeAcyclic/CollectResourceReferences及3个参考辅助，共5函数。implementation参考合同和progress已同步，根VPS make docs已生成files/functions。无正文格式迁移、OpenAPI/跨域/public字段变化。
- EnsureTreeAcyclic先检查完整原树，不用拟移动“修复”损坏原树；父链三色迭代无不受限递归。原树根为第1层，拟移subtree完整高度与新父深度之和不能大于32；返回原树后代ID字典序、不含自己，null moving只校验且返回空序列，输入不变。
- CollectResourceReferences沿content递归前序，按image/attachment.asset、drawing.scene/lightSVG/darkSVG、uploaded.source.asset或platform.snapshot.cover、video.poster的固定角色顺序。nullable资源不造空ref，不取URL、不探测外部服务；平台cover和poster同ref只保留一次。
- sourceKey是content+documentId+revisionId+sourceVersion+kind+blockId+assetId+assetVersion，保留不同block引用；同assetId/version出现不同SHA/长度/MIME返回INTEGRITY_ERROR。资源version用精确整数转换，不借contentInteger的float64通道；当前/发布/历史kind原样保留，retained=true仅表达当前被扫描正文仍保留。去重后10000上限，空序列为[]。
- 本轮冻结源码由根统一在隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8执行make docs/gen、两源码gofmt、14个现有Go模块-mod=readonly build/vet/gofmt与完整make check，全部通过（861文件/1185函数/379操作/2269类型）。根精确回传本域生成/格式文件，未手改生成物；暂存内容、Git身份与提交信息静态隐私扫描通过。
- 2完整文件和5函数按准确源码SHA记verified。正文保存/发布/HTTP/后台和真实私有adapter保持原blocked/planned，不把本轮纯规则解释为可运行服务或业务accepted。未本机Go或新增测试/fixture/mock/样例/临时业务程序，未真实接口/模型、服务、部署或push。

## 下一步（一项）

由根保留源码及本轮记录SHA完成一轮顺序集成并复核实际集成树；本域本轮后停止，不自行启动下一工作包。真实私有DB适配需后续先收口并批准schema、迁移路径与启动打开协议后另行开始。

---

# 第五轮交接：正文文件核心（2026-10-16，已交付）

本轮统一集成实际源码a6cd2bc，最终实际树6c44451通过VPS docs/gen、14模块readonly build/vet/gofmt与完整check（861/1182/379/2269），生成零变化。progress已映射主线实际SHA，后文原分支SHA保留溯源；用户要求本轮结束停止持续目标后push，本域已交付，不开展后包。

- 精确基线9ff93ae71e8f262ea0fc057aa3f6eea71ce52f22，分支codex/content-revision-files-r1，初始clean，仅拥有modules/content。上一public-site源f9aa83fd199efd17593cf349938ef4c823336cfe/记录95676374b480e14d74a7b7251ffb2151bc1cc6f0已保留，无用户内容reset。
- 本项源码交付02ede7bdfd70b263511a1f82343c602a3c73143c，7个完整核心文件、35个本包完成函数及VPS生成客户端按准确SHA记verified。没有真实私有DB adapter，不能从源码/窄接口推定保存链可运行；未执行行为验收。
- 根已裁定参考Write输入追加expectedHead(nullable)/expectedDraftVersion，上游提供已核CAS快照；author仅op，ObjectRef固定revisionId/version1。已有WithDraftCommitStore注入扩展窄接口，保存scope+规范摘要+首次Draft/Journal，preparing/MarkPrepared不推进head；读回资格/隔离必须真实持久，缺adapter失败关闭。
- 原8字段JSONL不改格式，无隐式迁移；16MiB硬限统一旧64MiB（尚无writer历史数据）。新写当前站点大小仅新操作核，旧材料先回放不因配额下降拒绝，历史读只服硬限，不等价上传大小。
- 平台机制只调用lifecycle.NewWriteContext/ValidatePrivatePath，不按合同ID误用platform包。每个阶段刷新当前窗口/打开epoch；tmp和目录fsync、目标已有只核原hash且不覆盖，成功准备与业务COMMITTED分离。
- ValidateBlockTree只本树结构/唯一/语义；历史retired复用由CommitDraftTransaction的ledger+head CAS原子拒绝。历史Read合法包含现已删除块ID，不把previousBody当历史账。持久文件边界用生成schema一次严格验证，纯函数不重做HTTP校验。
- 全树ID≤10000、深度≤32；table≤200×30且覆盖无洞/重叠、宽数等跨度，columns2..4比例和1误差≤1e-6且不嵌套；行内NUL/重复mark、URL userinfo/危险scheme、公式文件网络宏拒绝；保Unicode/代码/latex文本和数组顺序，只排序mark类型与对象键。
- 仅新增既有登记5路径：internal/body_canonicalizer.go、internal/block_validator.go、internal/table_validator.go、internal/columns_validator.go、internal/revision_reader.go。原revision_files.go追加writer，revision_repository.go复用可取消IO并澄清ledger责任；无新DB/迁移/其他域文件。
- 尚planned的SaveDraft/abort媒体协议已按根提醒同步完整source材料+nullable pinId+reason，源持久aborted/rejected后由media封存，pin未知不先Acquire；当前文件核心不实现媒体流程。
- 全部Go格式/gen/build/vet/check仅根VPS；未新增测试/fixture/mock/样例/临时业务程序，未运行SQL、真实API/模型/服务，未push或部署。生成物不手改，完整BuildApplication/main仍blocked。

## 最终检查与停止边界

2026-10-16隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14现有Go模块readonly build/vet/gofmt与完整make check通过（859文件/1142函数/379操作/2264类型）。最终快照包含同一UTC time.Time生成Draft时间和Journal字符串、RFC3339Nano比较及contentInteger对已受schema限制小整数的转换；生成/格式回传无冲突。未执行行为验收。

## 下一步（一项）

根集成会话按本域source/metadata提交顺序合并并复核后，执行本轮已授权推送和总体百分比报告。本域交付后停止并冻结；真实私有DB adapter、上游保存事务及HTTP装配保持缺口，未获新工作包前不开始实施。

---

# 文档生命周期会话交接

## 可空生成类型适配（2026-10-15，集成会话）

- 生成器修复后 DraftRevision.previousRevisionId 正确生成指针；CommitDraftJournal 复用 sameIdentifierPointer 对比 expectedHead 与前序修订，首次正文必须两者均为空，后续必须同值。
- 本域 readonly build/vet/gofmt 通过，最终全量检查随三个工作包集成记录；业务验收未执行。

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `cmd/content` 补 `Application.Serve/Shutdown` 方法，入口恢复可编译；运行状态适配解开 `state` 外层。
- `RecoverPreparedCommits` 重写恢复判定：已提交修订（COMMITTED 或已有 receipt 的 PREPARED）核对正文与哈希；只有每篇文档最新的已提交修订才要求 head 指向它，历史提交不再被误判为 head 不一致而隔离文档；没有 receipt 的孤儿 PREPARED 只隔离该次提交，不关闭文档（对应合同步骤2/3）。
- `normalizeDraftCommitError` 去掉恒同分支并修正非常量格式串（go vet）；补齐 go.sum。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。
- 遗留：恢复逻辑仍未经业务验收；`DraftCommitStore`/`RevisionRecoveryState` 的真实事务实现待后续工作包。

## 本次事实（R1-06 草稿提交事务边界，2026-10-15）

- 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`。分支：当前为 detached HEAD；模块交付源码提交：`448ad37fbb2980e0eab87845be100d2e731bc409`。
- 独占范围：`modules/content/`。本次新增 `internal/revision_repository.go`，并更新本模块 `STATUS.md`、`progress.json`、`docs/handoff.md`；未修改根文件、公共契约、生成物、其他模块、测试、样例、真实接口、真实模型或部署；未 push。
- 合同摘要 `fd5eb7fb13fd92655d9904f6a13d984435386fc9e17658315e9e438ce1e35d79` 未改变。

## 本次实现

- `content.RecoverPreparedCommits` 已实现：只能在注入的独占启动/safe 屏障内扫描本域正文 journal；严格拒绝未知字段、缺失字段、非法状态、非法 ID/hash/时间和不合法状态迁移。
- `COMMITTED` 逐条核对受信路径下正文文件、SHA-256 与当前持久 head；缺正文、非正规文件、哈希不符或 head 不指向该 revision 时，将正文移入本域 quarantine、调用事务状态隔离文档并追加 `quarantined` 记录。
- `PREPARED` 只有已有 committed receipt、正文核验通过且当前 head 已经指向 revision 时才追加 `committed`；没有 receipt 的 PREPARED 只隔离，绝不推进 head。函数的状态依赖接口没有写 head 方法，恢复不会擅自公开正文。
- 恢复依赖通过 `WithRevisionRecoveryStore` 注入；若没有独占启动/safe 屏障、活动数据代次、私有状态或受信 active 根，返回 `DEPENDENCY_UNAVAILABLE`，不伪造成功。未将函数接入 `BuildApplication`，符合本次单项范围。
- `CommitDraftJournal` 通过 `WithDraftCommitStore` 注入本域私有事务：先以operationId+canonical digest查询receipt；未命中时校验fence、草稿/head/journal一致性以及受信路径下的PREPARED正文大小、SHA-256、文件和目录同步，再调用单事务接口完成head CAS、稳定ID、COMMITTED journal、refs、receipt和outbox。事务存储未装配时返回 `DEPENDENCY_UNAVAILABLE`。

## 检查与范围

- 通过：`git diff --check`；本模块 progress、四份 contracts 均可由 PowerShell `ConvertFrom-Json` 解析；源码未发现私有配置、旧业务源码、密钥、真实邮箱或密码标记；提交范围仅为 `modules/content/`。
- 未运行：`go build`、`go vet`、`gofmt`、`make verify`、Docker构建和提交行为检查。当前环境没有 Go、gofmt、make、Docker；未以本地替代编译结论。
- 状态区分：源码与目标函数为 `implemented`；静态检查已记录但不能证明 Go 编译或行为，因此 `verification.state` 仍为 `not_run`；业务验收和部署均未开始，不能标 `verified` 或 `accepted`。
- 未新增测试、样例、快照、E2E；未调用真实接口、真实模型或部署。

## 阻塞与依赖

- `RecoverPreparedCommits` 尚未由 `BuildApplication` 装配；后续接线需要本域私有 DB 的 head/receipt/quarantine 事务实现，并由启动流程提供真实 safe 屏障和活动 dataEpoch。
- `CommitDraftJournal` 仍未接入真实私有DB、正文写入、HTTP handler或BuildApplication；`content.WriteRevisionFile`、`content.ReadRevisionBody`及全部HTTP handler仍按台账保持planned，不能把本次事务边界解释为完整文档保存/发布链路。
- Go 工具链不可用，因此没有生成或手写 `go.sum`，也没有声称编译通过。根公共结构清单属于集成会话所有权，本次未修改。

## 下一步（一项）

实现 `content.WriteRevisionFile`（`modules/content/internal/revision_files.go`）：先读本域 `docs/functions.md`、`docs/data.md`、`docs/workflows.md` 与 `implementation.json` 对应条款，生成不可变 PREPARED 正文文件、fsync后原子rename并写prepared journal；不得在同一项接入HTTP、发布/回滚用例或新增测试。


## 第四轮首页跨域只读协议

PublicTreeNode.canonicalPath为新增required完整路径，只是公开树响应契约，ReadPublicTree尚未实现，不宣称路由可用。ValidatePublications登记site-config只读链接调用方；不能由消费者读取私有树或猜路径。

readReferenceDecision仅media纯服务读取引用源最小决定，COMMITTED优先，缺失/未知不得释放pin。该新约束函数仍planned，不是已实现服务。

窄存储的FindPreparation必须合并原JSONL最新终止/隔离状态与COMMITTED receipt；不能用旧索引复活启动已隔离孤儿。隔离按COMMITTED优先：已提交损坏关闭文档，未提交坏准备只隔离该准备，不关闭另一个合法head；本轮没有假adapter实现。
