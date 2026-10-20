# 2026-10-21 复核：SnapshotManager drain 保持 partial/blocked

已核实本域当前 `BeginDrain/Drain` 只读取并轮询真实 `guard_permits(state=active)`、`guard_challenges(state=issued)`、`guard_waiters` 和 outbox 工作水位；没有在屏障期间自动把这些事实改成终局状态。`ExpireLeases` 是普通后台入口，`guard_challenges` 没有独立到期终结后台，等待队列也没有可证明的批量清理授权。

- active permit 只能通过原 owner/root 的 Release 或自然 expires 终结；issued challenge 只能由挑战验证或既有过期清理路径终结；waiter 只能由原 QA 会话按 requestId cancel/消费路径处理。当前没有 barrier 专用、带原资格证明的 cleanup 回调。
- `AuthorizeCleanupOrigin` 保持 `FORBIDDEN`，不会把随机协调器 root 或匿名 barrier 操作伪装成旧执行授权。`Drain` 在这些行仍存在时只能继续等待并最终返回 deadline/失败，不能返回 safe。
- 因此 `NewTrafficSnapshotManager` 已从 `BuildApplication` 构造并注入真实回调，但本域 SnapshotManager 的 drain/safe 闭环仍为 partial；`BuildApplication`、safe/export、完整服务继续 `blocked`。全域 `VerifyGlobalSeal` 仍显式 `DEPENDENCY_UNAVAILABLE`，等待 jobs 协调器。

# 2026-10-20 续作修正：复用集成平台统一 outbox work gate

上一版新增的 `migrations/0004_work_gate.sql` 与 `internal/snapshot_work.go` 未在原本域文件合同登记，已由提交 `09328bf3` 撤销；当前不再保留这两个路径，也不复制平台触发器或另建水位表。

- 源码提交：`e611720a33f56a22e012dc4b45849437a649f8fa`。在已登记的 [guard_runtime.go](internal/guard_runtime.go) 与 [policy_store.go](internal/policy_store.go) 接入集成平台 `outbox.RegisterWorkTable`、`Store.ObserveTx`、`Store.SealTx`、`Store.ResumeTx`。
- `RunMigrations` 完成后登记本域真实业务表；`ObserveWork` 在同一 SQLite 读事务逐表读取 pending login attempt、active permit、issued challenge 和等待队列，`WorkRevision` 直接复用 `nw_outbox_control.revision`。
- `SealWork`/`ResumeWork` 复用同一 `sql.Tx` 与平台事务 API，业务 gate 与 outbox 一起提交/回滚；未新增本域迁移或第二套 gate。
- 本次新增/修改尚未进入用户指定 CI，状态保持 `blocked`。未本机/SSH运行Go、gofmt、build、vet、test、pnpm、make check，未运行真实接口、模型、服务或部署。

# 2026-10-20 续作：受控快照归档读取源码单元

本工作树已从用户指定 `main=6204dfc94510f28752fe8da81df9f3a4d3ce781d` 建立续作分支 `codex/traffic-guard-20261020-r1`；旧的 `codex/traffic-guard-20261018-bulk` 分支引用保留。源码提交为 `0c0e2e17e00adf2bb159de97c31f7bf8b58892b0`。本次只改 `modules/traffic-guard/`。

- 新增 `internal/snapshot_archive.go`：`ReadSnapshotArchive` 要求 data-protection 服务携带已验证 admin 会话，核当前维护 backup/restore 屏障与 active dataEpoch，固定 traffic-guard、版本1、`application/x-tar` 的 `ObjectRef`，调用平台 `SnapshotAPI.OpenArchive`，缺真实管理器依赖时失败关闭。
- `internal/guard_runtime.go` 保存真实运行权威回调与 `SnapshotAPI`；`internal/http_management.go` 接入 `readSnapshotArchive` 二进制流路由；`internal/app/wire.go` 登记 data-protection 调用方、maintenance/controller scope 和运行回调。
- 仅做静态合同审阅与 `git diff --check`，通过；未在本机或SSH运行 Go、gofmt、build、vet、test、pnpm、make check，未运行真实接口、模型、服务或部署。按用户要求本单元及受影响文件保持 `blocked`，不记 implemented/verified。
- 完整 `BuildApplication` 仍失败关闭：真实 `SnapshotManager` 回调、写窗口、预算key、其他快照/后台/事件处理器和恢复回执尚未装配；没有用空 manager 或空 scope 放行启动。

