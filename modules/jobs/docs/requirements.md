# jobs 需求

领域算法由所属服务通过 HTTP 执行。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R002** 任务状态机、owner白名单、去重键、租约fencing和进度SSE

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R003** 带版本事件至少一次投递、inbox/outbox、消费者游标

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收


验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R005** 不承载翻译/embedding/渲染/恢复算法

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R006** 不重做已付费成功结果，不持有模型密钥或钱账

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R007** 不把停止新任务等同所有在途领域写入已安全

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R008** 服务间HTTP/JSON/SSE；事件使用公共EventEnvelope并标版本，至少一次按eventId和源修订去重

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R009** 所有写入验证当前dataEpoch，恢复后旧任务/租约/token/缓存不得修改或授权新数据

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R010** 私有存储；外域只通过拥有者HTTP合同读写，不共享数据库或直接读私有文件

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收


验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R012** 日志、事件与镜像不记录IP、提问正文或secret；结构化字段仅allowlist

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R013** 未来Go后端、Vue3模块界面按文件/函数合同手写；本阶段只登记设计，无业务源码和测试样例

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R014** owner停机：lease到期标orphan等待owner确认结果，不自动执行不可证明未完成的付费阶段。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R015** jobs停机：领域同步读取仍按本域权限；新异步工作明确503，领域outbox保留。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R016** 事件consumer停机：该consumer delivery退避/死信，其他consumer独立继续。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M13-R017** 恢复旧epoch拒绝所有旧job/fence/event；定时器从新epoch未来tick继续，已有paid结果refs只读复用。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

首版默认参数：

```json
{
  "leaseSeconds": 30,
  "heartbeatSeconds": 10,
  "claimBatchSize": 10,
  "maxAttempts": 5,
  "retrySeconds": [
    5,
    30,
    120,
    600
  ],
  "progressRetentionDays": 7,
  "terminalJobRetentionDays": 30,
  "eventRetentionDays": 30,
  "maxPayloadRefBytes": 16384,
  "scheduleTickSeconds": 60,
  "scheduleMissedTickPolicy": "coalesce-latest-one",
  "scheduleTimezoneDefault": "Asia/Shanghai",
  "maxConcurrentClaimsPerOwner": 8,
  "eventDeliveryTimeoutMs": 5000,
  "sseKeepaliveSeconds": 15
}
```

精确参数约束以types.json为准；不从客户端接受成本分类、可信actor、IP或secret。
