-- jobs 域初始权威表。nw_outbox 由 libs/platform/outbox 在同一业务库中幂等建立。
-- 本迁移只保存任务、租约、进度、事件、投递、定时和屏障元数据，绝不保存业务正文。
CREATE TABLE jobs (
    job_id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    type TEXT NOT NULL,
    immutable_input_ref BLOB NOT NULL,
    input_hash TEXT NOT NULL,
    dedupe_key TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'leased', 'running', 'retry-wait', 'dead-letter', 'cancel-requested', 'cancelled', 'succeeded')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    not_before TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_jobs_owner_epoch_dedupe ON jobs (owner, data_epoch, dedupe_key);
CREATE INDEX idx_jobs_claimable ON jobs (state, not_before, created_at, job_id);

CREATE TABLE leases (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    worker_id TEXT NOT NULL,
    fencing_token INTEGER NOT NULL CHECK (fencing_token > 0),
    expires_at TEXT NOT NULL,
    last_heartbeat TEXT NOT NULL
);

CREATE TABLE progress (
    job_id TEXT NOT NULL REFERENCES jobs(job_id),
    seq INTEGER NOT NULL CHECK (seq >= 0),
    phase TEXT NOT NULL,
    completed_units INTEGER NOT NULL CHECK (completed_units >= 0),
    total_units INTEGER NOT NULL CHECK (total_units >= 0),
    safe_message_code TEXT NOT NULL,
    PRIMARY KEY (job_id, seq)
);

CREATE TABLE job_results (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    result_ref TEXT NOT NULL,
    paid_result_refs_json BLOB NOT NULL,
    result_hash TEXT NOT NULL
);

CREATE TABLE events (
    event_id TEXT PRIMARY KEY,
    event_seq INTEGER NOT NULL UNIQUE,
    envelope_json BLOB NOT NULL,
    source_revision INTEGER NOT NULL CHECK (source_revision >= 0),
    data_epoch TEXT NOT NULL
);

CREATE TABLE deliveries (
    event_id TEXT NOT NULL REFERENCES events(event_id),
    consumer_id TEXT NOT NULL,
    state TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    retry_at TEXT NOT NULL,
    PRIMARY KEY (event_id, consumer_id)
);

CREATE TABLE schedules (
    schedule_id TEXT PRIMARY KEY,
    owner TEXT NOT NULL,
    cron TEXT NOT NULL,
    timezone TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    next_fire_at TEXT NOT NULL,
    last_tick TEXT NOT NULL
);

CREATE TABLE barriers (
    barrier_id TEXT PRIMARY KEY,
    state TEXT NOT NULL,
    domain_status_json BLOB NOT NULL,
    data_epoch TEXT NOT NULL,
    deadline TEXT NOT NULL
);

CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
