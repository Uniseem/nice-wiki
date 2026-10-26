-- 基线迁移：由历史迁移重放得到的最终结构（0001_domain.sql 至 0009_job_context.sql）。
CREATE TABLE events (
    event_id TEXT PRIMARY KEY,
    event_seq INTEGER NOT NULL UNIQUE,
    envelope_json BLOB NOT NULL,
    source_revision INTEGER NOT NULL CHECK (source_revision >= 0),
    data_epoch TEXT NOT NULL
, producer TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL DEFAULT '', event_version INTEGER NOT NULL DEFAULT 0, subject_id TEXT NOT NULL DEFAULT '', envelope_hash TEXT NOT NULL DEFAULT '');
CREATE TABLE deliveries (
    event_id TEXT NOT NULL REFERENCES events(event_id),
    consumer_id TEXT NOT NULL,
    state TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    retry_at TEXT NOT NULL, delivery_id TEXT NOT NULL DEFAULT '', lease_expires_at TEXT, revision INTEGER NOT NULL DEFAULT 1 CHECK (revision>=1), last_error_code TEXT NOT NULL DEFAULT '',
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
, job_type TEXT NOT NULL DEFAULT '', input_ref TEXT NOT NULL DEFAULT '', data_epoch TEXT NOT NULL DEFAULT '');
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
CREATE TABLE lease_operations (
    owner TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('claim', 'heartbeat', 'complete', 'fail', 'cancel')),
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    PRIMARY KEY (owner, data_epoch, operation_kind, operation_id)
);
CREATE TABLE "jobs" (
    job_id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    root_operation_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    type TEXT NOT NULL,
    immutable_input_ref BLOB NOT NULL,
    input_hash TEXT NOT NULL,
    dedupe_key TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'leased', 'running', 'retry-wait', 'dead-letter', 'cancel-requested', 'cancelled', 'succeeded', 'orphan')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    not_before TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    fencing_counter INTEGER NOT NULL DEFAULT 0 CHECK (fencing_counter >= 0),
    last_error_code TEXT NOT NULL DEFAULT '',
    cancel_requested INTEGER NOT NULL DEFAULT 0 CHECK (cancel_requested IN (0, 1))
, trigger_event TEXT NOT NULL DEFAULT '', target_language TEXT NOT NULL DEFAULT '');
CREATE TABLE job_orphans (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    expired_fence INTEGER NOT NULL,
    worker_id TEXT NOT NULL,
    next_check_at TEXT NOT NULL
);
CREATE TABLE event_sequence (
    singleton INTEGER PRIMARY KEY CHECK (singleton=1),
    last_seq INTEGER NOT NULL CHECK (last_seq>=0)
);
CREATE TABLE consumer_offsets (
    consumer_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    acknowledged_seq INTEGER NOT NULL CHECK (acknowledged_seq>=0),
    PRIMARY KEY(consumer_id,data_epoch)
);
CREATE TABLE event_operations (
    caller TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('publish','ack','replay')),
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(caller,data_epoch,operation_kind,operation_id)
);
CREATE TABLE job_management_operations (
    caller TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (caller, data_epoch, operation_id)
);
CREATE TABLE schedule_operations (
    caller TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (caller, data_epoch, operation_id)
);
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
    safe_message_code TEXT NOT NULL, checkpoint_key TEXT NOT NULL DEFAULT '', input_hash TEXT NOT NULL DEFAULT '', created_at TEXT,
    PRIMARY KEY (job_id, seq)
);
CREATE TABLE job_results (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    result_ref TEXT NOT NULL,
    paid_result_refs_json BLOB NOT NULL,
    result_hash TEXT NOT NULL
);
CREATE TABLE job_outcomes (
    job_id TEXT NOT NULL REFERENCES jobs(job_id),
    fencing_token INTEGER NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('complete', 'fail')),
    worker_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    PRIMARY KEY (job_id, fencing_token)
);
CREATE UNIQUE INDEX idx_progress_checkpoint ON progress(job_id, checkpoint_key) WHERE checkpoint_key <> '';
CREATE UNIQUE INDEX idx_jobs_owner_epoch_dedupe ON jobs(owner,data_epoch,dedupe_key);
CREATE INDEX idx_jobs_claimable ON jobs(state,not_before,created_at,job_id);
CREATE INDEX idx_events_epoch_subject ON events(data_epoch,producer,subject_id,event_seq);
CREATE UNIQUE INDEX idx_deliveries_id ON deliveries(delivery_id);
CREATE INDEX idx_deliveries_available ON deliveries(consumer_id,state,retry_at,event_id);
CREATE INDEX idx_schedules_epoch_enabled ON schedules (data_epoch, enabled, schedule_id);
CREATE INDEX jobs_management_recent ON jobs(data_epoch,created_at DESC,job_id DESC);
INSERT INTO "event_sequence"("singleton","last_seq") VALUES(1,0);
PRAGMA user_version=9;
