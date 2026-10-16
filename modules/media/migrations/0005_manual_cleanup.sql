-- 计划、精确扫描材料和原回执均为权威记录；只有显式人工确认能标记delete_pending。
CREATE TABLE manual_cleanup_plans (
    plan_id TEXT PRIMARY KEY,
    data_epoch TEXT NOT NULL,
    site_id TEXT NOT NULL,
    created_by TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK(revision>=0),
    state TEXT NOT NULL CHECK(state IN ('proposed','confirmed','waiting','cancelled','deleted','blocked')),
    confirmation_hash TEXT NOT NULL,
    plan_json TEXT NOT NULL CHECK(json_valid(plan_json)),
    plan_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE manual_cleanup_assets (
    plan_id TEXT NOT NULL REFERENCES manual_cleanup_plans(plan_id),
    asset_id TEXT NOT NULL REFERENCES reference_objects(asset_id),
    deletion_fence INTEGER NOT NULL CHECK(deletion_fence>=0),
    reference_generation INTEGER NOT NULL CHECK(reference_generation>=0),
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    snapshot_hash TEXT NOT NULL,
    PRIMARY KEY(plan_id,asset_id)
);
CREATE INDEX idx_manual_cleanup_assets_asset ON manual_cleanup_assets(asset_id,plan_id);
-- 后续确认必须同事务写标记、delete_pending及当次fence；未知标记不能授权取消或删除。
CREATE TABLE manual_cleanup_marks (
    asset_id TEXT PRIMARY KEY REFERENCES reference_objects(asset_id),
    plan_id TEXT NOT NULL,
    deletion_fence INTEGER NOT NULL CHECK(deletion_fence>=0),
    reference_generation INTEGER NOT NULL CHECK(reference_generation>=0),
    FOREIGN KEY(plan_id,asset_id) REFERENCES manual_cleanup_assets(plan_id,asset_id)
);
CREATE TABLE manual_cleanup_receipts (
    operation_id TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('propose','cancel')),
    plan_id TEXT NOT NULL REFERENCES manual_cleanup_plans(plan_id),
    result_json TEXT NOT NULL CHECK(json_valid(result_json)),
    result_hash TEXT NOT NULL,
    created_at TEXT NOT NULL
);