# 2026-10-18 续写第二批检查通过

GitLab pipeline84（源码72219d7；生成381、三个Go分片382/383/384、前端385）全部成功，16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及前端类型检查/构建通过。pipeline83各编译分片通过但缺既有信封校验登记，已补并复检；实际普通测试无测试文件。源码生成物由GitLab工具回传，不代表业务验收、隔离行为验收或部署。

本域本批1项完整函数通过源码检查，完整启动/快照恢复/业务accepted状态保持未完成。模型网关、知识、数据保护三个worktree继续，未在本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 2026-10-18 第二批集成检查通过并已双推源码

自部署GitLab pipeline70（生成311，三个Go分片312/313/314，前端315）全部成功；精确879713字节生成物回传到3deb8ba9cd394ec43274999eaf4f5e0484e224ae；pipeline71对该精确SHA再次全部成功并核对生成零漂移。16个Go模块readonly build/vet/gofmt与go test、完整make check、Vue类型检查与构建通过；go test无测试文件，不代表业务验收。

本域本轮精确清单58项记verified，模块当前75 verified、4 implemented；accepted=0。源码交付 `3deb8ba9cd394ec43274999eaf4f5e0484e224ae` 已普通双推并核对两个远程main一致；本结果记录在同次授权下复查同步。完整服务装配、恢复与其余具体未完项继续按台账blocked/planned，不以编译通过代替业务验收。模块代理均已冻结。

以下为历史记录，其中待CI/真实设计阻塞以最新逐项台账为准。

# 流量风控当前进度

更新：2026-10-18。用户要求持续并行实现，目标全仓至少75%，并及时经根GitLab检查及推送。独占分支codex/traffic-guard-20261018-bulk，基线0a69a5a，首批源码e8d9215。

首批16个既有主要函数与11个必要参考助手已有源码，全部blocked（待统一GitLab），不提前implemented/verified；原未改登录17函数与存储函数保留既有证据，accepted仍为0。已完成策略可信站点读取/首次0→1与后续CAS、同事务原安全回执/outbox、AI禁令/撤销/分页、三层桶（第二批补齐网络）、真实匿名风险信号、FIFO排队与显式取消、到期及保留清理。

ReportQuestionSignals只收已有前置风险，决策严格allow/deny/challenge；similarityScore/intervalMs可选，缺失保持未知/SQL NULL，不按0观测计数或增加信任。主体与网络仅取原guard签名scope，不收问题正文或原IP。

完整BuildApplication仍失败关闭，运行写窗口/独立预算key/其他HTTP/后台和恢复回执尚缺。下一批继续使用根新增平台统一许可/通过证明签名和30日网络预算scope，不在本域复制签名。

本机仅阅读和Git差异审阅，未运行Go/gofmt/生成/编译/SQL行为验收/服务/真实接口。未新增测试、fixture、mock、样例或临时验证程序。

第二批源码5b44ea5新增10个原有主业务函数及8个必要参考助手，仍blocked待GitLab：匿名主体、原根准入、并发分配、消费签名、续租释放、原签名/在线验证和HTTP入口。三层频率原子扣减，网络次数字段兼容默认600。先分配空token，只有原根已消费QA可签模型凭证；免费提示永不签模型token。继续挑战与事件，不等完整服务才交CI。

第三批源码f5f036b完成挑战4项及身份通知/转发2项，累计32个原有主业务函数源码。5个必要挑战助手同步实际合同；全部仍blocked待统一GitLab。删除不再使用的NormalizePeer参考合同，CheckBan只保留实际permit_store.go版本。ConsumeEvents公共窄接口/准确统计缺口保持partial；完整启动仍失败关闭。已冻结，由根统一gen/docs/readonly build/vet/gofmt/check与双推，本域不再扩功能。
