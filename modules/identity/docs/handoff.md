# 第三轮交接：账号管理紧密函数组（2026-10-15，VPS现阶段检查通过）

- 用户已重新授权继续实施，覆盖先前“不开展第三轮”的停止指令；旧停止记录保留为历史。独占modules/identity，分支codex/identity-management-r1，从精确9acad1ab56532521222f592af0d4b3a849792db5接续，初始工作树干净。
- ListAccounts只返回同site子账号，含deleted墓碑，不含超管。当前会话、权限与row.isSuperadmin在一致快照核验；accountName/accountId键集排序，默认20/最多100，游标签名绑定主体/site/dataEpoch/子账号筛选，不返回任何凭据/会话秘密。修正原OpenAPI limit任意字符串为整数。
- SetAccountStatus保持active↔disabled→deleted规则；唯一超管只接受active原样无副作用。状态真实变化CAS同时升revision/permissionRevision，RevokeAllSessions在同事务撤销全部session与refresh族并写status事件，重新启用不复活旧登录。删除保留不可再管理墓碑。
- SetAccountPermissions拒绝超管/墓碑，只改既有10项功能。选择排序后做HMAC规范化；集合相同只记安全回执，变化时CAS提升权限水位、替换功能表并发事件。旧access/委托同步失效，refresh仍可在既定安全条件下更新最新权限，不强行撤销其族。
- ResetAccountPassword预核当前超管row/同站点目标/CAS后在写事务外HashPassword，写事务重新核原成功回执和版本，CAS提升credentialRevision、设强改、保持原状态，并同事务撤销全部会话/族。超管自身忘密仍走服务器恢复；原始新口令只参与平台HMAC及Argon2，不入回执或日志。
- 三个写用例先回放原成功再目标CAS；原结果只含安全Account元数据，旧op不重置后来密码/权限或撤销后来新会话。实际变更继续由既有commitAccountWrite执行候选16MiB检查、control决定、active提交和派生索引补齐；重放只用当前权威补索引。
- 根已确认RevokeAllSessionsResult.count改非负int64并删除10000数量失败分支，因为登录未设会话总数上限；不新增产品会话限制。仅本域私有调用及生成类型受影响，无新HTTP端点。
- 四个真实HTTP映射已加入既有server.go。公共入口负责唯一schema/服务身份/委托/epoch/维护校验；写路径accountId从PathValue静态绑定，正文不能冒充目标；其余handler/后台未完，完整启动仍blocked。
- 本域无web/UI生成物。内部Go契约仍由identity OpenAPI；portal浏览器生成按portal OpenAPI x-browser-auth-adapters与Browser*合成认证安全副本，源说明已补，不新增目录或生成文件。
- 仅修改既有登记文件，无新增/删除实际路径。2026-10-15 第三轮最终合成快照：隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14个现有Go模块make build（-mod=readonly build/vet/gofmt）及完整make check通过（856文件/1047函数/374操作/2222类型）。未新增或运行业务测试、SQL行为验收、真实接口/模型、服务/镜像/部署；verified仅表示现阶段检查通过，不代表行为验收。
- 旧ResolveIdentityOperation及其DTO真实无调用，由readIdentityReceipt原事务完整安全回放替代，已删源码/合同/台账/两schema。operation_repository.go保留受信Store上下文与仍有真实调用的仓库前置并登记实际图。
- 本轮准确源码SHA为`8424db601aaf0ba822d98c79bf52f293451ebf93`（17个本域文件），完整四用例/相关参考助手已转verified；完整BuildApplication/main仍blocked，transport装配文件因缺其余handler/后台保持in_progress。没有本机编译/格式/生成/测试/运行SQL，也没有新增测试、fixture、mock、样例或临时验证程序，未push。

## 下一步（一项）

继续既有ReadPreferences/UpdatePreferences紧密组与真实HTTP映射：先读本域最新合同，沿用当前session权威、CAS和私有安全回执；受限初始口令会话仍不得读取/保存偏好。本工作包完成后冻结，由根分派后开展，不自行启动下一工作包。

