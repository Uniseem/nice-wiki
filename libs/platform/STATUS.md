# platform 状态

- 阶段：R1-00 已实现并检查（分支 codex/platform-r1-00，基线 d7cf792431d8，交付 fa70c657322c；台账全部 verified，行为验收待R2）。
- 范围：libs/platform 全部17个约束函数与共享类型包（platform.go）、nw-keys、scripts/gen_code.py 与 make gen；15域生成物可重现并编译通过。
- 检查：本地与隔离VPS（go1.25.5、8核）go vet ./...、go build ./... 通过；15个模块生成物经临时模块逐域编译验证（本地与VPS各15/15）；make check 本地与隔离VPS通过；make gen重复运行无差异。未运行业务验收。
- 契约修订（已同步 implementation.json/types.json 并 make docs）：新增 libs/platform/platform.go 共享类型包；BuildOperationContext 增加 self 参数；InvokeModelRequest 增加 onEvent 回调；RelayConfig/ConsumerConfig/WorkerConfig 增加 runtime 钩子（ConsumerConfig 另有 validatePayload）；WorkerConfig 增加 owner；DelegationExpectation.route 允许下游转发留空。跨域修订：jobs ClaimedJob 增加 attempt 与 dataEpoch（领取结果是JobExecutionInput唯一来源）。
- 实现时确认的口径：nw-keys 读取的 deploy/config/services.json 按 {version,services:[{slug,secretsDir}]} 解析并容忍其余字段，deploy实施时对齐；私钥为PKCS8 PEM（0600），公钥为base64url；trust-anchors.json排序输出；重放缓存与私钥缓存均在进程内。

详细交接见 docs/handoff.md；逐文件/函数台账见 progress.json。
