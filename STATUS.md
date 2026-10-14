# 当前状态（2026-10-14）

阶段：微服务设计文档已完成本轮编写和机械检查，业务实现尚未开始。

- old归档提交272bddc保存全部旧代码、旧文档及会话记录；旧server/web/api/deploy相对原main的blob未改变。main当前树只保留新框架、许可和产品需求快照，不复用旧业务实现。
- 站点功能及细节按old产品基线保留；分成15个独立服务加2个共享组件（libs/platform、deploy），登记825个未来文件、890个函数、372个HTTP操作、2159个类型定义（含公共类型引用和内部类型）；约束/参考数量见各域files.md。这些数量是规格条目，业务实现、验证、行为验收均为0。
- 每域有总索引、架构/数据/API/流程/文件/函数/运维/验收文档、四份机器契约，以及STATUS/progress/handoff。独立启动、健康/就绪、构建部署和UI装配条目已登记。
- 多会话按独立worktree和模块所有权开发；公共契约由集成会话统一修改。新会话先读根及本域交接并核对Git事实，每次只领取一项，记录提交和检查证据。
- 隔离VPS执行 `make check` 已通过：JSON/引用、文档文件链接、15模块资料、函数/类型/HTTP/权限/依赖、唯一事件生产方与载荷、结构清单、交接台账和隐私。未运行新业务编译、业务测试、E2E、真实模型或部署；检查不证明业务行为正确。
- 规格修订（ADR0002，用户授权）：事件统一经jobs拉取投递——9个消费域补订阅循环并在events.json登记handler，identity/site-config/content/media补outbox转发，jobs推送投递改为到期重投与死信，删除analytics推送入口和public-site两个无人生产也无人消费的事件，portal访问审计改调analytics:appendAudit。Go操作函数统一(ctx context.Context, op OperationContext)，浏览器函数统一(input, signal)；函数ID统一为“模块.函数名”；依赖全部可解析，非函数依赖移入resources；函数分约束/参考两级；functions.md/files.md改由make docs生成。本地make check通过（新增依赖、签名、事件链与生成文档校验）；未在VPS复跑，未编译或运行业务。
- 恢复简化（ADR0003，用户确认R0-07）：删除各域在线validate/prepare/commit/rollback共56个接口、专用恢复许可与单域激活；改为各域stage暂存→data-protection写切换决定→各服务由进程守护重启并在启动时换入active→全部回执后解除维护，失败可整体换回；恢复入口仍在初始化向导与后台。部署须配置自动重启。各域api.md接口清单改由make docs从OpenAPI生成。本地make check通过。
- 接线与工程（ADR0004、ADR0005，用户授权“按你的想法做好”）：逐条追踪关键流程并计算可达性，补齐编排函数、任务与后台登记、portal入口准入、模型结果读取与统一服务断言，除界面外函数全部可达；补齐old后台功能所需接口（估价、问答记录、可疑事件、任务队列、翻译进度与对照编辑、配置读取）；定时备份授权集中在data-protection；新增平台公共库与部署编排合同，统一生成代码登记，确定依赖许可、端口与内存预算；frontend.md列出后台页面与接口对照；R1拆为17个工作包。本地make check通过。
- Git作者使用用户已授权的既有GitHub代用身份，只修改本仓库配置。私有vps.txt被忽略且未同步；旧线上实例与数据未改变。
- 用户已明确授权规格收口后push old与main。两分支已普通原子推送并核对远端：old=272bddc，main规格基准=333b219；实际结果见docs/push-log/2026-10-14-0001.md，本条结果记录随后随main同步。

下一步：用户确认R0-05整体设计（含Docmost借用例外）；实施从identity、site-config、content、jobs、public-site最小链路开始，按docs/plan.md和目标模块交接认领一个工作包；先确认固定工程依赖/生成器与模块实现范围，不能把本轮文档push当作已实现或已上线。
