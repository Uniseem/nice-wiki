-- 元数据成功操作的完整同代原结果；与真实metadata/tree CAS及outbox同事务追加。
-- 不存正文，不建影子head/树或未获准的分享图来源/pin。
CREATE TABLE metadata_operation_receipts (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK(json_valid(result_json)),
    result_hash TEXT NOT NULL,
    PRIMARY KEY(site_id,data_epoch,operation_id)
);