---

# 本轮交接：完整登录注册与账号口令事务（2026-10-15，VPS现阶段检查通过）

## 第二轮最终集成与停止范围

- 登录/首次注册/子账号/改密/自身资料与认证HTTP核心通过；全域启动和未完成管理/恢复/后台仍blocked/in_progress。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


- 独占modules/identity，分支codex/identity-login-accounts-r1，从精确468986fc34e1c90b6b5ca575539fdd2b9748cd06新建，保留上一轮分支。本轮源码提交`491f3e049b98f01b85ee4e9bf69b6b3c147fadc9`（28文件，仅本域）；VPS格式/生成与依赖文件已回传，最终检查待根，准确SHA台账随后单独提交。状态以progress.json及实际Git为准。
- CreateSession：只接受portal签名LoginNetworkScopes，平台HMAC规范账号名，真实调用guard beginLoginAttempt/finishLoginAttempt；错密码/未知/停用统一拒绝，依赖失败或未知不发会话。authenticated核验事实在提交前确认，发送后不改报aborted。私有回执不重发secret，重新核权威凭据/状态后同事务签发。
- RegisterFirstSuperadmin：独立control.db不可逆reserved决定保存原op/caller/actor/HMAC/keyId/dataEpoch及固定账户/站点，只有全部匹配且同原epoch可续未决；active写唯一超管/凭据/会话/安全回执/outbox。active已提交安全回执可补记committed；恢复空库不重开注册，不宣称跨库原子。
- CreateAccount：当前唯一超管事务核验，权限只既有10项、默认全选，子账号初始口令强改；账户、凭据、功能、outbox和安全完整回执同事务。ChangePassword：校旧密码、凭据CAS、清强改、撤销其他设备并轮换当前设备秘密，安全回执仅引用，重复提示重新登录。
- ReadSelf从当前有效会话读取metadata并补credentialRevision，供改密expectedCredentialRevision；受限会话只允许静态必要自身资料。登录/注册nextRoute只从现有/admin/me（强改视图）、/admin/setup、/admin选择。
- 账号索引：本域账号互斥跨active提交到索引补齐；索引安装另有进程互斥。control journal先记identity.account-index决定，active提交后同步派生索引并完结；启动按active权威补齐，控制索引领先或未知决定失败关闭。
- 新实际文件：migrations/0004_private_operation_receipts.sql（active追加digest_key_id）、migrations/control/0001_bootstrap.sql（独立control schema1）。migrations.go只追加内嵌控制schema，active编号迁移不改写。根framework登记由集成会话完成。
- 公共依赖：platform.PrivateOperationDigest、OperationContext.LoginNetworkScopes与签名断言传递、guard beginLoginAttempt/finishLoginAttempt合同和生成客户端由根同步；HTTP公共机制已冻结并绑定已实现真实认证组；其余handler缺失时启动失败，不复制边界校验。公共依赖合成提交26138c1由根统一集成，本分支未自行cherry-pick。所有生成物只由VPS make gen/docs刷新；本轮gofmt/gen/docs和go生成的go.mod/go.sum已回传，最终build/check结论仍待根。
- 统一复核修订：匿名凭证回执/claim固定portal/system主体并清可选用户proof；维护仅核权/CSRF控制链只读白名单；索引统一16MiB且在active不可反悔提交前用共用编码检查，10000账户上限不变。真实认证组已绑定公共HTTP工厂，缺其余handler则整体启动失败。
- 已报告control路径P2已收口：仅调用根lifecycle.ValidatePrivatePath，目录建立/文件打开前拒绝dataRoot/control链中链接，control.db与WAL/SHM在打开前核正规文件，DB建立后再核；不扩新业务，待VPS最终检查。
- 最终源码补丁`8f8f515ba9d5bfd3d4d143ed23efd4c5c1fcac4a`已提交control路径修复和VPS生成物；主用例源码`491f3e049b98f01b85ee4e9bf69b6b3c147fadc9`，根统一合并后映射实际main SHA。
- 最终检查：集成会话修复快照在VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check全部通过（855文件/1012函数/374操作/2211类型），含control路径、匿名scope和16MiB索引修订。完成用例/助手转verified，部分未完文件仍in_progress，BuildApplication/main仍blocked；guard实际业务与portal cookie链的状态以各域台账为准，其他未完成handler/后台仍保留阻塞；这些能力和行为验收不得从本域编译通过推定完成。未本机编译/格式/测试/运行SQL，未新增测试/fixture/样例/临时业务程序，未运行真实服务/接口/模型，未部署或push。


