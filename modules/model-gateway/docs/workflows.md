# 统一模型网关算法与工作流

状态机：Invocation: accepted→reserved→dispatching→streaming→succeeded|failed|cancel_requested→cancelled|charge_unknown；charge_unknown→reconciled。Reservation: held→settled|released；未知账单保持held。Provider: disabled|enabled。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `model-gateway.LoadRuntime` | 启动时 | 启动并验证本服务私有持久化与密钥引用 |
| `model-gateway.RecoverLeases` | 启动时及每30秒 | 恢复崩溃中的调用 |
| `model-gateway.PublishOutbox` | 启动后持续，空闲每1秒 | 持久发布事件 |
<!-- flows:end -->

## 免费任务配置能力读取

仅已认证QA纯服务调用ReadTaskAvailability，按原请求顺序返回qa.preflight/qa.inspect/qa.answer的安全三态。统一入口处理服务断言、schema、当前runtime、维护与截止，本域只核该只读调用范围；在一次严格epoch的SQL快照内关联任务赋值、QA档位和候选元数据。缺赋值/档位为not_configured；已配但模型/供应方停用、chat不匹配或当前凭据未配为unavailable；存储未知或损坏必须返回公共错误，不伪造false。

单任务覆盖替换orderedModelIds第一项，原第二项起有序备用保留并去重；列表空可仅覆盖，不把被替换主模型加回备用。合法配置上限256项，先按完整配置过滤本地eligible，执行ResolveRoute再保序返回至多16个候选，不能先截断配置。本规则同步ResolveRoute参考合同，本轮不实现付费路由。所有候选只核元数据存在性，不读取密文/主密钥或供应方URL，不查价格、余额、预算，不发HTTP探测、不占额度或调用模型。即使返回available，也只表示当前配置材料具备。

## 任务分配配置读写

任务目录只由根contracts/ai-tasks.json生成到本域既有clients.gen.go；每任务默认档位不由管理员请求改写。覆盖按该任务的目录能力全部检查；保存只核模型已启用、同代和能力，不要求已配置凭据/价格，也不探测供应方。

管理写入经平台入口即时核superadmin；同一SQL事务核非空epoch、7天回执、屏障、模型能力与任务版本。有效同代原回执先返回，不重做当前模型检查或产生副作用；新配置才检查所有非released屏障并执行条件CAS，assignment.changed和原结果同事务写平台outbox/回执。过期键仅随成功新配置事务替换，失败回滚保持原状态。

任务读取按目录owner过滤纯服务范围，portal管理员读取在HTTP边界另查identity当前superadmin权限。只返回已存同代赋值，未配置不从默认目录合成；任务覆盖的缺失表示清除。完整服务仍因其他handler/epoch初始化/恢复/后台未齐拒绝启动。

## 档位配置保存

PutProfile在同一当前代次事务内，依次处理7天原结果、全部非released屏障、当前版本与唯一槽、候选直接能力、条件CAS、profile.changed平台outbox及完整回执。事件taskIds对旧/新fast改动保守包含vision/rerank回退任务并去重排序。有效原结果重放不受后来模型停用或本地写屏障阻断；未知Commit不自行重试。保存只核模型元数据，不读取价格、凭据或供应方在线性。

空候选列表保留配置行。old规定的视觉/重排未设回退保留在尚未实现的ResolveProfile/ResolveRoute：该行缺失或候选数组为空时，分别使用fast中支持vision的候选、或由fast模型代替重排；不在保存时合成候选，不自动重算嵌入或已有付费结果。

## 凭据保存与版本切换

可信启动只保存可选masterKeyRef，不读取secret；免费配置读取不依赖它。凭据写入才安全有界读取32字节主密钥、计算域分离HMAC输入摘要，严格当前epoch事务先处理7天原结果，再核所有未释放屏障、真实active/供应方CAS和未复用历史版本。新nonce/AAD加密后，密文、active、provider.changed、脱敏回执同事务；无网络探测或费用任务。

Put可首次保存/替换或修复缺失active材料；Rotate需要既有完整active材料，全部旧密文保留。当前没有真实派发版本引用与释放依据，最后引用销毁仍未交付，Rotate/credential.go整体partial；不能因invocations为空而清密文。

第二轮收口窄修：PutProvider同代普通摘要显式绑定callerService/dataEpoch，7天原结果先于新写屏障和当前active指针返回；新写拒绝任意非released屏障，不按屏障epoch/deadline过滤。过期键仅随成功新配置事务替换，PutModel及其旧共享helper未改。readCredentialReceipt与新增渠道回执参考签名均以ctx,op开头。首次候选Go检查通过但结构check因签名顺序失败，该次不记完整通过；修后根全量生成/编译/结构检查通过，证据见本轮交付记录。
