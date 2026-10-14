# jobs 数据与持久化

未来私有目录 `data/jobs/active/` 保存SQLite和领域对象；`data/jobs/restore/<restoreId>/staged|before/` 为恢复暂存与换出的旧数据；`data/jobs/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `jobs(job_id PK, owner, type, immutable_input_ref, input_hash, dedupe_key UNIQUE per owner/epoch, state, attempt, not_before, data_epoch, revision)`

- `leases(job_id PK, worker_id, fencing_token, expires_at, last_heartbeat)`

- `progress(job_id, seq, phase, completed_units, total_units, safe_message_code; PK job_id/seq)`

- `job_results(job_id PK, result_ref, paid_result_refs_json, result_hash)`

- `events(event_id PK, event_seq INTEGER UNIQUE递增, envelope_json, source_revision, data_epoch)`

- `deliveries(event_id, consumer_id, state, attempt, retry_at; PK event_id/consumer_id)`

- `schedules(schedule_id PK, owner, cron, timezone, revision, enabled, next_fire_at, last_tick)`

- `barriers(barrier_id PK, state, domain_status_json, data_epoch, deadline)`

- `inbox(event_id PK, producer, revision, data_epoch)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP管理协议（barrier/export/stage/discard），在safe屏障固定revisionWatermark；恢复先全域暂存，写切换决定后各服务重启换入，全部域登记回执才解维护。control区不随恢复替换。换入后先schema迁移与完整性验证，失败拒绝启动并可整体换回。