## 下一步（一项）

本轮仅收口control.db路径链已报告边界、VPS最终make gen/docs/build/check及统一合并证据；根完成第二轮后按用户当次授权推送并清空目标，本域不展开第三轮或新增业务用例。

---

# 当前工作包：会话签发、刷新与即时授权（2026-10-15，VPS编译与结构复核通过）

## 持续实施第一轮最终集成事实（2026-10-15）

- 会话签发/刷新/CSRF/核权与撤销通过；完整登录注册与HTTP/后台仍未完成。
- 主线实际源码提交：8092ab4、9c86c83；模块分支交付经cherry-pick后SHA已在progress.json映射，后文原分支SHA保留用于溯源。
- 三域逐个合入后各自VPS make docs/gen、14模块readonly build/vet/gofmt与完整make check通过；最终850文件/943函数/372操作/2186类型，生成回传零变化。现有设计检查补核managementPermission与jobManagementGrants集合一致。
- 全部编译/格式/vet/生成检查均在VPS；没有新增测试，也没有运行业务测试、SQL行为验收、服务/真实接口/模型、镜像、部署或push。部分文件与入口继续in_progress/blocked，不把检查当业务验收。
- 下一项以本节和根plan最新工作包为准，后文“等待VPS”等旧说明为过程历史。


