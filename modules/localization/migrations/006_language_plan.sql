-- 语言启用计划、固定估价 revision 与持久任务意图。
-- 计划只在全部 jobs 接受后变为 approved；pending 状态保留未完成意图，重试复用同一输入。
CREATE TABLE language_plans (
    plan_id              TEXT PRIMARY KEY,
    language             TEXT NOT NULL,
    site_id              TEXT NOT NULL,
    data_epoch           TEXT NOT NULL,
    actor_id             TEXT NOT NULL,
    operation_id         TEXT NOT NULL,
    snapshot_json        BLOB NOT NULL,
    snapshot_hash        TEXT NOT NULL,
    result_json          BLOB NOT NULL,
    quote_hash           TEXT NOT NULL,
    expires_at           TEXT NOT NULL,
    state                TEXT NOT NULL CHECK (state IN ('pending', 'approved', 'superseded')),
    approval_started_at  TEXT,
    approval_operation_json BLOB,
    approval_root_operation_id TEXT,
    approval_actor_id    TEXT,
    approval_deadline    TEXT,
    approved_at          TEXT,
    approved_by          TEXT,
    approved_result_json BLOB,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_language_plans_active
    ON language_plans (site_id, data_epoch, language)
    WHERE state = 'pending';
CREATE INDEX idx_language_plans_expiry
    ON language_plans (site_id, data_epoch, language, expires_at);

CREATE TABLE language_plan_documents (
    plan_id         TEXT NOT NULL REFERENCES language_plans(plan_id) ON DELETE CASCADE,
    document_id     TEXT NOT NULL,
    reference_json  BLOB NOT NULL,
    reference_hash  TEXT NOT NULL,
    unit_count      INTEGER NOT NULL CHECK (unit_count >= 0),
    input_tokens    INTEGER NOT NULL CHECK (input_tokens >= 0),
    PRIMARY KEY (plan_id, document_id)
);

CREATE TABLE language_plan_jobs (
    plan_id         TEXT NOT NULL REFERENCES language_plans(plan_id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('document', 'ui')),
    document_id     TEXT NOT NULL,
    input_ref_json  BLOB NOT NULL,
    input_ref_hash  TEXT NOT NULL,
    input_json      BLOB NOT NULL,
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
CREATE INDEX idx_language_plan_jobs_state
    ON language_plan_jobs (state, plan_id);
