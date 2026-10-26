-- 基线迁移：由历史迁移重放得到的最终结构（0001_domain.sql 至 0011_library_groups.sql）。
CREATE TABLE upload_sessions (
    upload_id      TEXT PRIMARY KEY,
    site_id        TEXT NOT NULL,
    created_by     TEXT NOT NULL,
    state          TEXT NOT NULL,
    expected_bytes INTEGER NOT NULL CHECK (expected_bytes > 0),
    session_json   TEXT NOT NULL CHECK (json_valid(session_json)),
    updated_at     TEXT NOT NULL
, data_epoch TEXT);
CREATE TABLE upload_receipts (
    operation_id TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    upload_id    TEXT NOT NULL REFERENCES upload_sessions (upload_id),
    created_at   TEXT NOT NULL
, result_json TEXT, result_hash TEXT);
CREATE TABLE assets (
    asset_id   TEXT PRIMARY KEY,
    site_id    TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    byte_size  INTEGER NOT NULL CHECK (byte_size > 0),
    media_type TEXT NOT NULL,
    state      TEXT NOT NULL,
    asset_json TEXT NOT NULL CHECK (json_valid(asset_json)),
    updated_at TEXT NOT NULL
);
CREATE TABLE blob_journal (
    seq         INTEGER PRIMARY KEY AUTOINCREMENT,
    event       TEXT NOT NULL CHECK (event IN ('PREPARED', 'COMMITTED', 'DELETING')),
    site_id     TEXT NOT NULL,
    upload_id   TEXT NOT NULL,
    sha256      TEXT NOT NULL,
    asset_id    TEXT,
    occurred_at TEXT NOT NULL
);
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE upload_chunks (
    operation_id TEXT PRIMARY KEY,
    upload_id TEXT NOT NULL REFERENCES upload_sessions(upload_id),
    site_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    expected_revision INTEGER NOT NULL CHECK(expected_revision>=0),
    byte_offset INTEGER NOT NULL CHECK(byte_offset>=0),
    byte_length INTEGER NOT NULL CHECK(byte_length BETWEEN 1 AND 8388608),
    sha256 TEXT NOT NULL,
    stage_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('PREPARED','COMMITTED')),
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    CHECK((state='PREPARED' AND result_json IS NULL AND result_hash IS NULL) OR
          (state='COMMITTED' AND result_json IS NOT NULL AND result_hash IS NOT NULL))
);
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
    created_at TEXT NOT NULL, resolution_json TEXT CHECK(resolution_json IS NULL OR json_valid(resolution_json)), resolution_hash TEXT, resolved_at TEXT,
    PRIMARY KEY(source_service,data_epoch,source_operation_id),
    CHECK((state='sealed' AND sealed_reason IS NOT NULL) OR
          (state IN ('pending','confirmed') AND sealed_reason IS NULL AND pin_id IS NOT NULL AND input_json IS NOT NULL)),
    CHECK((pin_id IS NULL AND expires_at IS NULL AND result_json IS NULL AND result_hash IS NULL) OR
          (pin_id IS NOT NULL AND expires_at IS NOT NULL AND result_json IS NOT NULL AND result_hash IS NOT NULL))
);
CREATE TABLE reference_objects (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id),
    deletion_fence INTEGER NOT NULL CHECK(deletion_fence>=0),
    reference_generation INTEGER NOT NULL CHECK(reference_generation>=0)
);
CREATE TABLE reference_pin_assets (
    pin_id TEXT NOT NULL REFERENCES reference_sources(pin_id),
    asset_id TEXT NOT NULL REFERENCES reference_objects(asset_id),
    deletion_fence INTEGER NOT NULL CHECK(deletion_fence>0),
    PRIMARY KEY(pin_id,asset_id)
);
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
CREATE TABLE reference_entries (
    pin_id TEXT NOT NULL REFERENCES reference_sources(pin_id),
    reference_id TEXT NOT NULL,
    asset_id TEXT NOT NULL REFERENCES reference_objects(asset_id),
    kind TEXT NOT NULL CHECK(kind IN ('current','history')),
    PRIMARY KEY(pin_id,reference_id,asset_id)
);
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
CREATE TABLE manual_cleanup_approvals (
    plan_id TEXT PRIMARY KEY REFERENCES manual_cleanup_plans(plan_id),
    operation_id TEXT NOT NULL UNIQUE,
    request_hash TEXT NOT NULL,
    approval_json TEXT NOT NULL CHECK(json_valid(approval_json)),
    approval_hash TEXT NOT NULL,
    job_input_json TEXT NOT NULL CHECK(json_valid(job_input_json)),
    job_input_hash TEXT NOT NULL,
    job_receipt_json TEXT CHECK(job_receipt_json IS NULL OR json_valid(job_receipt_json)),
    job_receipt_hash TEXT,
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK((job_receipt_json IS NULL)=(job_receipt_hash IS NULL)),
    CHECK((result_json IS NULL)=(result_hash IS NULL)),
    CHECK(result_json IS NULL OR job_receipt_json IS NOT NULL)
);
CREATE TABLE manual_cleanup_executions (
    plan_id TEXT PRIMARY KEY REFERENCES manual_cleanup_approvals(plan_id),
    approval_hash TEXT NOT NULL,
    job_id TEXT NOT NULL UNIQUE,
    execution_json TEXT NOT NULL CHECK(json_valid(execution_json)),
    execution_hash TEXT NOT NULL,
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    CHECK((result_json IS NULL)=(result_hash IS NULL))
);
CREATE TABLE manual_cleanup_deletions (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id),
    plan_id TEXT NOT NULL REFERENCES manual_cleanup_executions(plan_id),
    decision_json TEXT NOT NULL CHECK(json_valid(decision_json)),
    decision_hash TEXT NOT NULL,
    UNIQUE(plan_id,asset_id)
);
CREATE TABLE asset_reader_leases (
    reader_id TEXT PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id),
    data_epoch TEXT NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE asset_listing_state (
    singleton INTEGER PRIMARY KEY CHECK(singleton=1),
    revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1)
);
CREATE TABLE upload_expirations (
 upload_id TEXT PRIMARY KEY REFERENCES upload_sessions(upload_id),
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('prepared','removed')),
 decided_at TEXT NOT NULL,
 removed_at TEXT
);
CREATE TABLE asset_variants (
 parent_asset_id TEXT NOT NULL,
 variant_asset_id TEXT NOT NULL,
 PRIMARY KEY(parent_asset_id,variant_asset_id)
);
CREATE TABLE public_assets (
 asset_id TEXT NOT NULL,
 sha256 TEXT NOT NULL,
 first_public_at TEXT NOT NULL,
 PRIMARY KEY(asset_id,sha256)
);
CREATE TABLE asset_groups(group_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,name TEXT NOT NULL,created_at TEXT NOT NULL,UNIQUE(site_id,name));
CREATE INDEX idx_upload_sessions_site_state
    ON upload_sessions (site_id, state);