- 分支`codex/identity-session-complete-r1`；基线`325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，只拥有modules/identity。接续前代理相同工作包未提交撤销代码与追加迁移后逐项审阅；本轮源码提交`59070a0645a744e67426f511fcddd2040e46816f`，仅本域20文件；源码提交后工作树清洁，台账记录随后单独提交以引用准确SHA。
- IssueSessionTokens仅接受本域事务，由platform统一生成独立随机secret和专用Ed25519签名；固定issuer/audience/type，绑定会话/凭据/权限/认证/数据版本，15分钟访问且不超过首次签发后7天。原始cookie不入普通回执或日志，族、摘要与会话访问期限同事务。
- RefreshSession仅允许portal，入口建立固定Origin与Strict refresh cookie，不要求access有效。当前refresh一次消费；命中已消费摘要则同事务撤销族和session、登记安全outbox，提交后返回未认证。敏感响应丢失需重新登录，浏览器刷新singleflight。权限从本域权威功能表重建；刷新允许旧permissionRevision，同一事务CAS写入最新权限与session版本后签发；旧access/委托仍失败关闭，不放宽凭据/认证/数据代次或撤销。
- CheckAuthorization以互斥的portal access或可信委托解析主体，读取一致的账户/会话/族/auth_state快照。access同时验签与核对摘要及当前水位；委托严格匹配op.actor主体、站点、session、permissionRevision、正数sessionRevision。即时撤销不依赖异步事件。
- 受限初始口令AuthContext仅含account.self；还必须由可信静态action精确选择identity:changePassword、identity:revokeSession或identity:readSelf，缺失或其他动作失败关闭，不能扩充grants。ReadSessions拒绝受限主体，受限登出只能撤销当前设备。
- VerifySessionCSRF限定portal与固定公开Origin，验签access与CSRF独立secret摘要同一会话；可选sessionId若提供必须匹配。RevokeSession完整安全回执、撤销和outbox同事务，先查回执允许原结果重放。RevokeSessionFamily只允许私有事务；RevokeOtherSessions只允许改密事务，撤销其他族/session并CAS更新保留设备凭据水位。
- RevokeAllSessions同步撤销refresh族。InvalidateRestoredSessions将control决定NewAuthEpoch同事务写auth_state并全撤session/族；done重放核对认证代次一致。追加0003迁移不为旧会话生成可用凭据。
- 统一复核追加任务入口粗能力：MapFunctionGrants只读gen.JobManagementGrants，派生grants含任一登记项即追加job.manage，不新增用户开关；jobs按持久owner/type二次核验具体权限。受限初始口令/非active仍由sessionAuthContext清空或收窄grants。追加源码`2ee5c9395297238ab63c921b688d28e221b26b01`，根目录和生成器已同步，VPS最终检查通过。
- 本域契约：AuthorizationRequest新增accessToken与委托互斥来源、可选静态action；CSRF必需accessToken、sessionId可选；IdentityChanged.reason增加session；HTTP错误补完整性/CAS实际分支；登记运行配置和一致快照助手。没有新增业务端点。
- 公共协议由集成会话提供：platform.SessionAccessClaims/SessionSecrets、serviceauth.SignSessionAccess/VerifySessionAccess/GenerateSessionSecrets、公开Origin配置、AuthContext.SessionRevision与严格委托版本传递。本域未修改公共代码。根framework需登记实际出现的authorization_service.go、csrf_service.go和0003_session_tokens.sql。
- VPS最终快照make docs/gen、14模块make build（readonly build/vet/gofmt）及完整make check通过（850文件/943函数/372操作/2186类型）；交付函数和完整文件转verified，含未完成注册/登录/outbox循环的文件仍in_progress，行为验收未执行，不以源码存在当完成。未新增测试、样例、fixture或临时验证程序，未运行真实认证服务/接口/模型，未部署或push。业务验收未执行。

## 下一步（一项）

本轮VPS检查已通过，由集成会话统一合并并映射实际主分支SHA。下一工作包先收口CreateSession的traffic-guard可信来源、失败保护和初始化下一路由，再实现登录完整写事务与固定handler装配；不猜协议直接开放登录。

---

# 本轮交接（R1-01 自身会话分页与账户全部会话撤销，2026-10-15）

## 本轮统一复核结果（2026-10-15）

- 三个模块在独立worktree交付，集成分支逐个审查新增逻辑与合同；最终隔离VPS linux/amd64 go1.25.5、GOMAXPROCS=8 的14模块make build和完整make check通过。编译、结构检查不代表业务验收。
- make docs无漂移；make gen第二次运行零变化，核对发生在用户追加VPS专用要求之前；后续所有编译/测试只在VPS，VPS不可用则blocked，不自行本机回退。本轮没有新增或运行业务测试、真实接口、模型、服务或部署。
- 根framework已登记本轮10个实际文件，生成器与消费者统一刷新；本轮已完成函数和完整文件按证据转verified，部分文件与入口仍如实in_progress/blocked。主分支实际实现SHA以progress.json为准。

- 分支 `codex/identity-session-r1`，基线 `00dd199239aba487bceb5227aac52f27ce949d74`；独占 `modules/identity/`。公共依赖由集成会话提供：nullable生成修复 `7e5aa58`、平台游标签名 `1daa621`，本分支仅cherry-pick既有提交，未自行修改公共代码。源码提交 `b716d5a2fa613d0cfbb11815761e3d4f09034f0c`。
- `ReadSessions` 固定查询当前admin主体的site/account，按issuedAt/sessionId升序键集分页，默认20、最多100；平台 `SignCursor/VerifyCursor` 负责签名，签名scope绑定identity.sessions/site/account/dataEpoch，载荷仅含排序位置。只返回明确列出的会话元数据，revokedAt保留null，不返回secret/摘要/IP；`WithSessionCursorKey`由后续HTTP装配注入可信SigningKey。
- `RevokeAllSessions` 对自有事务核验superadmin，对嵌入调用只接受进程内`WithIdentityTransaction`句柄；按站点/账户读取当前权威版本，原子写revokedAt、提升session.revision、追加平台outbox。自有事务将动作/caller/epoch/请求摘要和完整计数回执一并提交，幂等重放返回原计数，不撤销首次提交后新建的会话；外层私有事务拥有最终回执与提交/回滚。
- `PublishIdentityEvent`已审计全部实际调用方：此前没有外部事务调用方，本轮新增RevokeAllSessions。外部事务分支只入outbox，不再抢占外层operationId回执；自有事务保留原事件输入摘要、queued结果哈希及同eventId去重模式，未改变该模式的请求语义。
- `0002_session_revocation.sql`只追加operation_receipts.result_json与自身列表索引，不改写旧会话，不预埋未确认的家族/authEpoch/签发字段。旧回执缺完整结果时不得猜测恢复。
- ReadSessions的OpenAPI补503/422，并将原任意字符串limit修正为types.json一致的整数1..100、默认20。`make gen`生成本域Go接口；函数/文件/API文档由`make docs`重生。

## 本轮检查与交付限制

- identity本域 `GOFLAGS=-mod=readonly go build ./...`、`go vet ./...`、`gofmt -l`为空与`git diff --check`通过。首次基线全量`make build`通过；合入公共nullable修复后的最终全量构建在content旧实现的PreviousRevisionId指针适配处失败，集成会话已认领，未跨域修复。
- `make gen`、`make docs`成功。`make check`仅报根framework未登记本轮3个实际文件：internal/session_service.go、internal/session_revocation.go、migrations/0002_session_revocation.sql；公共登记交集成会话处理。
- 新函数暂记blocked（待集成全量编译），不是业务验收通过；部分实现文件保持in_progress。未新增测试、样例、fixture、临时验证程序、真实接口/模型、服务进程、部署或push。
- 未实现CreateSession/IssueSessionTokens/RefreshSession/单会话或家族撤销/其他设备撤销/CSRF/HTTP接线；没有未登记的ValidateSession。CreateSession仍缺可信登录风控上下文与初始化下一路由字段；签发固定issuer/audience、验签入口、认证代次及refresh家族规则须先由集成会话统一收口，不能猜协议。

## 下一步（一项）

先收口`internal/session_service.go`中`IssueSessionTokens`的会话协议：对照本域types/implementation、公共protocols与portal授权解析入口，确认固定issuer/audience、access验签入口、refresh单次轮换与auth epoch权威来源；先同步合同与调用方，再实施签发，不能扩展HTTP或以未签名令牌代替。

# 身份权限会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- 运行状态适配解开 `state` 外层。
- `MapFunctionGrants` 改读 make gen 由根 `contracts/permission-groups.json` 生成的 `internal/gen/permission_groups.gen.go`，不再维护手写副本；唯一超管取全部登记功能组。
- 账号索引权威水位改为 Σ(revision+1)（账户只做墓碑、revision 只增），新建 revision=1 子账号不再与旧索引同水位冲突；索引文件尚不存在时由权威表首次建立。
- gofmt 整理 8 个文件。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；新增生成表 `internal/gen/permission_groups.gen.go` 按生成文件约定记 `planned`（由 make gen 重建）。
- 遗留：恢复到较旧快照后 control 区账号索引会领先权威表，目前按完整性错误拒绝；换回/恢复流程中如何重建索引需在恢复工作包中确定。

## 本次事实（R1-01 账号索引、注册前置、幂等与恢复安全边界，2026-10-15）

- 当前集成分支：`codex/analytics-r1-storage-contract-audit`；本轮从 `7aa44567609713b8810b9b8ca9ba61579648d55a` 接续。源码切片提交：账号索引 `138a2b45ae088431e25b0a3694d453f51c49a6f4`、注册/权限前置 `484aa7fb79d5d12e8ff1c5bb2b22ed83946e7d55`、幂等/事件/恢复安全 `1a6725d3eca0ba4674b1bbefa626c97ed14eb0c4`；未 push、未部署。
- 独占范围仍仅为 `modules/identity/`。本轮新增 `internal/server_recovery.go`、`internal/registration_service.go`、`internal/account_service.go`、`internal/permission_mapping.go`、`internal/operation_repository.go`、`internal/outbox_repository.go`、`internal/restore_security.go`，并扩展 `cmd/identity/main.go`；未修改 contracts、生成目录、根文件或其他模块。合并后补充的 Store 上下文冲突修复和损坏索引拒绝覆盖修复已随台账提交 `1ee1458` 落地。

## 本次完成

- `ReadServerAccountIndex`/`WriteAccountIndex`：读取或原子写入 `control/account-index.json`，校验私有目录/文件权限、UTF-8、未知字段、账号名/状态/唯一超管、revision 单调性；索引只含账号元数据，不含凭据、会话或口令哈希。`accounts` 子命令已读取该索引并输出账号名与状态，不启动 HTTP。
- `ReadRegistrationState`：读取本域账户权威表并调用已注入的 site-config 初始化状态边界；无账户才开放注册，已有账户返回站点标识。
- `MapFunctionGrants`：按已确认的十项功能组、authenticated 基础权限和 superadmin-only 权限做确定性排序映射，拒绝未知/重复功能。
- `ResolveIdentityOperation`：按 site/actor/operation 唯一键读取状态与结果哈希，不返回敏感输入。
- `PublishIdentityEvent`：在业务事务中登记固定身份事件信封、outbox 与幂等回执，投递仍必须由后续 `RelayOutbox` 经 jobs 完成。
- `InvalidateRestoredSessions`：要求启动编排显式注入已验证 `restoreId`，以 control journal 的 open/done 决定和事务批量撤销恢复数据中的会话；尚未接入 `BuildApplication`。
- 注册与账号创建目前只实现输入、授权、依赖和权限前置校验；未返回假成功，完整写事务仍保持阻塞。

## 检查与阻塞

- `git diff --check` 通过；本机无 Go、gofmt、make，未运行 `go build ./...`、`go vet ./...`、`gofmt -l` 或 `make check`，新增实现全部保持 `verification.not_run`。
- 合并时发现两个并行切片重复定义 `WithIdentityStore`/`identityStoreFromContext`，已在 identity 内统一为 operation repository 提供的受信 Store 上下文；该修复必须在下一提交中保留。
- `BuildApplication` 仍缺完整 OpenAPI x-handler 绑定、site-config `registerActivation` 回执、恢复后 `restoreId` 注入与 `InvalidateRestoredSessions` 调用、`ObserveSuperadminResetFile` watcher、`RelayOutbox` jobs 客户端/后台循环；因此 main 只能解除 `accounts` 子命令阻塞，不能标记整体完成。
- `ChangePassword`/会话/CSRF 代理审计确认当前 sessions 表缺少 refresh family、access/refresh/CSRF 摘要字段，且签发/撤销/幂等回执与 Store 注入未齐；未猜实现，相关函数仍未完成。
- 根 `docs/structure/framework.json` 尚未登记本轮新增实际文件；按并行规则不在本模块会话修改，交集成会话处理。未新增测试、样例、真实接口或模型调用。

## 下一步（一项）

先在具备 Go 工具链的 VPS 或集成环境编译/gofmt/vet 本轮 identity 源码，修正真实编译问题后，再实现并接入 `BuildApplication` 的完整 handler 装配与恢复回执；不要把结构检查或源码存在视为业务验收。

## 本次事实（R1-01 超管不可变纯规则，2026-10-15）

- 分支：`codex/identity-r1-05-superadmin`；基线 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`；源码提交 `7a059d4e95f8043808cecc21537afc5a18f38a6a`。未 push、未部署。
- 独占范围：仅 `modules/identity/`；本次实际修改 `internal/account_validator.go`、`STATUS.md`、`progress.json`、`docs/handoff.md`。未修改 contracts、生成目录、根文件、其他模块或公共库。
- `identity.AssertSuperadminImmutable` 已实现为纯规则函数：输入状态枚举先校验；`isSuperadmin=true` 仅允许当前与目标状态均为 `active`；非超管按 `active↔disabled→deleted` 状态机判断；不从 `permissions` 推断超管身份；无持久化、网络、会话或HTTP副作用。
- 状态口径：源码与台账为 `implemented`；因本环境无 Go/gofmt，验证为 `not_run`；M02-R01 业务验收为未执行、未接受。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用。

