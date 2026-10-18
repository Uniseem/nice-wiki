-- 匿名风控只保存不可逆作用域和安全引用；token、问题正文、原IP不入库。
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
    updated_at_ns INTEGER NOT NULL,
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
    consumed_request_id TEXT,revision INTEGER NOT NULL,policy_revision INTEGER NOT NULL,
    UNIQUE(site_id,data_epoch,owner_caller,request_id)
);
CREATE INDEX guard_permits_active ON guard_permits(site_id,data_epoch,state,expires_at_ns);
CREATE INDEX guard_permits_session ON guard_permits(site_id,data_epoch,session_key,state,expires_at_ns);
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
);
CREATE INDEX guard_bans_network ON guard_bans(site_id,data_epoch,network_key,state,expires_at_ns);
CREATE TABLE guard_question_signals (
    signal_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    question_operation_id TEXT NOT NULL,input_hash TEXT NOT NULL,
    session_key TEXT NOT NULL,network_key TEXT NOT NULL,question_hash TEXT NOT NULL,
    similarity_score INTEGER,interval_ms INTEGER,model_verdict TEXT NOT NULL,
    occurred_at_ns INTEGER NOT NULL,
    UNIQUE(site_id,data_epoch,question_operation_id)
);
CREATE INDEX guard_signals_window ON guard_question_signals(site_id,data_epoch,session_key,occurred_at_ns);
CREATE TABLE guard_risk_events (
    event_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
    action TEXT NOT NULL,decision TEXT NOT NULL,network_key TEXT NOT NULL,occurred_at_ns INTEGER NOT NULL
);
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
