# 持续实施第一轮：授权安全显示名（2026-10-16，VPS检查通过）

用户重新授权每轮三子代理编写、审核并push至R1代码完成。集成会话为content编辑锁补齐公共前置：AuthorizationDecision追加当前已校验主体的accountName，readSessionSnapshot同一账户JOIN读取不可变账号名，不新增任意账户读取能力、不改变grants。来源685db4f，独占本次identity修改；本域原后续服务器重置工作包仍未实施。

隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14模块readonly build/vet/gofmt及完整make check通过（869文件/1299函数/379操作/2289类型）。首次生成因既有pnpm未在PATH失败，补入已安装入口后完整重跑通过；未改工具版本或新增依赖。未新增测试/样例，未执行SQL/HTTP行为验收、真实接口/模型、服务或部署。源码提交a5b671322a120c3d1eeb5a63e9f4d06dd9ccb5a9；两项修改函数与对应源码/生成文件按此SHA记verified，证据仅为现阶段检查。

---

# 当前工作包：第四轮本人后台偏好（2026-10-15，VPS现阶段检查通过）

按持续实施授权，独占modules/identity，分支codex/identity-preferences-r1，精确基线de21a2bcb5e1a8b25e41ec84ba3734fc163b5ab1。ReadPreferences/UpdatePreferences与真实HTTP绑定已通过现阶段检查：当前本人站点/账号会话权威、受限会话拒绝、固定缺省、偏好CAS、完整私有安全原回执；无账号/会话/index/outbox变更。

locale与后台route key由根公共目录统一生成，入口由公共偏好schema校验，身份只按gen.AdminRoutes核目标页权限，不复制portal私有路由或信任前端授权。约束分支已同步；新增实际文件仅既有登记的internal/preference_service.go，不新增迁移。最终VPS证据：2026-10-15 第四轮本人偏好与公共目录/portal消费者冻结快照：隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14个现有Go模块make build（-mod=readonly build/vet/gofmt）及完整make check通过（856文件/1049函数/374操作/2222类型），生成/格式回传0冲突。该快照不包含仍在写的site-config/jobs第四轮改动，不声称其已验证。未新增或运行业务测试、SQL行为验收、真实接口/模型、服务/镜像/部署；verified仅表示现阶段检查，不是行为验收。

本轮源码`b00aae001b2fe117e908cdc9fec5f46808a7db02`（16个本域文件），ReadPreferences/UpdatePreferences及2个完整助手、偏好文件/本轮生成物已转verified。完整BuildApplication/main仍blocked，transport仍缺服务器恢复/屏障等handler及后台，不当成可启动服务。公共来源已复用精确既有提交bd748982e9ddc5dc1e55eaf7b2b92e660cba79cb，没有自行改公共。未本机go/gofmt/生成/编译/测试，未push。

下一项为server_recovery.go的ObserveSuperadminResetFile/ApplySuperadminReset紧密组；本工作包交付后冻结，等根分派再开始。

---

# 当前工作包：第三轮账号管理（2026-10-15，VPS现阶段检查通过）

用户已重新授权持续实施，先前第二轮停止说明只保留为历史。本分支codex/identity-management-r1，基线9acad1ab56532521222f592af0d4b3a849792db5，独占modules/identity。

本轮四个已登记函数ListAccounts、SetAccountStatus、SetAccountPermissions、ResetAccountPassword及真实HTTP绑定已通过现阶段检查：唯一超管取权威row，子账号签名键集分页、CAS、原安全回执、即时撤销与索引决定恢复均复用既有机制。私有撤销count解除与会话模型不一致的10000上限，不新增会话产品限制。无新增实际文件。

2026-10-15 第三轮最终合成快照：隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14个现有Go模块make build（-mod=readonly build/vet/gofmt）及完整make check通过（856文件/1047函数/374操作/2222类型）。未新增或运行业务测试、SQL行为验收、真实接口/模型、服务/镜像/部署；verified仅表示现阶段检查通过，不代表行为验收。

本轮源码提交`8424db601aaf0ba822d98c79bf52f293451ebf93`（17个本域文件）；四个用例及完整参考函数、账户/撤销/上下文文件与本轮OpenAPI生成物已记verified。完整BuildApplication/main仍因偏好、服务器恢复、屏障及后台未齐保持blocked，transport装配文件仍in_progress。旧ResolveIdentityOperation及其DTO真实无调用，已删除；原文件保留受信Store上下文/既有仓库前置并据实登记。无新增/删除文件路径，未本机go/gofmt/生成/编译/测试，未push。

