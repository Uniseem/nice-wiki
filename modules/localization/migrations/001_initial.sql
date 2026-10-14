-- localization 域初始权威表。
-- nw_inbox/nw_outbox 由 libs/platform 在启动时幂等建立，不在迁移中复制。

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

CREATE UNIQUE INDEX idx_languages_single_default
    ON languages (default_language) WHERE default_language = 1;

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

CREATE INDEX idx_translation_jobs_state
    ON translation_jobs (state, target_language);

CREATE TABLE translation_units (
    unit_id                  TEXT PRIMARY KEY,
    translation_id           TEXT NOT NULL,
    document_id              TEXT NOT NULL,
    source_hash              TEXT NOT NULL,
    source_text              TEXT NOT NULL,
    target_language          TEXT NOT NULL,
    target_file_ref          TEXT,
    target_text              TEXT NOT NULL,
    target_hash              TEXT NOT NULL,
    origin                   TEXT NOT NULL CHECK (origin IN ('machine', 'human')),
    protected                INTEGER NOT NULL CHECK (protected IN (0, 1)),
    version                  INTEGER NOT NULL CHECK (version >= 0),
    human_generation         INTEGER NOT NULL CHECK (human_generation >= 0),
    status                   TEXT NOT NULL CHECK (status IN ('normal', 'review', 'fallback')),
    previous_target_text     TEXT,
    data_epoch               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
);

CREATE INDEX idx_translation_units_translation
    ON translation_units (translation_id, unit_id);

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
);

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
);

CREATE INDEX idx_glossary_lookup
    ON glossary (source_language, target_language, source_term, generation);

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
);

CREATE TABLE cache_answer_translations (
    translation_id           TEXT PRIMARY KEY,
    cache_id                 TEXT NOT NULL,
    answer_revision          INTEGER NOT NULL CHECK (answer_revision >= 0),
    source_hash              TEXT NOT NULL,
    target_language          TEXT NOT NULL,
    translated_question      TEXT NOT NULL,
    translated_answer        TEXT NOT NULL,
    state                    TEXT NOT NULL CHECK (state IN ('queued', 'running', 'ready', 'failed', 'cancelled', 'superseded')),
    data_epoch               TEXT NOT NULL,
    updated_at               TEXT NOT NULL
);

CREATE INDEX idx_cache_answer_translations_source
    ON cache_answer_translations (cache_id, answer_revision, target_language);

CREATE TABLE schema_migrations (
    version                    INTEGER PRIMARY KEY,
    name                       TEXT NOT NULL,
    applied_at                 TEXT NOT NULL
);
