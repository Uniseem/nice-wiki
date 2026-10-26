-- 多用户：用户、第三方登录身份、站点、成员、域名、邀请码、举报、平台设置。
-- 每个用户有一个“家账户”（accounts.site_id 形如 home:<用户ID>），会话与现有账户机制不变；
-- 访问某个站点时由授权服务按成员角色即时派生权限。
CREATE TABLE platform_settings (
    key        TEXT PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE platform_secrets (
    name       TEXT PRIMARY KEY,
    nonce      BLOB NOT NULL,
    ciphertext BLOB NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE users (
    user_id      TEXT PRIMARY KEY,
    account_id   TEXT NOT NULL UNIQUE REFERENCES accounts (account_id),
    email        TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL,
    avatar_url   TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'deleted')),
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_users_email ON users (email) WHERE email <> '';
CREATE TABLE user_identities (
    provider   TEXT NOT NULL,
    subject    TEXT NOT NULL,
    user_id    TEXT NOT NULL REFERENCES users (user_id),
    email      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (provider, subject)
);
CREATE INDEX idx_user_identities_user ON user_identities (user_id);
CREATE TABLE user_limits (
    user_id       TEXT PRIMARY KEY REFERENCES users (user_id),
    max_sites     INTEGER NOT NULL,
    storage_bytes INTEGER NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE TABLE oauth_states (
    state_hash   TEXT PRIMARY KEY,
    provider     TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);
CREATE TABLE registration_invites (
    invite_id  TEXT PRIMARY KEY,
    code_hash  TEXT NOT NULL UNIQUE,
    note       TEXT NOT NULL DEFAULT '',
    max_uses   INTEGER NOT NULL CHECK (max_uses >= 1),
    used       INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT,
    revoked_at TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE sites (
    site_id       TEXT PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    owner_user_id TEXT NOT NULL REFERENCES users (user_id),
    status        TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'deleted')),
    visibility    TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
    base_language TEXT NOT NULL DEFAULT 'zh-Hans',
    revision      INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE INDEX idx_sites_owner ON sites (owner_user_id);
CREATE TABLE site_members (
    site_id    TEXT NOT NULL REFERENCES sites (site_id),
    user_id    TEXT NOT NULL REFERENCES users (user_id),
    role       TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    created_at TEXT NOT NULL,
    PRIMARY KEY (site_id, user_id)
);
CREATE INDEX idx_site_members_user ON site_members (user_id);
CREATE TABLE site_domains (
    host         TEXT PRIMARY KEY,
    site_id      TEXT NOT NULL REFERENCES sites (site_id),
    kind         TEXT NOT NULL CHECK (kind IN ('subdomain', 'custom')),
    status       TEXT NOT NULL CHECK (status IN ('pending', 'verified', 'failed')),
    is_primary   INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
    verify_token TEXT NOT NULL DEFAULT '',
    verified_at  TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_site_domains_site ON site_domains (site_id);
CREATE TABLE reports (
    report_id    TEXT PRIMARY KEY,
    site_id      TEXT NOT NULL DEFAULT '',
    host         TEXT NOT NULL,
    url          TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL,
    detail       TEXT NOT NULL DEFAULT '',
    contact_hash TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'dismissed', 'takedown', 'banned')),
    resolution   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_reports_status ON reports (status, created_at);