下一项为既有ReadPreferences/UpdatePreferences及真实HTTP绑定，先核本域最新合同与当前会话授权；本工作包交付后冻结，由根分派再开始。

---

# 当前工作包：登录注册与账号口令事务（2026-10-15，VPS现阶段检查通过）

## 第二轮最终集成与停止范围

- 登录/首次注册/子账号/改密/自身资料与认证HTTP核心通过；全域启动和未完成管理/恢复/后台仍blocked/in_progress。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


分支codex/identity-login-accounts-r1，精确基线468986fc34e1c90b6b5ca575539fdd2b9748cd06，仅本域。源码提交`491f3e049b98f01b85ee4e9bf69b6b3c147fadc9`（28文件），已准备CreateSession、RegisterFirstSuperadmin、CreateAccount、ChangePassword、ReadSelf真实用例源码，使用guard版本化begin/finish准入与账号/网络HMAC，未知依赖失败关闭；新增独立control bootstrap不可逆决定、私有HMAC/keyId回执与账号索引决定恢复。raw token仅首次提交响应，不进入普通回执。

第二轮最终修复快照在VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过（855文件/1012函数/374操作/2211类型），源码补丁`8f8f515ba9d5bfd3d4d143ed23efd4c5c1fcac4a`包含control目录链及SQLite正规文件边界。已完成用例转verified，账号管理/服务器重置等未完成文件保留in_progress，完整BuildApplication/main仍blocked；实际guard业务、portal cookie链和其余handler不能视为已完成或已验收。未本机编译/格式/生成/vet/测试/运行SQL，未新增测试/样例/临时验证程序，未运行真实接口、模型、服务、部署或push。按用户最新要求只收口第二轮，之后由根推送并清空目标，不开展第三轮。


---

# 当前工作包：会话签发刷新与即时授权（2026-10-15，VPS编译与结构复核通过）

最新统一集成事实（2026-10-15）：会话签发/刷新/CSRF/核权与撤销通过；完整登录注册与HTTP/后台仍未完成。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：8092ab4、9c86c83，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


分支`codex/identity-session-complete-r1`，基线`325f56d7b2f6c5567c16d7e21f66d5a88a97e342`；仅拥有本域。源码提交`59070a0645a744e67426f511fcddd2040e46816f`，仅本域20文件；任务入口能力追加提交`2ee5c9395297238ab63c921b688d28e221b26b01`。隔离源码已准备：会话签发、单次refresh轮换、旧值复用先提交族撤销、固定Origin与独立CSRF摘要、即时权限检查、单设备/族/其他设备撤销。追加0003迁移保存认证代次和摘要；恢复决定NewAuthEpoch与数据库权威同步。初始口令仅允许静态action指定的改密、当前登出和必要自身资料，会话列表拒绝。

统一复核追加：MapFunctionGrants根据根生成的JobManagementGrants追加任务入口粗预检能力；具体任务权限仍由jobs按持久owner/type核验，不新增用户开关。根生成目录和函数文档已由VPS生成并回传。

本轮交付函数与完整文件依据VPS gen/docs、14模块make build和完整make check通过转verified（850文件/943函数/372操作/2186类型），部分未完成文件保持in_progress；未在本机执行编译、格式、vet、生成或测试。注册、登录风控、完整改密事务、HTTP与后台装配仍未完成。未新增测试、样例或临时验证程序，未运行真实服务/接口/模型，未部署或push。下文仅为先前已合并批次的历史证据，不证明本轮通过。

---

# 身份权限当前进度

最新集成结果：隔离VPS的14模块make build与完整make check通过；本轮已完成条目转verified，实际主分支SHA见progress.json。部分入口/用例仍未实现，业务验收和部署未开始。下列本机与待登记说明保留为交付时历史事实。

更新：2026-10-15。阶段：R1-01 实施中（本轮补自身会话分页和账户全部会话撤销，待集成全量编译复核）；业务验收与部署未开始。

