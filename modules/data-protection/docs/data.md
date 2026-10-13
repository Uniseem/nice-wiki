# data-protection 数据与持久化

未来私有目录 `data/data-protection/active/` 保存SQLite和领域对象；`data/data-protection/staging/<restoreId>/` 为隔离导入区；`data/data-protection/control/` 保存不被业务恢复替换的barrier/recovery授权信任锚和操作journal。外域不得直接读这些路径。

- `backup_runs(backup_id PK, source_epoch, barrier_id, status, manifest_hash, encrypted_manifest_ref, key_id, created_at, completed_at)`

- `backup_objects(backup_id, owner, snapshot_id, schema_version, watermark, object_hash, encrypted_ref; PK backup_id/owner)`

- `restore_runs(restore_id PK, backup_id, source_epoch, target_epoch, barrier_id, status, runtime_revision, committed_at)`

- `restore_decisions(decision_id PK, restore_id UNIQUE, target_epoch, decision(commit/abort), prepared_manifest_hash, signature, decided_at)`

- `domain_stages(restore_id, owner, stage_id, snapshot_hash, state, last_error_code; PK restore_id/owner)`

- `mirror_runs(mirror_id PK, backup_id, manifest_hash, local_commit, remote_ref, state)`

- `remote_config(config_id PK, url_without_credentials, branch, credential_ref, revision)`

- `outbox(event_id PK, payload_ref, state, retry_at)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP六管理协议，在safe屏障固定revisionWatermark；导入全域校验通过后prepare，持久commit决定后切代，全部域activate才解维护。当前恢复control记录不得由导入覆盖。迁移先隔离拷贝、schema校验、SQL迁移、完整性验证，失败保留旧active指针。
