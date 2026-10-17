-- 真实人工配置生产的不可变价目；无默认金额，不建立额度、账本或报价缓存。
CREATE TABLE price_book_entries (
    price_snapshot_id TEXT PRIMARY KEY,
    model_id TEXT NOT NULL REFERENCES models(model_id),
    model_version INTEGER NOT NULL CHECK(model_version BETWEEN 1 AND 9007199254740991),
    data_epoch TEXT NOT NULL,
    source TEXT NOT NULL CHECK(source IN ('manual','remote','bundled')),
    effective_seconds INTEGER NOT NULL,
    effective_nanos INTEGER NOT NULL CHECK(effective_nanos BETWEEN 0 AND 999999999),
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    snapshot_hash TEXT NOT NULL,
    UNIQUE(model_id,data_epoch,source,model_version)
);
CREATE INDEX price_book_model_effective ON price_book_entries(model_id,data_epoch,source,effective_seconds,effective_nanos);