## 本次检查与阻塞

- `git diff --check` 与提交前暂存差异检查已通过。
- Go、gofmt、make 在当前环境不可用，未运行 `go build ./...`、`go vet ./...`、`gofmt -l` 或 `make check`；不可将本次实现标为 verified。
- 根 `docs/structure/framework.json` 与 `THIRD_PARTY_NOTICES.md` 的既有公共登记缺口仍由集成会话处理；本模块会话未修改公共文件。

## 本次事实（R1-01 纯规则函数组，2026-10-15）

- 分支：`codex/identity-r1-04-pure-rules`（managed worktree）；基线 `ac593c57861d2242f27d07562a2acae535d18c38`；源码与依赖提交 `bdcb9128d3ec5983d54c57f0fddc7abb793e93f4`。无未提交文件、未 push、未部署。
- 独占范围：仅 `modules/identity/`；新增/修改 `internal/account_validator.go`、`internal/credential_service.go`、`go.mod`、`go.sum`。未改 contracts、生成目录、根文件、其他模块或公共库。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用；业务验收仍未执行。

## 本次完成

- `identity.NormalizeAccountName`：先验证 UTF-8，再做 NFC；拒绝空值、控制字符、首尾 Unicode 空白和超过 64 个 Unicode 字符；不 trim、不大小写折叠、不回显原值。
- `identity.HashPassword`：使用 `x/crypto/argon2` 的 Argon2id v19，固定 `memory=65536 KiB`、`time=3`、`parallelism=4`、`hash=32`、CSPRNG 16 字节盐；输出 PHC 编码，受限并发，检查 context/operation deadline，清理敏感缓冲。
- `identity.VerifyPassword`：解析并限制 PHC Argon2id 成本不超过当前上限，派生后常量时间比对；仅对有效口令返回 `needsRehash`，损坏或越界编码返回 `INTEGRITY_ERROR`。
- `identity.ValidatePasswordChange`：按 schema 验证旧/新口令长度与 UTF-8，要求两次新口令完全相同且与旧口令不同；语义不满足返回 `valid=false`，不记录或回显口令。
- `go.mod` 新增固定版本 `golang.org/x/crypto v0.28.0`、`golang.org/x/text v0.20.0`；两者许可证核对为 BSD-3-Clause。根 `THIRD_PARTY_NOTICES.md` 尚未登记，交集成会话补登记。

