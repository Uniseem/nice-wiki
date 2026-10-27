# ADR0004：流程接线完整性、服务身份与入口准入

- 日期：2026-10-14。
- 状态：用户授权“把所有的规划按你的想法做好”，已写入合同；业务实现尚未开始。
- 后续（2026-10-27）：第 6 条“备份恢复授权集中在 data-protection”及其屏障、导出、暂存、丢弃接口已作废（服务和协议都删除了），其余接线与服务身份决定仍有效。
- 来源：对规格逐条追踪关键流程（访客付费问答、发布→翻译、权限撤销、初始化、媒体回收），并从全部入口计算函数可达性。

## 问题

1. 836 个函数中有 257 个从任何入口都不可达：问答、检索、翻译、记忆、模型网关与 portal 代理只有零件没有编排函数；启动恢复与定时清理函数没有挂到启动流程；public-site、analytics、data-protection、media 的异步任务没有领取循环；各 RunWorker 引用的 JOB_HANDLERS 从未定义。
2. 访客提问路径上没有人申请风控准入：traffic-guard 需要访客原始地址，只有 portal 拿得到，但 portal 转发时删除了地址也不申请；问答扣减的许可无人签发。
3. 24 个调用模型的函数只依赖 createInference（202 受理），没有任何函数读取结果流，模型输出拿不回来。
4. 功能缺口：启用语言前估价确认（old B15）、界面字符串翻译入口、访客语言偏好入口、记忆恢复/重排/清单接口、死信重放、镜像状态查询、发布写 data git 镜像的触发、内容缺口统计的数据来源。
5. 权限与一致性：定时备份过不了要求超管委托的屏障/导出接口；预览入口权限写成 service；HTML 访问服务端与浏览器重复计数；knowledge 读取附件时误用引用预留；服务身份三套写法（mTLS proof、服务签名凭证、mTLS 服务令牌）；生成代码三套登记方式。

## 选择

1. **显式登记任务与后台函数**：每个 implementation.json 增加 `jobTypes`（类型、执行函数、触发、可选系统定时）与 `backgroundTasks`（函数、运行时机）。RunWorker 依赖全部执行函数，BuildApplication 依赖全部后台函数；有系统定时的域在启动时以服务身份经 jobs:createSchedule 幂等登记。jobs 的类型白名单与事件订阅目录由这些清单生成。
2. **补编排函数**：model-gateway.ExecuteInvocation、knowledge.ExecuteBuild/ExecuteExtraction、qa.ExecuteTurn、localization.ExecuteTranslation/ExecuteCacheAnswerTranslation、data-protection.ExecuteBackup/ExecuteRestore、portal.ProxyRequest，以及 media、public-site、analytics、data-protection 的 RunWorker；knowledge 检索、memory 上下文解析、public-site 页面渲染补齐零件依赖。
3. **模型调用统一读取到终态**：调用模型的五个域各有 InvokeModel 助手（createInference→resumeInference→必要时 cancelInference），原调用点改为依赖它；问答流式输出经它逐条转发。
4. **入口准入**：OpenAPI 用 `x-admission` 标记公开 AI 接口（qa createConversation/startTurn、localization suggestLanguage）。portal.ProxyRequest 对这些路由用可信代理链得到的访客地址（只在内存）调用 traffic-guard:createAdmission，通过后附 `X-Admission-Permit` 转发；问答消费许可、长回答续租、排队轮次换许可，网关本地验签。
5. **服务身份统一**：服务之间用部署时生成的 Ed25519 密钥签发≤60秒的服务断言（JWT EdDSA，绑定 audience、method、path、bodyHash、jti）；用户身份由 portal 签发≤10秒的委托证明向下传递，接收方仍经 identity:checkAuthorization 核权限与会话版本；访客用 traffic-guard 签署的伪名会话。全部 OpenAPI 使用同一组安全方案（serviceAssertion、userDelegation、visitorSession、admissionPermit，portal/identity 另有 sessionCookie、csrfToken）。不使用 mTLS。
6. **备份恢复授权集中在 data-protection**：各域屏障/导出/暂存/丢弃接口只认 data-protection 服务身份；手动备份与恢复由 data-protection 核验 superadmin，定时备份按超管保存的设置执行。
7. **功能补齐**：localization 估价/批准/界面补译/访客偏好接口，memory 恢复/重排/清单接口，jobs 死信重放，data-protection 镜像状态与发布镜像订阅，analytics 订阅问答完成/失败事件（问答完成事件增加 answerKind），qa 删除无人调用的 translateCacheEntry 接口，public-site 删除无用的 ParseMarkdown，model-gateway 删除没有任务类型的 RunWorker。
8. **检查**：check_docs 校验清单格式、执行函数挂接、后台函数挂接、除界面外函数全部可达、service 接口必须有调用方、x-admission 只在匿名接口；workflows.md 的“入口与后台流程”由 make docs 生成。

## 代价与影响

- 合同与生成文档再次大幅变化；所有条目仍是 planned。
- 可达性只证明“有入口调用”，不证明调用顺序与分支正确；关键编排函数的步骤仍需实现时逐条对照 ai-context-and-cache.md 等规格。
- 界面函数不在可达性规则内，界面页面清单见 frontend.md。
