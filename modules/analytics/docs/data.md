# analytics 数据与持久化

未来私有目录 `data/analytics/active/` 保存SQLite和领域对象；`data/analytics/staging/<restoreId>/` 为隔离导入区；`data/analytics/control/` 保存不被业务恢复替换的barrier/recovery授权信任锚和操作journal。外域不得直接读这些路径。

- `beacon_inbox(beacon_id PK, document_id, locale, event_type, status_class, elapsed_bucket, daily_visitor_hash, source_domain, country_code, occurred_at, data_epoch；不存IP/UA原文，保留30天)`

- `hourly_page_counts(hour, document_id, locale, views, visits, load_bucket_counts, data_epoch; PK hour/document_id/locale)`

- `audit_records(audit_id PK, event_id UNIQUE, actor_id, actor_kind, action, resource_ref, result_code, occurred_at, hash, previous_hash, data_epoch)`

- `usage_projection(ledger_entry_id PK, ledger_revision, operation_id, cost_class, provider_id, model_id, amount_micros, currency, state, occurred_at, data_epoch)`

- `daily_usage(day, cost_class, provider_id, model_id, amount_micros, entry_count, data_epoch; PK day/class/provider/model)`

- `event_inbox(event_id PK, source_revision, data_epoch)`

- `aggregation_watermarks(source, revision, updated_at)`

- `export_runs(export_id PK, filter_hash, actor_id, object_ref, expires_at, state)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP六管理协议，在safe屏障固定revisionWatermark；导入全域校验通过后prepare，持久commit决定后切代，全部域activate才解维护。当前恢复control记录不得由导入覆盖。迁移先隔离拷贝、schema校验、SQL迁移、完整性验证，失败保留旧active指针。