## 检查证据

- 基线与提交前 `git status --short --branch`、`git diff --cached --check`、作者/邮箱和暂存隐私扫描通过；提交内容仅在 identity 模块内。
- `git diff --check` 通过。当前环境没有 Go 工具链、gofmt 或 make，未执行 `go build ./...`、`go vet ./...`、`gofmt -l`；不可把源码实现标为 verified。
- `python scripts/check_docs.py` 已运行但因根 `docs/structure/framework.json` 尚未登记两个新增源码文件而失败；`make check` 因 make 不可用未执行。根 framework 与第三方声明是公共文件，按并行规则交集成会话处理。
- 无行为测试或业务验收证据；M02-R01～R06 仍未执行。

## 依赖与边界

- 本次只落地纯规则函数；会话、账号 CRUD、HTTP 路由、后台循环、`AssertSuperadminImmutable`、`ChangePassword` 均未实现。
- 公共登记缺口：根 `THIRD_PARTY_NOTICES.md` 需登记 x/crypto 与 x/text；根 `docs/structure/framework.json` 需登记两个新源码文件。未在本会话修改公共文件。
- 依赖版本已固定在 identity module；Go 构建需在具备 Go 1.25 工具链的 VPS 或集成环境复核。

## 本次事实（R1-01 私有持久化，2026-10-14）

