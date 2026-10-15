# 站点配置与初始化当前进度

## 第二轮最终集成与停止范围

- 一般设置权威读写、校验、对象原始流及HTTP核心通过；Home等及完整启动仍未完成。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


## 持续实施第二轮（2026-10-15）

- 基线 `468986fc34e1c90b6b5ca575539fdd2b9748cd06`，分支 `codex/site-config-settings-r1`，仅拥有 `modules/site-config/`；已交付核心函数通过最终统一 VPS 门槛和复核，记 verified（仅对应检查范围）。
- 已写一般设置的 ReadSiteConfig/UpdateSiteConfig、纯 ValidateSiteConfig/NormalizeBaseURL、ReadConfigObject 原始对象流；复用既有文件 IO、COMMITTED/CAS/回执与平台 outbox，不复制公共身份/签名机制。
- 一般设置候选材料只供未完成操作恢复，保存首次 updatedAt；正式读取与成功重试均读不可变文件。新增 0004 只追加迁移保存候选材料并约束对象 ID/站点修订唯一。
- NormalizeBaseURL 依赖根提供的 lifecycle.NormalizePublicURL（统一 IDNA/URL/显式 loopback HTTP 能力）；公共 HTTP 工厂冻结后再接线，不自造入口机制。
- ReadConfigObject 限当前真实已提交 settings；返回进程内 Reader，HTTP 直接消费关闭并输出绑定元数据，不发送 bodyHandle。
- Home 组保留下一紧密组：按钮禁用/解析 URL/AI 可用性协议及 media pin 崩溃续办尚未收口；完整 HTTP、RelayOutbox、恢复回执与快照/屏障继续未完成。
- 已写真实 HTTP 协议适配：config 读写/原始对象流、初始化状态/三用例、runtime 读/迁移/activation 与健康/就绪；统一工厂建立身份/epoch/期限/schema，并使用平台 identity 权威授权回调。缺 Home/品牌/公告/历史/快照/屏障等真实 handler 时工厂拒绝启动，完整 BuildApplication 仍 blocked。
- VPS 合成快照（315a3ff+公共26138c1）docs/gen及14模块readonly build/vet/gofmt通过；check余4项均非本域。随后按复核补维护边界：updateSiteConfig仅旧成功只读重放可到达，新/未完成操作在任何候选IO前CheckWriteEpoch；该追加修复已被最终VPS复跑覆盖（见下条最终证据）。
- 最终证据：隔离 VPS make docs/gen、14模块readonly build/vet/gofmt及完整 make check 全通过（855文件/1012函数/374操作/2211类型），包含维护边界修复；原先“等待VPS/非本域检查失败”为过程历史，已由此最终结果覆盖。
- 本轮源码提交 948a834、39ddb52、315a3ff、9a21f1b；依赖根公共26138c1及后续统一修复，交付只含本域，真实主线SHA由集成会话映射。
- 按用户最新指令，本轮收口后停止自动实施，不开展第三轮；本域不push，集成会话统一执行本轮已获授权的推送。Home等剩余项保持planned/blocked，不标全产品完成。
- 本轮未在本机格式化、生成、编译、vet、lint、类型检查或测试；未新增测试/样例/fixture/临时验证程序，未运行服务、真实接口、模型、部署或 push。

---


最新统一集成事实（2026-10-15）：初始化与settings权威提交、恢复和文件IO通过；配置编辑、完整HTTP/后台仍未完成。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：a0d6adf、0e8cc9f、c2d734b，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


更新：2026-10-15。阶段：R1-02 实施中；本轮初始化完成与 settings 权威提交及配置 IO 边界修复已实现，经隔离 VPS 全仓编译/生成/完整结构检查与集成复核，已交付函数记 verified（仅对应检查范围）。业务验收与部署未开始。

- 本轮源码提交 `5b6bc4e39fd72457d302031e0f369cece55ae1c5`，配置 IO 修复 `53156c7d74fb065b1355250ab916d03518c49d0b`，格式/生成与 vet 收口 `6396715d91a56bb6a12cb4ddd151b4117ff27520`、`8613254c72d63ddf30ce3710cf222d21a7738181`；尚未 push。
- 基线 `325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，分支 `codex/site-config-initialization-complete-r1`，独占 `modules/site-config/`。
- 已写 InitializeSite/ResumeInitialization/AbortInitialization：唯一超管会话与 site 作用域、稳定业务摘要、原始结果回执、不可反悔决定、blocked 后幂等补完；不调用 AI、identity owner 准备或真实接口。
- 最终检查：隔离 VPS `make docs/gen` 成功；14 个 Go 模块 `make build`（readonly build/vet/gofmt，含本域）及完整 `make check` 均通过（850 文件/943 函数/372 操作/2186 类型）。此前 IO 错误包装 vet 失败已修复并经本轮复跑通过。
- 逐函数复核追加：配置 IO 使用 os.Root 目录能力逐段 Lstat，拒绝链接/特殊文件，正规文件打开前后身份核对，正文与 journal/head 的 JSON 上限 16 MiB；该追加修复已通过本轮 VPS 门槛。
- 已写 WriteConfigRevision/CommitConfigHead/RecoverConfigJournal：schemaVersion=1 权威文件、不可变 PREPARED/COMMITTED、排他 Link、head Rename、逐级 fsync、SQLite 写锁/CAS、崩溃回放、孤儿隔离、SQL 与文件水位核对。
- `0003_config_commit.sql` 只追加 config_operations；配置水位/回执和平台 outbox 同事务，不复制公共 outbox 表或转发机制。RelayOutbox 循环仍待 HTTP/后台装配工作包。
- 可信公开地址由公共 ServiceConfig 边界校验后 ConfigureInitialization 注入，缺失地址初始化失败关闭；不从请求 Host 推导。启动恢复只根据明确 COMMITTED 决定，旧准备决定缺身份回执明确失败关闭。
- 未完成：完整 OpenAPI 业务 x-handler、其他设置用例、RelayOutbox 循环、恢复换目录回执与快照/屏障装配。BuildApplication 的本组接线不等于完整模块装配完成。

本轮未在本机执行编译、格式化、lint、类型检查、测试或生成校验；未新增测试、样例、fixture 或临时验证程序；未运行服务、真实接口、模型、联调、镜像、部署或 push。
