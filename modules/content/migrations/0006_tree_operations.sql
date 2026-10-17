-- 树操作只追加原回执；树与元数据仍由现有content_tree_state/documents权威表保存。
-- 完整scope、动作和规范摘要唯一绑定；仅业务成功短事务写入，不保存正文或影子树。
CREATE TABLE tree_operation_receipts (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('move','reorder')),
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    PRIMARY KEY (site_id,data_epoch,operation_id)
);
