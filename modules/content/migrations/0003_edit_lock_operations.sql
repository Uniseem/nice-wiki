-- 仅保存编辑操作原结果、可靠通知序号与审计；重启不从这些记录重建内存租约。
-- 事件表定义仍由libs/platform/outbox.EnsureSchema建立，本迁移不复制平台机制。
CREATE TABLE edit_lock_receipts (
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    scope TEXT NOT NULL,
    document_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    result_hash TEXT NOT NULL,
    PRIMARY KEY (caller_service,actor_id,site_id,data_epoch,operation_id,scope)
);
CREATE TABLE edit_lock_event_revisions (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision BETWEEN 1 AND 9007199254740991),
    PRIMARY KEY (site_id,document_id)
);
CREATE TABLE edit_lock_audit (
    event_id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_kind TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    server_boot_id TEXT NOT NULL,
    fencing_token INTEGER NOT NULL,
    action TEXT NOT NULL,
    affected_account_id TEXT,
    occurred_at TEXT NOT NULL
);
