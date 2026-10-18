# data-protection 数据与持久化

未来私有目录 `data/data-protection/active/` 保存SQLite和领域对象；`data/data-protection/restore/<restoreId>/staged|before/` 为恢复暂存与换出的旧数据；`data/data-protection/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `backup_runs(backup_id PK, source_epoch, barrier_id, status, manifest_hash, encrypted_manifest_ref, key_id, created_at, completed_at)`

- `backup_objects(backup_id, owner, snapshot_id, schema_version, watermark, object_hash, encrypted_ref; PK backup_id/owner)`

- `restore_runs(restore_id PK, backup_id, source_epoch, target_epoch, barrier_id, status, runtime_revision, committed_at)`

- `restore_decisions(decision_id PK, restore_id UNIQUE, target_epoch, decision(switch/abort/rollback), staged_manifest_hash, decided_at)`，位于control区，不随恢复替换

- `domain_stages(restore_id, owner, snapshot_id, snapshot_hash, state(staged/switched/rolled-back/discarded), last_error_code; PK restore_id/owner)`

- `mirror_runs(mirror_id PK, backup_id, manifest_hash, local_commit, remote_ref, state)`

- `remote_config(config_id PK, url_without_credentials, branch, credential_ref, revision)`

- `outbox(event_id PK, payload_ref, state, retry_at)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP管理协议（barrier/export/stage/discard），在safe屏障固定revisionWatermark；恢复先全域暂存，写切换决定后各服务重启换入，全部域登记回执才解维护。control区不随恢复替换。换入后先schema迁移与完整性验证，失败拒绝启动并可整体换回。

R1实现收口：active/protection.db保存备份/对象/镜像目录、remote设置与目录revision；control/protection-control.db保存恢复运行、决定、域回执、屏障及journal，整体不参与业务恢复。各库user_version=1，未知更高版本拒绝启动。实际迁移0001_domain.sql与0002_control.sql在SQLite短事务应用；平台inbox/outbox直接EnsureSchema，不复制结构。对象只允许本域active/incoming、active/objects及restore隔离根，不能读取其他域私有路径。

目录分页由created_at/backup_id倒序keyset和目录revision绑定，平台签名游标固定稳定身份/会话/权限修订/epoch/limit，10分钟过期；水位变化拒绝混页。未配置镜像或不存在备份返回NOT_FOUND，不用空成功伪造历史。