- 分支：codex/identity-r1-01-storage（worktree `../nice-wiki-identity`）；基线 bbb1e535c2f1（main）；实现提交 28af316bf6a42a2378f30f17a2fe036380af4ff9；台账提交紧随其后。
- 独占范围：modules/identity/。公共文件仅在交付提交中同步 docs/structure/framework.json（+3 文件）、THIRD_PARTY_NOTICES.md（modernc.org/sqlite v1.59.0），请集成会话复核。
- 未提交文件：无。未新增测试/样例/真实接口调用；未 push、未部署。
- 沿用说明：migrations/migrations.go 与 0001_domain.sql 初稿来自并行子代理（因用量上限中断），本会话核对其符合 data.md/ADR0005 后沿用并补齐存储层、装配接线与台账。

## 本次完成

- internal/storage.go：identity.OpenDatabase（WAL/foreign_keys/busy_timeout=3000ms、单连接串行、control 区决定日志 0600 追加句柄）、identity.RunMigrations（migrations.All 校验编号连续，事务执行并登记 schema_migrations，集外版本 INTEGRITY_ERROR）、identity.RecoverJournal（逐行末行状态生效，open 条目隔离）；AppendDecision/CompleteDecision/HasSuperadmin/Close 为未登记参考级助手，随首个调用方交付时登记。
- BuildApplication 在 ApplyRestoreSwap 后打开业务库并执行迁移与恢复；Ready 补 initialized（唯一超管存在）/journalRecovered/activeDataEpoch（打开时代次，运行代次不一致 503）；Shutdown 在 HTTP 关闭后关闭业务库。
- 合同同步：implementation.json 增 internal/storage.go、migrations/migrations.go（函数 identity.All）与 0001_domain.sql（migration 类），BuildApplication 依赖补三个存储函数；make docs 已重生成 files.md/functions.md；progress.json contractSha256 已重算并新增 4 函数/3 文件行。