CREATE INDEX idx_assets_site_sha
    ON assets (site_id, sha256, state);
CREATE INDEX idx_blob_journal_key
    ON blob_journal (site_id, upload_id, sha256, seq);
CREATE INDEX idx_upload_chunks_upload ON upload_chunks(upload_id,state,byte_offset);
CREATE UNIQUE INDEX idx_upload_chunks_committed_revision ON upload_chunks(upload_id,expected_revision) WHERE state='COMMITTED';
CREATE UNIQUE INDEX idx_upload_chunks_committed_offset ON upload_chunks(upload_id,byte_offset) WHERE state='COMMITTED';
CREATE INDEX idx_reference_sources_object
    ON reference_sources(source_service,data_epoch,site_id,source_object_id,source_version);
CREATE INDEX idx_reference_pin_assets_asset ON reference_pin_assets(asset_id,pin_id);
CREATE INDEX idx_reference_entries_asset_kind ON reference_entries(asset_id,kind);
CREATE INDEX idx_manual_cleanup_assets_asset ON manual_cleanup_assets(asset_id,plan_id);
CREATE INDEX idx_manual_cleanup_deletions_plan ON manual_cleanup_deletions(plan_id,asset_id);
CREATE INDEX idx_asset_reader_leases_asset ON asset_reader_leases(asset_id,expires_at_ns);
CREATE INDEX idx_assets_listing_order ON assets(site_id,json_extract(asset_json,'$.kind'),json_extract(asset_json,'$.createdAt'),asset_id);
CREATE INDEX idx_asset_variants_variant ON asset_variants(variant_asset_id);
CREATE INDEX asset_group_members ON assets(site_id,json_extract(asset_json,'$.groupId'));
CREATE VIEW asset_current_references AS
 SELECT asset_id FROM reference_entries WHERE kind='current'
 UNION ALL
 SELECT v.variant_asset_id FROM asset_variants v JOIN reference_entries e ON e.asset_id=v.parent_asset_id WHERE e.kind='current';
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
INSERT INTO "asset_listing_state"("singleton","revision") VALUES(1,1);
PRAGMA user_version=11;
