-- 外观市场（docs/theme-standard.md）：登录用户提交外观包，平台管理员审核通过后上架。
-- 同一个外观 id 只属于第一个提交者；每个版本单独审核，市场列表取每个 id 最新通过的版本。
CREATE TABLE market_themes (
    theme_id      TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users (user_id),
    created_at    TEXT NOT NULL
);
CREATE TABLE market_theme_versions (
    theme_id     TEXT NOT NULL REFERENCES market_themes (theme_id),
    version      TEXT NOT NULL,
    digest       TEXT NOT NULL,
    manifest     TEXT NOT NULL,
    package      BLOB NOT NULL,
    byte_size    INTEGER NOT NULL CHECK (byte_size > 0),
    status       TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    review_note  TEXT NOT NULL DEFAULT '',
    submitted_at TEXT NOT NULL,
    reviewed_at  TEXT,
    PRIMARY KEY (theme_id, version)
);
CREATE INDEX idx_market_theme_versions_status ON market_theme_versions (status, submitted_at);