## 实现口径（供后续与集成对齐）

- 业务库 active/identity.db，决定日志 control/identity-journal.jsonl（control 区不随恢复替换）。0001 权威表按 types.json 公开 DTO 建列；sessions 的 refresh 族/令牌哈希列不在 0001，随会话工作包追加 0002（迁移只追加）。
- 平台 nw_outbox/nw_inbox 由 EnsureSchema 创建（outbox 工作包接入时），不进本域迁移。
- 换向后回执、InvalidateRestoredSessions、x-handler 绑定与后台循环仍 blocked，缺口见 progress.json；site-config 控制库（R1-02 下一包）就绪前运行权威读取等待依赖。

## 检查证据

- 本地 darwin/arm64 go1.27.1 与隔离 VPS（8核，go1.25.5，/root/build/nice-wiki-identity-persistence，tar 同步排除 .git/__pycache__/AppleDouble）：`go build ./...`、`go vet ./...`、`gofmt -l` 为空均通过；根 `make check` 均通过（829文件/894函数/372操作一致）。
- 未运行业务进程与业务验收（site-config 未实现，启动按合同 DEPENDENCY_UNAVAILABLE；无已授权测试）。

## 下一步（一项）

实现 `modules/identity/internal/account_validator.go` 的 `identity.AssertSuperadminImmutable`：先读 `docs/functions.md`、`types.json` 和 M02-R01 条款，保持纯函数边界，完成后仅更新本域台账/STATUS/交接并在具备 Go 工具链的环境执行既有检查；不得顺手实现账号状态接口或会话逻辑。
