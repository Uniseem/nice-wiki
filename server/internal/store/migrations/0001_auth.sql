CREATE TABLE sessions (
 token_hash TEXT PRIMARY KEY,
 account_id TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL,
 last_seen_at INTEGER NOT NULL
);
CREATE INDEX sessions_account ON sessions(account_id);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE login_failures (
 scope TEXT NOT NULL,
 subject_hash TEXT NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 window_start INTEGER NOT NULL,
 locked_until INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(scope, subject_hash)
);
CREATE TABLE audit_log (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 account_id TEXT NOT NULL,
 account_name TEXT NOT NULL,
 action TEXT NOT NULL,
 target_id TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL
);
CREATE INDEX audit_log_time ON audit_log(created_at DESC);
