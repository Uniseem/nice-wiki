-- 独立列表水位不是引用来源水位或删除许可；任何资产/对象generation变化均在原事务推进。
CREATE TABLE asset_listing_state (
    singleton INTEGER PRIMARY KEY CHECK(singleton=1),
    revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1)
);
INSERT INTO asset_listing_state(singleton,revision) VALUES(1,1);
CREATE INDEX idx_assets_listing_order ON assets(site_id,json_extract(asset_json,'$.kind'),json_extract(asset_json,'$.createdAt'),asset_id);

CREATE TRIGGER trg_listing_assets_insert AFTER INSERT ON assets
BEGIN
    SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM asset_listing_state WHERE singleton=1)
        THEN RAISE(ABORT,'media_listing_revision_missing') END;
    UPDATE asset_listing_state SET revision=CASE WHEN revision<9223372036854775807
        THEN revision+1 ELSE RAISE(ABORT,'media_listing_revision_exhausted') END WHERE singleton=1;
END;

CREATE TRIGGER trg_listing_assets_update AFTER UPDATE ON assets
BEGIN
    SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM asset_listing_state WHERE singleton=1)
        THEN RAISE(ABORT,'media_listing_revision_missing') END;
    UPDATE asset_listing_state SET revision=CASE WHEN revision<9223372036854775807
        THEN revision+1 ELSE RAISE(ABORT,'media_listing_revision_exhausted') END WHERE singleton=1;
END;

CREATE TRIGGER trg_listing_assets_delete AFTER DELETE ON assets
BEGIN
    SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM asset_listing_state WHERE singleton=1)
        THEN RAISE(ABORT,'media_listing_revision_missing') END;
    UPDATE asset_listing_state SET revision=CASE WHEN revision<9223372036854775807
        THEN revision+1 ELSE RAISE(ABORT,'media_listing_revision_exhausted') END WHERE singleton=1;
END;

CREATE TRIGGER trg_listing_reference_objects_insert AFTER INSERT ON reference_objects
BEGIN
    SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM asset_listing_state WHERE singleton=1)
        THEN RAISE(ABORT,'media_listing_revision_missing') END;
    UPDATE asset_listing_state SET revision=CASE WHEN revision<9223372036854775807
        THEN revision+1 ELSE RAISE(ABORT,'media_listing_revision_exhausted') END WHERE singleton=1;
END;

CREATE TRIGGER trg_listing_reference_objects_update AFTER UPDATE ON reference_objects
BEGIN
    SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM asset_listing_state WHERE singleton=1)
        THEN RAISE(ABORT,'media_listing_revision_missing') END;
    UPDATE asset_listing_state SET revision=CASE WHEN revision<9223372036854775807
        THEN revision+1 ELSE RAISE(ABORT,'media_listing_revision_exhausted') END WHERE singleton=1;
END;

CREATE TRIGGER trg_listing_reference_objects_delete AFTER DELETE ON reference_objects
BEGIN
    SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM asset_listing_state WHERE singleton=1)
        THEN RAISE(ABORT,'media_listing_revision_missing') END;
    UPDATE asset_listing_state SET revision=CASE WHEN revision<9223372036854775807
        THEN revision+1 ELSE RAISE(ABORT,'media_listing_revision_exhausted') END WHERE singleton=1;
END;
