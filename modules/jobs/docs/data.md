# jobs 数据与持久化

未来私有目录 `data/jobs/active/` 保存SQLite和领域对象；`data/jobs/staging/<restoreId>/` 为隔离导入区；`data/jobs/control/` 保存不被业务恢复替换的barrier/recovery授权信任锚和操作journal。外域不得直接读这些路径。

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

快照只经HTTP六管理协议，在safe屏障固定revisionWatermark；导入全域校验通过后prepare，持久commit决定后切代，全部域activate才解维护。当前恢复control记录不得由导入覆盖。迁移先隔离拷贝、schema校验、SQL迁移、完整性验证，失败保留旧active指针。
