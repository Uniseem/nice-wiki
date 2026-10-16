# 持续目标第二轮公共IO错误工具（VPS检查通过）

集成会话新增NormalizeIOError，保留公共错误及统一ctx取消/截止，未知错误只用可信代码选择的安全说明，不传播原始存储错误。problem.FromError复用它，保持未知INTERNAL兜底；新域不再复制context错误分类。无新路径/外部依赖；首次完整编译通过但新工具因既有FromError未登记而结构检查失败，补真实FromError及jobs响应调用边界后待完整重跑，再回填源码SHA；不代表业务验收。

# 两轮并行第二轮公共前置（2026-10-16，已检查）

ServiceConfig新增可选masterKeyRef及NW_MASTER_KEY_REF覆盖，非空仅校验绝对路径。加载配置不读取主密钥，model-gateway的免费读取可独立；密钥实际读取和加密由该域凭据用例负责。无新增函数、路径或依赖；源码993904524e4d7a792a6c84a80606de6215f7ace5，VPS docs/gen、14模块readonly build/vet/gofmt及完整check通过（865/1258/379/2277）；对应文件/函数准确引用源码。未读取真实密钥或执行行为验收。

# platform 状态

第四轮公共CanonicalJSON、有界写窗口/恢复循环与后台安全关闭已完成，源码9ff93ae。公共源和jobs分别集成后实际树均通过VPS docs/gen、14模块readonly build/vet/gofmt及完整check，最终858文件/1105函数/379操作/2253类型，生成零变化；调用方Home Go仍待完整检查，不称行为验收或部署。逐项以progress.json为准。下述早期过程记录保留溯源。

第二轮已实现公共机制通过最终VPS gen/docs、14模块build/vet/gofmt及完整check，函数台账已更新实际提交与证据；调用方未完成的完整启动、guard/peer/cookie和owner机制不因此变成可用。第二轮后已按要求停止；用户随后授权继续，第三轮可信代理来源、每日网络HMAC及浏览器认证类型合成已在冻结合成快照通过VPS docs/gen、14模块build/vet/gofmt与完整check，等待逐模块集成复核。


最新统一集成事实（2026-10-15）：会话签名、委托、可信配置和worker控制/结果链已通过统一复核；调用服务完整装配仍未完成。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：dad2dce，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


本轮持续实施集成分支（未合main）：统一会话访问签名/独立秘密、专用委托签名、会话版本、可信公共地址与任务完整结果写回。worker控制操作使用独立幂等键，费用根与executor原操作不变；取消确认由owner负责，权威未知暂停领取。
VPS上一快照14模块build/vet/gofmt与make check通过；委托二跳与代理错误映射的最后修订等待最终复跑。业务验收和部署未执行。

最新集成结果：隔离VPS的14模块make build与完整make check通过；本轮已完成条目转verified，实际主分支SHA见progress.json。部分入口/用例仍未实现，业务验收和部署未开始。下列本机与待登记说明保留为交付时历史事实。

- 阶段：R1-00 已实现并检查（分支 codex/platform-r1-00，基线 d7cf792431d8，交付 fa70c657322c；台账全部 verified，行为验收待R2）。
- 范围：libs/platform 全部17个约束函数与共享类型包（platform.go）、nw-keys、scripts/gen_code.py 与 make gen；15域生成物可重现并编译通过。
- 检查：本地与隔离VPS（go1.25.5、8核）go vet ./...、go build ./... 通过；15个模块生成物经临时模块逐域编译验证（本地与VPS各15/15）；make check 本地与隔离VPS通过；make gen重复运行无差异。未运行业务验收。
- 契约修订（已同步 implementation.json/types.json 并 make docs）：新增 libs/platform/platform.go 共享类型包；BuildOperationContext 增加 self 参数；InvokeModelRequest 增加 onEvent 回调；RelayConfig/ConsumerConfig/WorkerConfig 增加 runtime 钩子（ConsumerConfig 另有 validatePayload）；WorkerConfig 增加 owner；DelegationExpectation.route 允许下游转发留空。跨域修订：jobs ClaimedJob 增加 attempt 与 dataEpoch（领取结果是JobExecutionInput唯一来源）。
- 实现时确认的口径：nw-keys 读取的 deploy/config/services.json 按 {version,services:[{slug,secretsDir}]} 解析并容忍其余字段，deploy实施时对齐；私钥为PKCS8 PEM（0600），公钥为base64url；trust-anchors.json排序输出；重放缓存与私钥缓存均在进程内。

详细交接见 docs/handoff.md；逐文件/函数台账见 progress.json。
- 2026-10-15 补充：新增 `lifecycle.NewSiteConfigClient` 与 `apivalidate.NewRequestValidator`（平台库新增 kin-openapi 依赖）；make build 与 make check 通过，详见 [handoff](docs/handoff.md)。

- 本轮三个工作包集成：新增作用域绑定的分页游标签名/验证，CheckLease补workerId；平台单模块编译/vet通过，新增函数已按隔离VPS全量检查转verified。

本次最终检查：2026-10-16持续目标第二轮公共前置：VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14模块readonly build/vet/gofmt和完整make check通过（872/1332/379/2297）。HTTP用实际RequestBody，dataEpoch仅可信上下文；公共错误分类有真实FromError调用。无新测试/业务验收/真实接口模型/服务部署。
