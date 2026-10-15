# jobs 数据与持久化

未来私有目录 `data/jobs/active/` 保存SQLite和领域对象；`data/jobs/restore/<restoreId>/staged|before/` 为恢复暂存与换出的旧数据；`data/jobs/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `jobs(job_id PK, operation_id, root_operation_id, owner, type, immutable_input_ref, input_hash, dedupe_key UNIQUE per owner/epoch, state, attempt, not_before, data_epoch, revision, fencing_counter, last_error_code, cancel_requested)`

- `leases(job_id PK, worker_id, fencing_token, expires_at, last_heartbeat)`

- `lease_operations(owner, data_epoch, operation_kind, operation_id, request_hash, result_json; PK owner/data_epoch/operation_kind/operation_id)`

- `progress(job_id, seq, phase, completed_units, total_units, safe_message_code, checkpoint_key, input_hash; PK job_id/seq，非空checkpoint_key按job唯一)`

- `job_results(job_id PK, result_ref, paid_result_refs_json, result_hash)`

- `events(event_id PK, event_seq INTEGER UNIQUE递增, envelope_json, source_revision, data_epoch)`

- `deliveries(event_id, consumer_id, state, attempt, retry_at; PK event_id/consumer_id)`

- `schedules(schedule_id PK, owner, cron, timezone, revision, enabled, next_fire_at, last_tick)`

- `barriers(barrier_id PK, state, domain_status_json, data_epoch, deadline)`

- `inbox(event_id PK, producer, revision, data_epoch)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP管理协议（barrier/export/stage/discard），在safe屏障固定revisionWatermark；恢复先全域暂存，写切换决定后各服务重启换入，全部域登记回执才解维护。control区不随恢复替换。换入后先schema迁移与完整性验证，失败拒绝启动并可整体换回。

租约实施补充（R1-03）：`fencing_counter` 属于job权威记录，不随释放lease归零；领取时原子递增，后续回收也须递增同一计数。claim回执保存完整原批次，heartbeat回执固定原expiry；回执只存安全引用及控制状态。可选心跳进度在同一租约事务内按checkpointKey/inputHash幂等，seq单调且同phase完成数不得倒退，事件经平台outbox记录。

屏障布局待后续工作包收口：0001迁移曾把barriers登记在active库，与control区约束不符；本次租约代码不读取该表，必须由装配层注入可信runtime与control屏障安全点/deadline，缺少钩子或维护期有效截止时失败关闭。完整屏障工作包再处理持久布局与旧表迁移。


终局与恢复实施补充（R1-03）：`job_outcomes(job_id,fencing_token,operation_kind,worker_id,request_hash,result_json)`保存原终局，`lease_operations`扩展complete/fail/cancel种类；两类回执与结果、lease释放、状态outbox同事务。`job_results.result_ref`保存ObjectRef JSON，无主结果用空串；paid_result_refs_json仅合并保留引用，不因取消、失败或重试清空。终局不伪造retryAt。

`job_orphans(job_id,expired_fence,worker_id,next_check_at)`只记录已失效执行权的恢复引用和轮转检查时间；旧fence不能复活。owner确认调用在事务外，应用结果时重新核当前epoch/control并CAS。历史root_operation_id未知留空，不猜operation_id就是付费根；Claim排除空根，回收事务按orphan隔离并写outbox。迁移只改结构，不做无事件的业务状态迁移。


取消意图实施补充：`cancel_requested`是持久权威，不能因临时失败改为retry-wait而丢失。取消后未报告安全CANCELLED的失败进入orphan并交还lease，既有付费引用保留；只有owner确认completed或安全结束才收敛。后台不得重新领取、开始或重排带取消标志的任务。


事件投递实施补充（第二轮）：events追加producer/type/version/subject与规范完整envelope_hash，event_sequence单行保持不可复用的接收seq。deliveries追加稳定delivery_id、lease_expires_at、revision和last_error_code；queued首次发出后才有确认租约，超时可退避重排或死信，绝不自动覆盖ack。consumer_offsets按consumer/epoch保存连续确认水位，event_operations按caller/epoch/种类/operation保存安全成功回执及输入hash，不保存输入正文。

订阅目录和payload规格均由所有域events.json生成的EventTypes提供，不从请求动态注册。唯一生产者、消费者范围和具体重放权限均取该目录；正文actor/epoch不构成授权真相。旧delivering缺租约无法证明安全时返回完整性错误，不造时间；旧事件原文及元数据、全部投递均保留。
