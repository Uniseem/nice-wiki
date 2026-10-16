-- 终局原结果与source状态必须由同一事务写入；旧缺结果不从mutable引用重建。
ALTER TABLE reference_sources ADD COLUMN resolution_json TEXT CHECK(resolution_json IS NULL OR json_valid(resolution_json));
ALTER TABLE reference_sources ADD COLUMN resolution_hash TEXT;
ALTER TABLE reference_sources ADD COLUMN resolved_at TEXT;

CREATE TABLE reference_heads (
    source_service TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    site_id TEXT NOT NULL,
    source_object_id TEXT NOT NULL,
    source_operation_id TEXT NOT NULL,
    source_revision_id TEXT NOT NULL,
    source_version INTEGER NOT NULL CHECK(source_version>=0),
    reference_digest TEXT NOT NULL,
    PRIMARY KEY(source_service,data_epoch,site_id,source_object_id),
    FOREIGN KEY(source_service,data_epoch,source_operation_id)
        REFERENCES reference_sources(source_service,data_epoch,source_operation_id)
);

-- 完整资源元数据和原pending材料永久保留在reference_sources.input_json。
-- 此表只维护已确认引用分类，不能把pending pin或sealed来源当作confirmed。
CREATE TABLE reference_entries (
    pin_id TEXT NOT NULL REFERENCES reference_sources(pin_id),
    reference_id TEXT NOT NULL,
    asset_id TEXT NOT NULL REFERENCES reference_objects(asset_id),
    kind TEXT NOT NULL CHECK(kind IN ('current','history')),
    PRIMARY KEY(pin_id,reference_id,asset_id)
);
CREATE INDEX idx_reference_entries_asset_kind ON reference_entries(asset_id,kind);
