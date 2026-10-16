-- 来源材料与原 pin 回执属于权威业务记录，不从事件索引推断。
-- sealed 可没有 pin，后续明确源终止时仍须永久封存完整材料。
CREATE TABLE reference_sources (
    source_service TEXT NOT NULL CHECK(source_service IN ('content','site-config')),
    data_epoch TEXT NOT NULL,
    source_operation_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    source_object_id TEXT NOT NULL,
    source_revision_id TEXT NOT NULL,
    source_version INTEGER NOT NULL CHECK(source_version>=0),
    reference_digest TEXT NOT NULL,
    binding_hash TEXT NOT NULL,
    input_json TEXT CHECK(input_json IS NULL OR json_valid(input_json)),
    state TEXT NOT NULL CHECK(state IN ('pending','confirmed','sealed')),
    sealed_reason TEXT CHECK(sealed_reason IN ('source_aborted','source_rejected')),
    pin_id TEXT UNIQUE,
    expires_at TEXT,
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    PRIMARY KEY(source_service,data_epoch,source_operation_id),
    CHECK((state='sealed' AND sealed_reason IS NOT NULL) OR
          (state IN ('pending','confirmed') AND sealed_reason IS NULL AND pin_id IS NOT NULL AND input_json IS NOT NULL)),
    CHECK((pin_id IS NULL AND expires_at IS NULL AND result_json IS NULL AND result_hash IS NULL) OR
          (pin_id IS NOT NULL AND expires_at IS NOT NULL AND result_json IS NOT NULL AND result_hash IS NOT NULL))
);
CREATE INDEX idx_reference_sources_object
    ON reference_sources(source_service,data_epoch,site_id,source_object_id,source_version);

-- ready 资源删除与所有新增 pin 共用本表 fence。只在迁移时为旧资产回填；
-- 运行时不存在的 fence 不补造，否则可能掩盖已持久删除决定丢失。
CREATE TABLE reference_objects (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id),
    deletion_fence INTEGER NOT NULL CHECK(deletion_fence>=0),
    reference_generation INTEGER NOT NULL CHECK(reference_generation>=0)
);
INSERT INTO reference_objects(asset_id,deletion_fence,reference_generation)
    SELECT asset_id,0,0 FROM assets;

CREATE TABLE reference_pin_assets (
    pin_id TEXT NOT NULL REFERENCES reference_sources(pin_id),
    asset_id TEXT NOT NULL REFERENCES reference_objects(asset_id),
    deletion_fence INTEGER NOT NULL CHECK(deletion_fence>0),
    PRIMARY KEY(pin_id,asset_id)
);
CREATE INDEX idx_reference_pin_assets_asset ON reference_pin_assets(asset_id,pin_id);
