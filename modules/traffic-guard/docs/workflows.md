# traffic-guard 流程

问答准入：入口验证可信服务→内存解析peer→匿名subject→检查维护/封禁/挑战→同一事务扣主体与站点频率并占并发→签名 permit→qa 调 gateway；gateway 独立执行唯一费用预占和结算。

持续问答每10秒续租30秒。续租失败 qa 停止新模型请求并请求 gateway 取消；已经产生供应方费用仍由 gateway 结算，不回退准入频率消费。

挑战完成后仍需重新 admission；PoW 难度固定上限22 bits，外部captcha失败不给放行，匿名HTML读取可独立继续。

恢复管理校验：维护前由identity为每域签发RecoveryAuthorization；本域控制记录/屏障/信任锚独立于业务导入。维护中的prepare/commit/rollback按本地签名、audience、restore/barrier、两epoch、到期和决策状态校验，不回查已换代身份业务DB；普通业务不接受此许可；全域完成后撤销。

old D22/D23/D50 产品保留：Turnstile首选，加载失败PoW；可疑交互重验；确定滥用10分钟IP AI禁令，IPv6 /64；1会话1个问题排队。问题500字/输出500-800-1500token/4子问题/3轮5工具由qa和gateway实现，不在guard复制费用权威。

状态迁移：

准入 evaluate→denied/challenge-required/permitted；permit active→released/expired/revoked

挑战 issued→verified/expired/consumed；成功只赋予 10 分钟 challengePassed，不免除频率或并发限制

禁令 active→expired/revoked；每次 admission 和 permit renew 都重新检查

并发和幂等：每个修改函数明确CAS、fencing/租约或事务边界，见functions；公共事件至少一次，不能依赖全局顺序。接收eventId相同但hash不同拒CONFLICT；较低sourceRevision记已见但不应用；旧dataEpoch拒绝并不能重绑新epoch。
