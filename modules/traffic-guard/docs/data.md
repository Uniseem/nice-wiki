# traffic-guard 数据与持久化

未来私有目录 `data/traffic-guard/active/` 保存SQLite和领域对象；`data/traffic-guard/staging/<restoreId>/` 为隔离导入区；`data/traffic-guard/control/` 保存不被业务恢复替换的barrier/recovery授权信任锚和操作journal。外域不得直接读这些路径。

- `guard_policy(site_id, revision, policy_json, data_epoch; PK site_id)`

- `buckets(scope_key PK, capacity, tokens_milli, updated_at_ms, data_epoch)`

- `permits(admission_id PK, request_id UNIQUE, subject_key, action, expires_at, hard_deadline, state, policy_revision, data_epoch)`

- `challenges(challenge_id PK, subject_key, nonce_hash, difficulty, expires_at, state, attempts)`

- `bans(ban_id PK, subject_key, scope, starts_at, expires_at, reason_code, revision)`

- `event_inbox(event_id PK, source_revision, data_epoch)`

- `outbox(event_id PK, payload_ref, state, retry_at)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP六管理协议，在safe屏障固定revisionWatermark；导入全域校验通过后prepare，持久commit决定后切代，全部域activate才解维护。当前恢复control记录不得由导入覆盖。迁移先隔离拷贝、schema校验、SQL迁移、完整性验证，失败保留旧active指针。
