-- 基线迁移：由历史迁移重放得到的最终结构（0001_domain.sql 至 0004_ip_bans.sql）。
CREATE TABLE guard_policy (
    site_id     TEXT PRIMARY KEY,
    revision    INTEGER NOT NULL CHECK (revision >= 0),
    policy_json TEXT NOT NULL CHECK (json_valid(policy_json)),
    data_epoch  TEXT NOT NULL
);
CREATE TABLE policy_operation_receipts (
    site_id       TEXT NOT NULL,
    operation_id  TEXT NOT NULL,
    input_hash    TEXT NOT NULL,
    revision      INTEGER NOT NULL CHECK (revision >= 0),
    data_epoch    TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (site_id, operation_id)
);
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE login_attempts (
    attempt_id TEXT PRIMARY KEY,
    data_epoch TEXT NOT NULL,
    caller TEXT NOT NULL CHECK (caller = 'identity'),
    request_id TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    account_key TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('login', 'bootstrap')),
    created_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL CHECK (expires_at_ns > created_at_ns),
    creation_revision INTEGER NOT NULL CHECK (creation_revision = 1),
    outcome TEXT NOT NULL CHECK (outcome IN ('pending', 'authenticated', 'rejected', 'aborted')),
    finished_at_ns INTEGER,
    UNIQUE (data_epoch, caller, request_id),
    UNIQUE (data_epoch, attempt_id),
    CHECK ((outcome = 'pending' AND finished_at_ns IS NULL) OR
           (outcome <> 'pending' AND finished_at_ns IS NOT NULL))
);
CREATE TABLE login_attempt_networks (
    data_epoch TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    network_key TEXT NOT NULL,
    PRIMARY KEY (data_epoch, attempt_id, network_key),
    FOREIGN KEY (data_epoch, attempt_id) REFERENCES login_attempts (data_epoch, attempt_id)
);
CREATE TABLE login_failures (
    data_epoch TEXT NOT NULL,
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('account', 'network')),
    scope_key TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    failed_at_ns INTEGER NOT NULL,
    PRIMARY KEY (data_epoch, scope_kind, scope_key, attempt_id),
    FOREIGN KEY (data_epoch, attempt_id) REFERENCES login_attempts (data_epoch, attempt_id)
);
CREATE TABLE login_locks (
    data_epoch TEXT NOT NULL,
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('account', 'network')),
    scope_key TEXT NOT NULL,
    locked_until_ns INTEGER NOT NULL,
    PRIMARY KEY (data_epoch, scope_kind, scope_key)
);
CREATE TABLE login_finish_receipts (
    data_epoch TEXT NOT NULL,
    caller TEXT NOT NULL CHECK (caller = 'identity'),
    operation_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('authenticated', 'rejected', 'aborted')),
    created_at_ns INTEGER NOT NULL,
    PRIMARY KEY (data_epoch, caller, operation_id),
    FOREIGN KEY (data_epoch, attempt_id) REFERENCES login_attempts (data_epoch, attempt_id)
);
CREATE TABLE guard_operation_receipts (
    site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, caller TEXT NOT NULL,
    operation_kind TEXT NOT NULL, operation_id TEXT NOT NULL, input_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at_ns INTEGER NOT NULL,
    PRIMARY KEY(site_id,data_epoch,caller,operation_kind,operation_id)
);
CREATE TABLE guard_subjects (
    site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, subject_key TEXT NOT NULL,
    session_key TEXT NOT NULL, network_key TEXT NOT NULL,
    challenge_passed_until_ns INTEGER NOT NULL DEFAULT 0,
    model_verdict TEXT NOT NULL DEFAULT 'normal', questions_since_challenge INTEGER NOT NULL DEFAULT 0,
    challenge_level INTEGER NOT NULL DEFAULT 0,suspicious_until_ns INTEGER NOT NULL DEFAULT 0,
    updated_at_ns INTEGER NOT NULL, ip TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(site_id,data_epoch,subject_key)
);
CREATE TABLE guard_network_aliases (
    site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, subject_key TEXT NOT NULL,
    network_key TEXT NOT NULL, last_seen_at_ns INTEGER NOT NULL,
    PRIMARY KEY(site_id,data_epoch,subject_key,network_key)
);
CREATE TABLE guard_buckets (
    site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,action TEXT NOT NULL,scope_key TEXT NOT NULL,
    tokens_milli INTEGER NOT NULL CHECK(tokens_milli>=0),updated_at_ms INTEGER NOT NULL,
    PRIMARY KEY(site_id,data_epoch,action,scope_key)
);
CREATE TABLE guard_permits (
    admission_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    owner_caller TEXT NOT NULL,request_id TEXT NOT NULL,input_hash TEXT NOT NULL,
    subject_key TEXT NOT NULL,session_key TEXT NOT NULL,network_key TEXT NOT NULL,action TEXT NOT NULL,
    expires_at_ns INTEGER NOT NULL,hard_deadline_ns INTEGER NOT NULL,created_at_ns INTEGER NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('active','released','expired','revoked')),
    consumed_request_id TEXT,revision INTEGER NOT NULL,policy_revision INTEGER NOT NULL, ip TEXT NOT NULL DEFAULT '',
    UNIQUE(site_id,data_epoch,owner_caller,request_id)
);
CREATE TABLE guard_challenges (
    challenge_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    subject_key TEXT NOT NULL,session_key TEXT NOT NULL,request_id TEXT NOT NULL,input_hash TEXT NOT NULL,
    method TEXT NOT NULL,difficulty INTEGER NOT NULL,nonce_hash TEXT NOT NULL,
    expires_at_ns INTEGER NOT NULL,created_at_ns INTEGER NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('issued','consumed','expired')),attempts INTEGER NOT NULL DEFAULT 0,
    UNIQUE(site_id,data_epoch,subject_key,request_id)
);
CREATE TABLE guard_bans (
    ban_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    network_key TEXT NOT NULL,scope TEXT NOT NULL CHECK(scope='ai'),reason_code TEXT NOT NULL,
    starts_at_ns INTEGER NOT NULL,expires_at_ns INTEGER NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('active','expired','revoked')),revision INTEGER NOT NULL
