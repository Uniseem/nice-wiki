# 当前状态（2026-10-15，R1并行工作包合并与集成复核修复）

阶段：R1实施中；本轮两轮并行业务切片已合入main并经集成复核修复，14个Go模块编译/vet/gofmt与结构检查通过；行为验收和部署尚未完成。

- old归档提交272bddc保存全部旧代码、旧文档及会话记录；旧server/web/api/deploy相对原main的blob未改变。main当前树只保留新框架、许可和产品需求快照，不复用旧业务实现。
- 站点功能及细节按old产品基线保留；分成15个独立服务加2个共享组件（libs/platform、deploy），当前实际结构检查统计843个文件、905个函数、372个HTTP操作、2161个类型定义（含公共类型引用和内部类型）；约束/参考数量见各域files.md。已实现条目仍须以模块progress.json和检查证据为准，不能视为业务验收通过。
- 每域有总索引、架构/数据/API/流程/文件/函数/运维/验收文档、四份机器契约，以及STATUS/progress/handoff。独立启动、健康/就绪、构建部署和UI装配条目已登记。
- 多会话按独立worktree和模块所有权开发；公共契约由集成会话统一修改。2026-10-15按用户授权并行启动多轮模块工作包，分别记录了基线、独占目录、提交和阻塞；根公共结构登记由本集成会话统一完成。
- 隔离VPS执行 `make check` 已通过：JSON/引用、文档文件链接、15模块资料、函数/类型/HTTP/权限/依赖、唯一事件生产方与载荷、结构清单、交接台账和隐私。未运行新业务编译、业务测试、E2E、真实模型或部署；检查不证明业务行为正确。
- 规格修订（ADR0002，用户授权）：事件统一经jobs拉取投递——9个消费域补订阅循环并在events.json登记handler，identity/site-config/content/media补outbox转发，jobs推送投递改为到期重投与死信，删除analytics推送入口和public-site两个无人生产也无人消费的事件，portal访问审计改调analytics:appendAudit。Go操作函数统一(ctx context.Context, op OperationContext)，浏览器函数统一(input, signal)；函数ID统一为“模块.函数名”；依赖全部可解析，非函数依赖移入resources；函数分约束/参考两级；functions.md/files.md改由make docs生成。本地make check通过（新增依赖、签名、事件链与生成文档校验）；未在VPS复跑，未编译或运行业务。
- 恢复简化（ADR0003，用户确认R0-07）：删除各域在线validate/prepare/commit/rollback共56个接口、专用恢复许可与单域激活；改为各域stage暂存→data-protection写切换决定→各服务由进程守护重启并在启动时换入active→全部回执后解除维护，失败可整体换回；恢复入口仍在初始化向导与后台。部署须配置自动重启。各域api.md接口清单改由make docs从OpenAPI生成。本地make check通过。
- 接线与工程（ADR0004、ADR0005，用户授权“按你的想法做好”）：逐条追踪关键流程并计算可达性，补齐编排函数、任务与后台登记、portal入口准入、模型结果读取与统一服务断言，除界面外函数全部可达；补齐old后台功能所需接口（估价、问答记录、可疑事件、任务队列、翻译进度与对照编辑、配置读取）；定时备份授权集中在data-protection；新增平台公共库与部署编排合同，统一生成代码登记，确定依赖许可、端口与内存预算；frontend.md列出后台页面与接口对照；R1拆为17个工作包。本地make check通过。
- Git作者使用用户已授权的既有GitHub代用身份，只修改本仓库配置。私有vps.txt被忽略且未同步；旧线上实例与数据未改变。
- 用户已明确授权规格收口后push old与main。两分支已普通原子推送并核对远端：old=272bddc，main规格基准=333b219；实际结果见docs/push-log/2026-10-14-0001.md，本条结果记录随后随main同步。

- R1-00已实现并合并（分支codex/platform-r1-00，基线d7cf792431d8，实现提交fa70c657322c）：libs/platform全部17个约束函数与共享类型包（platform.go，仅标准库）、nw-keys密钥工具、scripts/gen_code.py落地make gen。15域internal/gen（oapi-codegen v2.5.1服务端+自动断言客户端）、12个web|ui的api.gen.ts（openapi-typescript 7.10.1）、jobs任务/订阅目录、portal路由表全部生成且可重现；15域生成物经临时模块编译通过（本地与隔离VPS各15/15）。实现中的合同修订：platform新增platform.go、BuildOperationContext加self参数、循环配置加runtime钩子与owner、InvokeModelRequest加onEvent；jobs的ClaimedJob补attempt与dataEpoch；详见libs/platform/STATUS与handoff。THIRD_PARTY_NOTICES登记两个生成工具。各域生成文件在其模块台账仍为planned（design阶段规则），由make gen随时重建。
- platform台账18文件/17函数全部verified（实现+已有检查通过），行为验收待R2；GLM（ZCode）实现署名以提交trailer标注。

