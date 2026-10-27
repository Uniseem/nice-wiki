-- 私有站点的另外两种访问方式：访问密码（只存哈希）和分享链接（只存令牌摘要，可设有效期、可作废）。
ALTER TABLE sites ADD COLUMN private_password_hash TEXT NOT NULL DEFAULT '';
CREATE TABLE private_share_links (
    link_id    TEXT PRIMARY KEY,
    site_id    TEXT NOT NULL REFERENCES sites (site_id),
    token_hash TEXT NOT NULL UNIQUE,
    note       TEXT NOT NULL DEFAULT '',
    expires_at TEXT,
    revoked_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_private_share_links_site ON private_share_links (site_id);
