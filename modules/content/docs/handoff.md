# 文档生命周期会话交接

## 本次事实（R1-06 草稿提交事务边界，2026-10-15）

- 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`。分支：当前为 detached HEAD；模块交付源码提交：`448ad37`。
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
