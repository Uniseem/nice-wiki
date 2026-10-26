-- 私有站点：协作者在平台域名确认身份后得到一次性码，到站点域名换取短期访问（只存摘要，60 秒内有效、只能用一次）。
CREATE TABLE private_access_codes (
    code_hash  TEXT PRIMARY KEY,
    site_id    TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX idx_private_access_expiry ON private_access_codes (expires_at);
