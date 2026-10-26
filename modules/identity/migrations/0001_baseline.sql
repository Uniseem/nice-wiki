-- 基线迁移：由历史迁移重放得到的最终结构（0001_domain.sql 至 0004_private_operation_receipts.sql）。
CREATE TABLE accounts (
    account_id           TEXT PRIMARY KEY,
    site_id              TEXT NOT NULL,
    account_name         TEXT NOT NULL UNIQUE,
    is_superadmin        INTEGER NOT NULL CHECK (is_superadmin IN (0, 1)),
    must_change_password INTEGER NOT NULL CHECK (must_change_password IN (0, 1)),
    status               TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'deleted')),
    revision             INTEGER NOT NULL,
    permission_revision  INTEGER NOT NULL,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);
CREATE TABLE credentials (
    account_id          TEXT PRIMARY KEY REFERENCES accounts (account_id),
    password_hash       TEXT NOT NULL,
    credential_revision INTEGER NOT NULL
);
CREATE TABLE account_permissions (
    account_id TEXT NOT NULL REFERENCES accounts (account_id),
    permission TEXT NOT NULL,
    PRIMARY KEY (account_id, permission)
);
CREATE TABLE sessions (
    session_id                  TEXT PRIMARY KEY,
    account_id                  TEXT NOT NULL REFERENCES accounts (account_id),
    site_id                     TEXT NOT NULL,
    credential_revision         INTEGER NOT NULL,
    permission_revision         INTEGER NOT NULL,
    issued_at                   TEXT NOT NULL,
    expires_at                  TEXT NOT NULL,
    absolute_expires_at         TEXT NOT NULL,
    device_label                TEXT NOT NULL DEFAULT '',
    restricted_to_password_change INTEGER NOT NULL CHECK (restricted_to_password_change IN (0, 1)),
    revoked_at                  TEXT,
    revision                    INTEGER NOT NULL
);
CREATE TABLE preferences (
    account_id    TEXT PRIMARY KEY REFERENCES accounts (account_id),
    theme         TEXT NOT NULL,
    locale        TEXT NOT NULL,
    default_route TEXT NOT NULL,
    revision      INTEGER NOT NULL
);
CREATE TABLE operation_receipts (
    operation_id  TEXT NOT NULL,
    site_id       TEXT NOT NULL,
    actor_id      TEXT NOT NULL,
    input_digest  TEXT NOT NULL,
    result_hash   TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'committed',
    created_at    TEXT NOT NULL, result_json TEXT, digest_key_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (site_id, actor_id, operation_id)
);
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE auth_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    auth_epoch TEXT NOT NULL
);
CREATE TABLE session_families (
    family_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE REFERENCES sessions (session_id),
    auth_epoch TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    refresh_digest TEXT NOT NULL UNIQUE,
    access_digest TEXT NOT NULL,
    csrf_digest TEXT NOT NULL,
    revoked_at TEXT,
    revision INTEGER NOT NULL CHECK (revision >= 1)
);
CREATE TABLE consumed_refresh_tokens (
    refresh_digest TEXT PRIMARY KEY,
    family_id TEXT NOT NULL REFERENCES session_families (family_id),
    consumed_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_accounts_single_superadmin
    ON accounts (is_superadmin) WHERE is_superadmin = 1;
CREATE INDEX idx_sessions_account ON sessions (account_id);
CREATE INDEX idx_sessions_open ON sessions (account_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_self_page ON sessions (account_id, site_id, issued_at, session_id);
CREATE INDEX idx_consumed_refresh_family ON consumed_refresh_tokens (family_id);
