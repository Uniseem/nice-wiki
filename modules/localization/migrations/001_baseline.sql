-- 基线迁移：由历史迁移重放得到的最终结构（001_baseline.sql 至 001_baseline.sql）。
CREATE TABLE languages (
    language_tag                         TEXT PRIMARY KEY,
    version                              INTEGER NOT NULL CHECK (version >= 0),
    enabled                              INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    default_language                     INTEGER NOT NULL CHECK (default_language IN (0, 1)),
    translation_profile                  TEXT,
    automatic_translation                INTEGER NOT NULL CHECK (automatic_translation IN (0, 1)),
    direction                            TEXT NOT NULL CHECK (direction IN ('ltr', 'rtl')),
    reflection_enabled                   INTEGER NOT NULL CHECK (reflection_enabled IN (0, 1)),
    batch_output_tokens                  INTEGER NOT NULL CHECK (batch_output_tokens BETWEEN 100 AND 8192),
    translation_memory_minimum_characters INTEGER NOT NULL CHECK (translation_memory_minimum_characters BETWEEN 1 AND 1000),
    data_epoch                           TEXT NOT NULL,
    created_at                           TEXT NOT NULL,
    updated_at                           TEXT NOT NULL
);
CREATE TABLE translation_jobs (
    job_id                  TEXT PRIMARY KEY,
    document_ref             TEXT NOT NULL,
    target_language          TEXT NOT NULL,
    state                    TEXT NOT NULL CHECK (state IN ('queued', 'running', 'review', 'ready', 'superseded', 'failed', 'cancelled', 'stale')),
    source_generation        INTEGER NOT NULL CHECK (source_generation >= 0),
    glossary_generation      INTEGER NOT NULL CHECK (glossary_generation >= 0),
    human_generation         INTEGER NOT NULL CHECK (human_generation >= 0),
    memory_version_hash      TEXT NOT NULL,
    lease_id                 TEXT,
    lease_expires_at         TEXT,
    data_epoch               TEXT NOT NULL,
    created_at               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
);
CREATE TABLE translation_manifests (
    translation_id           TEXT PRIMARY KEY,
    publication_ref          TEXT NOT NULL,
    target_language          TEXT NOT NULL,
    generation               INTEGER NOT NULL CHECK (generation >= 0),
    memory_hash              TEXT NOT NULL,
    glossary_generation      INTEGER NOT NULL CHECK (glossary_generation >= 0),
    state                    TEXT NOT NULL CHECK (state IN ('queued', 'running', 'review', 'ready', 'superseded', 'failed', 'cancelled', 'stale')),
    data_epoch               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
, file_ref TEXT);
CREATE TABLE glossary (
    term_id                  TEXT PRIMARY KEY,
    source_language          TEXT NOT NULL,
    target_language          TEXT NOT NULL,
    source_term              TEXT NOT NULL,
    target_term              TEXT NOT NULL,
    scope                    TEXT NOT NULL CHECK (scope IN ('global', 'document')),
    document_id              TEXT,
    version                  INTEGER NOT NULL CHECK (version >= 0),
    generation               INTEGER NOT NULL CHECK (generation >= 0),
    source                   TEXT NOT NULL CHECK (source IN ('manual', 'auto')),
    extraction_batch_id      TEXT,
    data_epoch               TEXT NOT NULL,
    created_at               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
, deleted INTEGER NOT NULL DEFAULT 0 CHECK(deleted IN(0,1)));
CREATE TABLE interface_catalog (
    language                 TEXT NOT NULL,
    key                      TEXT NOT NULL,
    source_hash              TEXT NOT NULL,
    value_file_ref           TEXT,
    value_text               TEXT NOT NULL,
    placeholders             TEXT NOT NULL,
    generation               INTEGER NOT NULL CHECK (generation >= 0),
    origin                   TEXT NOT NULL CHECK (origin IN ('builtin', 'machine', 'human')),
    protected                INTEGER NOT NULL CHECK (protected IN (0, 1)),
    version                  INTEGER NOT NULL CHECK (version >= 0),
    data_epoch               TEXT NOT NULL,
    updated_at               TEXT NOT NULL,
    PRIMARY KEY (language, key)
);
CREATE TABLE language_preferences (
    visitor_subject          TEXT PRIMARY KEY,
    language                 TEXT NOT NULL,
    expires_at               TEXT NOT NULL
);
CREATE TABLE approved_proposals (
    proposal_id              TEXT PRIMARY KEY,
    proposal_hash            TEXT NOT NULL,
    state                    TEXT NOT NULL CHECK (state IN ('accepted', 'applied', 'rejected')),
    term_id                  TEXT,
    approved_by              TEXT NOT NULL,
    approval_revision        INTEGER NOT NULL CHECK (approval_revision >= 0),
    expected_glossary_generation INTEGER NOT NULL CHECK (expected_glossary_generation >= 0),
    term_json                TEXT NOT NULL,
    data_epoch               TEXT NOT NULL,
    created_at               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
, site_id TEXT NOT NULL DEFAULT '', root_operation_id TEXT NOT NULL DEFAULT '', result_generation INTEGER NOT NULL DEFAULT 0);
CREATE TABLE schema_migrations (
    version                    INTEGER PRIMARY KEY,
    name                       TEXT NOT NULL,
    applied_at                 TEXT NOT NULL
);
CREATE TABLE operation_receipts (
    site_id          TEXT NOT NULL,
    actor_id         TEXT NOT NULL,
    function_id      TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    data_epoch       TEXT NOT NULL,
    input_hash       TEXT NOT NULL,
    result_json      TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    PRIMARY KEY (site_id, actor_id, function_id, idempotency_key)
);
CREATE TABLE runtime_meta (
    singleton   INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE TABLE local_barrier (
    singleton   INTEGER PRIMARY KEY CHECK (singleton = 1),
    state       TEXT NOT NULL CHECK (state IN ('draining', 'safe', 'released', 'failed')),
    data_epoch  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE TABLE domain_versions (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),glossary_generation INTEGER NOT NULL,catalog_generation INTEGER NOT NULL
);
CREATE TABLE interface_sources (
 key TEXT PRIMARY KEY,value_text TEXT NOT NULL,source_hash TEXT NOT NULL,placeholders TEXT NOT NULL
);
CREATE TABLE translation_units (
 unit_id TEXT NOT NULL,translation_id TEXT NOT NULL,document_id TEXT NOT NULL,
 source_hash TEXT NOT NULL,source_text TEXT NOT NULL,target_language TEXT NOT NULL,
 target_file_ref TEXT,target_text TEXT NOT NULL,target_hash TEXT NOT NULL,
 origin TEXT NOT NULL CHECK(origin IN('machine','human')),protected INTEGER NOT NULL CHECK(protected IN(0,1)),
 version INTEGER NOT NULL CHECK(version>=0),human_generation INTEGER NOT NULL CHECK(human_generation>=0),
 status TEXT NOT NULL CHECK(status IN('normal','review','fallback')),previous_target_text TEXT,
 data_epoch TEXT NOT NULL,updated_at TEXT NOT NULL,PRIMARY KEY(translation_id,unit_id)
);
CREATE TABLE suggestion_approvals (
 approval_id TEXT PRIMARY KEY,
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 approved_at TEXT NOT NULL,
 approved_by TEXT NOT NULL,
 permission_revision INTEGER NOT NULL,
 root_operation_id TEXT NOT NULL,
 UNIQUE(site_id,data_epoch,version)
);
CREATE TABLE suggestion_policy (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 approval_id TEXT NOT NULL REFERENCES suggestion_approvals(approval_id),
 PRIMARY KEY(site_id,data_epoch)
);
CREATE TABLE language_plans (
    plan_id              TEXT PRIMARY KEY,
    language             TEXT NOT NULL,
    site_id              TEXT NOT NULL,
    data_epoch           TEXT NOT NULL,
    actor_id             TEXT NOT NULL,
    operation_id         TEXT NOT NULL,
    snapshot_json        TEXT NOT NULL,
    snapshot_hash        TEXT NOT NULL,
    result_json          TEXT NOT NULL,
    quote_hash           TEXT NOT NULL,
    expires_at           TEXT NOT NULL,
    state                TEXT NOT NULL CHECK (state IN ('pending', 'approved', 'superseded')),
    approval_started_at  TEXT,
    approval_operation_json TEXT,
    approval_root_operation_id TEXT,
    approval_actor_id    TEXT,
    approval_deadline    TEXT,
    approved_at          TEXT,
    approved_by          TEXT,
    approved_result_json TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);
CREATE TABLE language_plan_documents (
    plan_id         TEXT NOT NULL REFERENCES language_plans(plan_id) ON DELETE CASCADE,
    document_id     TEXT NOT NULL,
    reference_json  TEXT NOT NULL,
    reference_hash  TEXT NOT NULL,
    unit_count      INTEGER NOT NULL CHECK (unit_count >= 0),
    input_tokens    INTEGER NOT NULL CHECK (input_tokens >= 0),
    PRIMARY KEY (plan_id, document_id)
);
CREATE TABLE language_plan_jobs (
    plan_id         TEXT NOT NULL REFERENCES language_plans(plan_id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('document', 'ui')),
    document_id     TEXT NOT NULL,
    input_ref_json  TEXT NOT NULL,
    input_ref_hash  TEXT NOT NULL,
    input_json      TEXT NOT NULL,
    input_hash      TEXT NOT NULL,
    dedupe_key      TEXT NOT NULL,
    state           TEXT NOT NULL CHECK (state IN ('pending', 'accepted')),
    job_id          TEXT,
    operation_id    TEXT,
    accepted_at     TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    PRIMARY KEY (plan_id, kind, document_id)
);
CREATE TABLE language_suggestions (
 cache_key TEXT PRIMARY KEY,
 data_epoch TEXT NOT NULL,
 language TEXT NOT NULL,
 confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
 created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_languages_single_default
    ON languages (default_language) WHERE default_language = 1;
CREATE INDEX idx_translation_jobs_state
    ON translation_jobs (state, target_language);
CREATE INDEX idx_glossary_lookup
    ON glossary (source_language, target_language, source_term, generation);
CREATE UNIQUE INDEX idx_glossary_active_scope ON glossary(source_language,target_language,scope,COALESCE(document_id,''),source_term) WHERE deleted=0;
CREATE INDEX idx_translation_units_translation ON translation_units(translation_id,unit_id);
CREATE UNIQUE INDEX idx_language_plans_active
    ON language_plans (site_id, data_epoch, language)
    WHERE state = 'pending';
CREATE INDEX idx_language_plans_expiry
    ON language_plans (site_id, data_epoch, language, expires_at);
CREATE INDEX idx_language_plan_jobs_state
    ON language_plan_jobs (state, plan_id);
CREATE TRIGGER suggestion_approvals_no_update BEFORE UPDATE ON suggestion_approvals
BEGIN SELECT RAISE(ABORT,'immutable suggestion approval'); END;
CREATE TRIGGER suggestion_approvals_no_delete BEFORE DELETE ON suggestion_approvals
BEGIN SELECT RAISE(ABORT,'immutable suggestion approval'); END;
INSERT INTO "runtime_meta"("singleton","data_epoch","updated_at") VALUES(1,'','');
INSERT INTO "local_barrier"("singleton","state","data_epoch","updated_at") VALUES(1,'released','','');
INSERT INTO "domain_versions"("singleton","glossary_generation","catalog_generation") VALUES(1,0,0);
