# 公共协议

## HTTP与类型

服务内部API采用OpenAPI 3.1和JSON Schema Draft2020-12；每域唯一来源在contracts/openapi.json和types.json。请求用JSON，文件用二进制/分段，长结果用SSE。JSON不接受未声明字段，required、null、范围和枚举按schema；未知版本明确拒绝。公共类型在 [common.schema.json](../contracts/common.schema.json)，操作权限与错误码各有全局登记。

每operation明确operationId、x-handler函数ID、x-permission、请求、成功及失败响应；有路径变量必须声明参数。响应不把内部堆栈、密钥、路径或提问内容混进Problem.detail。问题响应采用application/problem+json，字段参照 [RFC9457](https://www.rfc-editor.org/rfc/rfc9457)。协议来源参照 [OpenAPI3.1](https://spec.openapis.org/oas/v3.1.0)。

## 身份与上下文

服务凭证确定调用服务，用户委托确定业务主体与权限。OperationContext含requestId、operationId、rootOperationId、callerService、actor、deadline、dataEpoch；付费调用加可信costClass和取消标识。公开客户端不能指定费用类别、匿名网络作用域或后台身份。grants不含资源可见性的推测：每域仍按资源归属执行检查。

身份令牌只接受固定发行方、固定签名算法、audience、有效期及可撤销版本；拒绝客户端选择算法。具体密钥产生/轮换见identity函数合同；JWT字段含义参照 [RFC7519](https://www.rfc-editor.org/rfc/rfc7519)，不复制第三方实现。


会话访问令牌统一由platform.serviceauth签发和验证：固定EdDSA、typ=NW-SESSION、iss=identity、aud=portal，iat/exp为UNIX秒且最多900秒。声明携带session/account/site、credential/permission/session版本、authEpoch、dataEpoch和jti；签名有效之后，identity仍同步读取账户/会话/族/认证代次权威与秘密摘要。refresh和CSRF各自32随机字节，持久化只保存摘要；refresh旧值复用须先持久撤销族，再返回未认证。AuthContext的sessionRevision是会话单调版本，portal委托SessionVersion必须取此值，不以permissionRevision替代。

portal使用identity:checkAuthorization的accessToken模式，由静态路由提供permission/action；其他域使用匹配可信委托的subject/site/session模式，两个来源不能混用。初始口令会话只允许identity:changePassword、identity:revokeSession和identity:readSelf；缺动作也拒绝。账户权限写请求检查CSRF；匿名凭证登录/刷新仍受部署固定Origin和Strict cookie约束，不要求尚未过期的access。可信公共地址只取进程配置publicBaseURL，不从请求Host反推；HTTP仅在显式allowLocalHTTP且loopback本地开发时允许。

## 时间、版本、错误与幂等

机器时间统一RFC3339 UTC；界面转换用户时区。钱统一USD整数micros，priceVersion随费用记录，禁止浮点累计。页大小默认20、上限100，游标签名且绑定过滤条件。写入含expectedVersion或expectedRevision；锁带lease与单调fencingToken，过期锁不能覆盖后继锁。

幂等键绑定调用方、业务操作、请求体哈希和dataEpoch；同键不同请求409，同键重放返回原operation，不能再次调模型。网络超时不证明供应方未接收；供应方attempt与业务operation分别记录。错误码/HTTP映射见 [errors.json](../contracts/errors.json)。

## SSE与取消

先accepted(operation/call ID)，再progress、delta、citation，终态completed或error。序号单调，重连从持久事件水位读取；已输出文本后不将备用模型结果拼成同一回答。心跳不作为业务进度。取消向子任务传播，停止未开始的付费步骤，已开始调用用独立有限结算上下文记录费用。

## 事件与公共管理

事件只经jobs中转（ADR0002）：生产方在业务事务内写outbox，本域转发循环在提交后调用jobs:publishEvent，确认后才标relayed；jobs按各域events.json consume条目生成的订阅目录建立投递；消费方订阅循环以durable模式调用jobs:subscribeEvents，按handler字段调用本域处理函数，本地事务提交后才jobs:acknowledgeEvent；未确认投递在确认租约到期后重投，超限转死信。portal浏览器转发使用ephemeral订阅，不建投递。

EventEnvelope必须符合生产方events.json；至少一次持久投递，按对象revision拒绝旧事件，按eventId去重，dataEpoch不匹配拒写。事件传必要元数据与不可变ObjectRef，不任意抓客户端提供URL。

有状态域的管理API为barriers和snapshots export/stage/discard（ADR0003）；request/response使用公共schema。入口无业务存储只参加维护，不虚构快照。所有管理路径受服务调用方、超管委托、屏障以及dataEpoch共同限制。

## 服务身份与委托（[ADR0004](adr/0004-wiring-and-service-identity.md)）

服务之间的每个请求携带服务断言：调用方用部署时登记的 Ed25519 私钥签发≤60秒的 JWS（iss=调用服务、aud=目标服务、method、path、bodyHash、jti），接收方按公钥表验证并拒绝重放。用户请求由 portal 签发≤10秒的委托证明（X-Portal-Proof：actor、grants、sessionVersion、route、bodyHash），下游链路原样传递；接收方仍经 identity:checkAuthorization 核实权限与会话版本。访客只持 traffic-guard 签署的伪名会话。全部 OpenAPI 使用同一组安全方案：serviceAssertion、userDelegation、visitorSession、admissionPermit；portal 与 identity 另有 sessionCookie、csrfToken。签发与验证只在 libs/platform 实现。

## 公开 AI 准入

OpenAPI 用 `x-admission` 标记公开 AI 接口。portal 对这类请求用可信代理链得到的访客地址（只在内存）调用 traffic-guard:createAdmission，通过后附 X-Admission-Permit 转发；拒绝时返回 429/403 与 retryAfter 或挑战要求。qa 消费许可，长回答每10秒续租，排队轮次出队时换取活动许可；网关本地验签，不回查 guard。

## 模型调用

createInference 只返回 202 受理。调用方一律经本域 InvokeModel（libs/platform 实现）按序号读取 resumeInference 直到终态，取消或截止时 cancelInference；失败与备用重试由网关负责，调用方不换模型、不重放。

## 任务与后台登记

每个 implementation.json 登记 `jobTypes`（类型、执行函数、触发、可选系统定时）与 `backgroundTasks`（函数、运行时机）。RunWorker 只领取登记类型并调用对应执行函数；BuildApplication 启动全部后台函数，并以服务身份经 jobs:createSchedule 幂等登记系统定时。jobs 的类型白名单、事件订阅目录与 portal 路由表由全部合同生成。

任务管理的入口能力按permission-groups.jobManagementGrants派生job.manage，不新增用户开关；具体任务由jobs按持久type/owner的managementPermission再次向identity核最新权限，系统聚合任务要求system.manage。用户提交的owner或job.manage不能替代这一判断。任务控制调用使用独立子operationId，同一传输重试复用，费用rootOperationId和executor原业务操作不变。

portal配置delegationKeyRef（NW_DELEGATION_KEY_REF）单独引用已登记portalDelegationKeys的委托私钥；signingKeyRef仅用于服务断言。委托严格十秒内且过期即拒绝。代理不跟随上游重定向，上游请求取消句柄保留到响应Body读完关闭；IPv6和默认端口按规范化Origin比较。

委托链区分来源：先验本跳服务断言（当前方法/path/bodyHash），再验证原portal proof的签名、短期时效与会话版本。已认证issuer为portal时必须核原bodyHash；后续服务调用仍原样保留原proof，衍生请求体由该跳服务断言绑定，各域按原主体再次调用identity并核本域资源。不能从浏览器头选择此模式，也不能把后续服务证明当新用户授权。

portal浏览器认证适配：nw_sid和nw_refresh均host-only、Path=/、Secure、HttpOnly、SameSite=Strict。access使用identity返回的expiresAt，refresh为浏览器session cookie，权威绝对期限由identity控制。浏览器刷新输入严格空对象，portal只从refresh cookie构造内部请求并重新绑定服务断言正文；浏览器不能提交refreshToken。认证成功只返回安全session（sessionId/auth/mustChangePassword/expiresAt/csrfToken）及明确account/nextRoute，剥离内部tokens/currentSessionTokens及access/refresh明文；CSRF仅响应供内存使用，所有认证响应禁缓存。内部失败或敏感回执重放不生成新cookie，也不盲清已有cookie。

identity:checkAuthorization的AuthorizationDecision同时返回当前已校验账户的安全accountName，来源为同一权威快照的不可变账号名；content只用它显示当前编辑者，不能从请求正文或委托头自报显示名，也不能据此增加权限。

jobs:createJob可携可选notBefore指定最早领取时刻，显式值固定纳入去重；省略保留原立即可领和旧摘要。创建RPC短deadline只管提交传输，排队后由所属服务领取时建立执行期限，不能把原短请求截止当成延迟任务到期。


服务断言可带签名siteId：出站只从可信OperationContext.Actor.SiteID复制，不取HTTP站点头。无用户委托时接收方保留签名站点及纯服务身份；旧断言省略siteId继续为system。有用户委托且断言带siteId时两者必须相同，否则FORBIDDEN。该范围不代表权限，拥有服务仍核对象归属和当前授权。主动切纯服务时清除用户proof并建立服务actor，保留已核本站/epoch/root/费用类和更早截止；传输层同时删除旧proof头。

worker领取为队列控制：平台每次从当前runtime建立owner纯服务system、epoch及独立有界控制op；领取后由域从持久批准恢复实际site和执行权限，再发送真实任务对象RPC。公共WorkerJobsClient只要求六租约端口，HTTP映射与生产方schema校验集中worker.NewJobsClient，各域只装配客户端和六Result校验器。

OpenAPI顶级x-runtime-payload-schemas显式列出后台或客户端需要的生产方载荷组件别名；make gen把这些schema及引用闭包写入clients.gen.go的PayloadValidationSpec，用现有NewPayloadValidator严格校验。它不增加HTTP路径，也不把未使用的内部DTO全部导出，解决oapi-codegen默认剪除未被HTTP引用组件的限制。生成物只由GitLab生成。

## 问答许可分配与消费（2026-10-18）

guard createAdmission只分配持久元数据，token为空串。qa付费提问消费时核同owner、原根、站点、代次、有效租约和ban，持久提交消费事实后才签发nw-admission-v1。只有该非空签名可进入MG的qa费用分支；MG本地验签，requestId固定等于原问题RootOperationID，保留原匿名subject，二跳服务不能替换主体或费用scope。续租不换根、不重扣桶、不复活到期许可。localization.suggest仅免费限流，永不签模型token。

需人机验证时返回CHALLENGE_REQUIRED/429且retryable=false，由调用方沿既有issueChallenge领取挑战，不在Problem中藏ID。纯频率为RATE_LIMITED，ban按原FORBIDDEN。挑战通过的十分钟session证明使用独立用途，不能充当模型许可。

创建会话使用原有 `qa.conversation` 人机准入：分配、消费与核验都保留空模型 token，持久创建或原回执返回后幂等释放并发占位。它不调用模型；只有 `qa.ask`、`qa.followup` 已消费许可可签发模型凭证，不能把会话许可换成提问许可。

风险信号响应只有实际禁令存在时才返回可选 `banExpiresAt` 时间；allow/challenge 无禁令时省略，禁止空时间或 null。

## 域快照归档版本1（2026-10-18）

公共 `DomainSnapshotManifest` 为未压缩tar首entry `manifest.json` 的唯一来源，canonical JSON上限16MiB；仅允许声明的 `files/<relativePath>` 普通文件，拒绝链接/特殊节点、重复、路径遍历及未声明内容。总归档20GiB、最多100000文件。域在真实安全截点回调穷尽files、objects、publications与跨域references，未知不能用空数组冒充完整。目标方逐项核长度/哈希/原epoch/格式版本与EOF后暂存，目标epoch只由域迁移回调建立；旧正文块ID和付费结果不重新生成。

## 后台监督与真实租约镜像

TaskGroup只监督域已持久接受的本进程操作，Start唯一key去重并固定原业务deadline，HTTP断开不改deadline或取消接受任务；进程停止/显式Cancel/原deadline仍取消IO，关闭拒绝新任务并等所有回调退出。域回调负责终态/费用，监督集合不重派模型。WorkerConfig.OnLeaseRenewed在Start后的真实Heartbeat及每次权威续约后执行，只做本域同worker/fence/epoch镜像CAS，错误停止执行，禁止递归CheckLease或另造心跳。ClaimedJob.WorkerID由平台本轮领取者填入。RuntimeView.revision/barrierId来自真实site-config控制库，Ready不伪造水位。

生成客户端对二进制成功响应返回带原Content-Type/Length/ETag的http.Response；调用方有界读、核引用并Close，所有拥有者客户端拒绝重定向，不把归档当JSON或跟随任意地址。AllowedGitHosts只由data-protection可信部署配置提供，默认空拒Git远程，不从请求或服务依赖推导。

## 受控解析子进程

平台lifecycle.RunBoundedParser只执行当前二进制固定`--nw-isolated-parser=<kind>`入口，清空继承配置/密钥环境，管道输入最多50MiB、输出16MiB、最早父ctx或60秒，超输出立即取消杀进程。域main在读配置/密钥/数据库前调用RunParserChild；回调仅处理内存。Linux amd64先设CPU30秒、数据段512MiB、GOMEMLIMIT384MiB及禁core/文件输出，再用NO_NEW_PRIVS与seccomp TSYNC封闭允许集；拒绝文件打开、网络、新程序、非线程clone、io_uring、跨进程读取和提升资源限额。其他架构明确不可用，不降级在主进程解析。知识域仍负责PDF50MiB/500页/8MiB文本等业务限制；编译不是隔离行为验收。

## Inbox冲突隔离提交

新处理链必须把ClaimInboxBeforeWrite放在事务首项写入，提供只提交该认领事务的回调；同ID身份/载荷冲突时，先提交隔离再返回完整性错误，不能让外层defer Rollback擦掉隔离。已隔离事件即使旧hash再次投递也不确认。修订水位按producer/eventType/eventVersion/subject隔离，只比较已applied；并发唯一冲突未经核对不确认。旧ClaimInbox保留兼容签名但不承诺替调用方提交隔离。

数据保护部署可配置GitCredentialRefs（最多64个Identifier到规范绝对私有路径），其他域不得配置，加载只校验引用不读秘密。HTTP credentialRef只能选择已配置ID，不能拿请求路径读取私有文件。浏览器API端口仅为恢复只读/恢复取消许可/备份下载提供固定命名token选项，portal只能映射对应data-protection精确操作头，禁止任意headers或URL携token。responseType=stream返回原Response供有界管道下载，不能把20GiB归档统一读Blob。