, ip TEXT NOT NULL DEFAULT '');
CREATE TABLE guard_question_signals (
    signal_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    question_operation_id TEXT NOT NULL,input_hash TEXT NOT NULL,
    session_key TEXT NOT NULL,network_key TEXT NOT NULL,question_hash TEXT NOT NULL,
    similarity_score INTEGER,interval_ms INTEGER,model_verdict TEXT NOT NULL,
    occurred_at_ns INTEGER NOT NULL, ip TEXT NOT NULL DEFAULT '',
    UNIQUE(site_id,data_epoch,question_operation_id)
);
CREATE TABLE guard_risk_events (
    event_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    action TEXT NOT NULL,decision TEXT NOT NULL,network_key TEXT NOT NULL,occurred_at_ns INTEGER NOT NULL
, ip TEXT NOT NULL DEFAULT '');
CREATE TABLE guard_waiters (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    session_key TEXT NOT NULL,request_id TEXT NOT NULL,created_at_ns INTEGER NOT NULL,
    UNIQUE(site_id,data_epoch,session_key,request_id)
);
CREATE TABLE guard_identity_revisions (
    site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,account_id TEXT NOT NULL,permission_revision INTEGER NOT NULL,
    PRIMARY KEY(site_id,data_epoch,account_id)
);
CREATE TABLE guard_permit_networks (
    admission_id TEXT NOT NULL,network_key TEXT NOT NULL,
    PRIMARY KEY(admission_id,network_key),
    FOREIGN KEY(admission_id) REFERENCES guard_permits(admission_id)
);
CREATE INDEX idx_policy_receipts_epoch
    ON policy_operation_receipts (data_epoch, created_at);
CREATE INDEX login_account_pending ON login_attempts (data_epoch, account_key, outcome, expires_at_ns);
CREATE INDEX login_network_pending ON login_attempt_networks (data_epoch, network_key, attempt_id);
CREATE INDEX login_failure_window ON login_failures (data_epoch, scope_kind, scope_key, failed_at_ns);
CREATE INDEX guard_permits_active ON guard_permits(site_id,data_epoch,state,expires_at_ns);
CREATE INDEX guard_permits_session ON guard_permits(site_id,data_epoch,session_key,state,expires_at_ns);
CREATE INDEX guard_bans_network ON guard_bans(site_id,data_epoch,network_key,state,expires_at_ns);
CREATE INDEX guard_signals_window ON guard_question_signals(site_id,data_epoch,session_key,occurred_at_ns);
CREATE INDEX guard_permit_ip ON guard_permits(site_id,data_epoch,ip,state);
CREATE INDEX guard_ip_bans ON guard_bans(site_id,data_epoch,ip,state,expires_at_ns);
CREATE TRIGGER guard_risk_ip AFTER INSERT ON guard_risk_events WHEN NEW.ip='' BEGIN UPDATE guard_risk_events SET ip=COALESCE((SELECT ip FROM guard_subjects WHERE site_id=NEW.site_id AND data_epoch=NEW.data_epoch AND network_key=NEW.network_key ORDER BY updated_at_ns DESC LIMIT 1),'') WHERE event_id=NEW.event_id; END;
PRAGMA user_version=4;