- 本轮会话撤销工作包：新增 `ReadSessions`、`WithSessionCursorKey`、`RevokeAllSessions` 和追加迁移 `0002_session_revocation.sql`；修正 `PublishIdentityEvent` 外部事务的回执归属，实际实现等待集成全量 `make build` 通过后转 implemented/verified。identity 本域 build/vet/gofmt 已通过；全量构建仅被公共 nullable 生成修复后的 content 适配编译错误阻断，结构检查仅缺根框架登记3个实际文件。未新增测试或业务验收。
- 2026-10-15 集成复核修复：运行状态适配解开 `state` 外层。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本轮并行切片已合并到当前集成分支：账号索引/`accounts` 运维子命令（`138a2b45ae08`）、注册状态与功能权限映射及账号前置校验（`484aa7fb79d5`）、幂等回执查询、身份事件入outbox、恢复后会话失效（`1a6725d3eca0`）。合并时统一了受信 `WithIdentityStore` 上下文，并拒绝覆盖损坏的账号索引文件。
- `ReadRegistrationState`、`MapFunctionGrants`、`ReadServerAccountIndex`、`WriteAccountIndex`、`ResolveIdentityOperation`、`PublishIdentityEvent`、`InvalidateRestoredSessions` 状态为 implemented；仅完成前置校验的 `RegisterFirstSuperadmin`、`CreateAccount` 与缺少 jobs 客户端接线的 `RelayOutbox` 保持 in_progress。
- `main` 的 `accounts` 子命令已解除原有 `ReadServerAccountIndex` 阻塞；`BuildApplication` 仍因完整 OpenAPI handler、恢复回执、`restoreId` 注入、`ObserveSuperadminResetFile` 和 `RelayOutbox` 后台循环而 blocked，不能提前开放完整服务。

- 启动装配组（第一包，提交 533a1239fa8c）：cmd 入口、app 装配、transport HTTP 生命周期、9 文件；Serve/Shutdown/Health/Ready verified。
- 私有持久化（第二包，分支 codex/identity-r1-01-storage，基线 bbb1e535c2f1，实现提交 28af316bf6a）：internal/storage.go 打开 active/identity.db（modernc.org/sqlite v1.59.0，WAL/foreign_keys/busy_timeout=3000ms，单连接串行）、migrations/ 只追加 SQL 迁移（编号连续校验、事务执行、自举 schema_migrations、集外版本拒绝启动）、control/identity-journal.jsonl 决定日志（append+fsync、末行状态生效、open 条目 INTEGRITY_ERROR 隔离）；BuildApplication 接入存储三步，Ready 的 initialized/journalRecovered/activeDataEpoch 与代次比对生效，Shutdown 关闭业务库。本地与隔离 VPS 检查通过。
- 纯规则工作包（分支 codex/identity-r1-04-pure-rules，基线 ac593c57861d2，实现提交 bdcb9128d3ec）：internal/account_validator.go 实现 NormalizeAccountName（NFC、有效 UTF-8、控制字符、首尾空白、64 字符上限）；internal/credential_service.go 实现 HashPassword、VerifyPassword、ValidatePasswordChange（Argon2id v19 固定参数、PHC 编码、成本上限、常量时间核对、敏感缓冲清理）；新增 x/crypto v0.28.0 与 x/text v0.20.0 直接依赖。仅源码已实现，未完成 Go 编译/格式验证。
- 本次超管不可变纯规则（分支 codex/identity-r1-05-superadmin，基线 82c83226b120，实现提交 7a059d4e95f8）：`internal/account_validator.go` 实现 `AssertSuperadminImmutable`，超管仅允许保持 active，子账号按 `active↔disabled→deleted` 状态机迁移，权限列表不改变超管身份。条目状态为 implemented；当前环境无 Go/gofmt，验证状态仍为未运行，M02-R01 业务验收未接受。
- 0001_domain.sql 建权威表：accounts（唯一超管部分唯一索引）/credentials/account_permissions/sessions/preferences/operation_receipts；refresh 族与令牌哈希列随会话工作包按需追加迁移；平台 nw_outbox/nw_inbox 由 EnsureSchema 创建不进本域迁移。
- `ChangePassword`、会话签发/刷新/单会话撤销/CSRF、账号写事务、完整 HTTP handler、超管重置 watcher、屏障/快照仍未实现；具体状态和阻塞逐条见 [progress.json](progress.json)。
- 现网行为事实：无 site-config 依赖或不可达时按合同 DEPENDENCY_UNAVAILABLE 退出；换向后回执未实现前拒绝开放入口；未完成决定日志存在时 INTEGRITY_ERROR 拒绝启动。

本次未新增测试/样例/快照/E2E/真实接口或模型调用，未 push、未部署。历史切片的“无Go工具链”记录保留为当时事实；本轮已使用本地Go完成本域编译/vet/格式检查，集成会话统一完成VPS全量复核与根框架登记，详见 handoff。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