- R1-01首个工作包（identity启动装配组）已实现并合并（分支codex/identity-r1-01，基线9c667f3580ee，实现提交533a1239fa8c）：cmd入口、app装配（目录三区/信任锚/签名密钥/运行权威/ApplyRestoreSwap）、transport HTTP生命周期与匿名healthz/readyz、go.mod/go.sum（kin-openapi v0.133.0、oapi-codegen/runtime v1.1.2）、defaults/manifest/Dockerfile/Makefile共9文件；进程模板复用lifecycle.RunService。Serve/Shutdown/Health/Ready台账verified；main与BuildApplication保持blocked（accounts子命令、业务存储、x-handler绑定、换向回执、后台循环依赖后续业务函数，缺口逐条见模块progress.json）。本地与隔离VPS go build/vet/gofmt及make check通过；framework.json登记9文件，THIRD_PARTY_NOTICES补登kin-openapi。site-config（R1-02）就绪前进程按合同以DEPENDENCY_UNAVAILABLE退出，联调等待依赖。

- R1-02首个工作包（site-config启动装配组）已实现并合并（分支codex/site-config-r1-02，基线ffe1ae4，实现提交5ea6affabea0）：与本域R1-01同模式的9文件；关键差异是site-config为运行状态权威，RuntimeClient用本地localRuntime读自身权威（control.db落地前如实返回DEPENDENCY_UNAVAILABLE，不以占位数据冒充）。main/Serve/Shutdown/Health/Ready台账verified；BuildApplication保持blocked（控制库读取、active区DB与迁移、公告journal恢复、x-handler绑定、换向回执、后台循环）。本地与隔离VPS检查通过。

- R1-01第二个工作包（identity私有持久化）已实现并合并（分支codex/identity-r1-01-storage，基线bbb1e535c2f1，实现提交28af316bf6a）：internal/storage.go打开active/identity.db（modernc.org/sqlite v1.59.0，WAL/foreign_keys/busy_timeout=3000ms，单连接串行）、migrations/只追加SQL迁移（编号连续校验、事务执行、集外版本拒绝启动）、control区identity-journal.jsonl决定日志（append+fsync、open条目INTEGRITY_ERROR隔离）；0001_domain.sql建accounts/credentials/account_permissions/sessions/preferences/operation_receipts权威表。BuildApplication存储步骤与Ready深度校验（initialized/journalRecovered/activeDataEpoch代次比对）生效。合同同步：implementation.json增3文件4函数并重算台账摘要，framework.json+3，THIRD_PARTY_NOTICES补modernc.org/sqlite。本地与隔离VPS检查通过。

- 2026-10-15两轮并行交付已合并：identity账号/注册/权限/恢复切片，site-config语言标签与运行状态持久化，jobs任务提交，content草稿提交事务，media上传与blob恢复，traffic-guard策略存储，localization/memory/model-gateway私有持久化，knowledge文本规范化，qa问题规范化，以及portal路由/代理/维护门禁。analytics完成契约审计并如实记录阻塞；各域未完成的handler、事件/任务循环、模型调用和业务验收仍按台账标记。
- 2026-10-15集成复核修复（本会话，main工作区）：在本地Go工具链与隔离VPS首次复核云端切片。修复构建层问题：11个模块go.mod/go.sum不完整，jobs/site-config的go.sum校验和抄错，content/model-gateway/media/portal/site-config有编译错误，vet报非常量格式串，34个文件未gofmt。修复逻辑缺陷：9域运行状态响应未解开`state`外层导致无法启动；portal委托期限超10秒致登录请求全失败，同源判断、CSRF自比、转发头透传可信头，端口与ADR0005错位；content恢复误隔离历史提交；memory启动失败后永久锁死；media改为ADR0005私有SQLite，并修复COMMITTED先于元数据落盘；site-config换回永远无法完成；identity账号索引水位冲突；model-gateway/memory自建outbox/inbox绕开平台。jobs任务类型与identity权限映射改由make gen生成Go表；jobs、traffic-guard、localization、memory、model-gateway、media的迁移改为go:embed唯一来源；jobs `dedupeKey`合同收紧为非空。台账中不存在的变基前SHA已映射为main上的实际提交，qa baseCommit补全。
- 当前台账统计为843文件、905函数：已实现或验证133个（33个verified），in_progress 9个，blocked 14个，planned 749个。本次修复提交为`978e0cee659dc342650193c628710ff75115add5`，新增迁移包/文件与7个函数已回填该SHA并转verified，新增生成表按约定记planned。本地darwin/arm64 go1.27.1与隔离VPS go1.25.5：14个Go模块`-mod=readonly`下build/vet通过、gofmt为空，make docs无漂移、make gen可重复、make check通过（372操作、2161类型）。未新增测试、未运行业务进程、联调、镜像或部署；检查不证明业务行为正确。
- 已推送历史无法改写的记录问题：云端82个提交中只有3个带AI合著者署名；各模块handoff中的旧短SHA仍是变基前编号，以progress.json为准。

下一步：本次修复与台账已在本地提交、未推送（push需当次授权）；后续按各模块handoff的下一项与依赖顺序继续实现。push仍需当次授权；不得把结构检查当作业务验收，也不部署。
