-- AI 额度：邀请码、用户权益、站点访客问答上限。账本（已花费多少）在 model-gateway，这里只放“能花多少”。
CREATE TABLE ai_invites (
    invite_id     TEXT PRIMARY KEY,
    code_hash     TEXT NOT NULL UNIQUE,
    note          TEXT NOT NULL DEFAULT '',
    cap_micros    INTEGER NOT NULL CHECK (cap_micros >= 0),
    period        TEXT NOT NULL CHECK (period IN ('once', 'monthly')),
    tasks_json    TEXT NOT NULL DEFAULT '[]',
    max_uses      INTEGER NOT NULL CHECK (max_uses >= 1),
    used          INTEGER NOT NULL DEFAULT 0,
    max_sites     INTEGER,
    storage_bytes INTEGER,
    expires_at    TEXT,
    revoked_at    TEXT,
    created_by    TEXT NOT NULL,
    created_at    TEXT NOT NULL
);
CREATE TABLE user_ai (
    user_id       TEXT PRIMARY KEY REFERENCES users (user_id),
    invite_id     TEXT NOT NULL REFERENCES ai_invites (invite_id),
    cap_micros    INTEGER NOT NULL,
    period        TEXT NOT NULL CHECK (period IN ('once', 'monthly')),
    tasks_json    TEXT NOT NULL DEFAULT '[]',
    redeemed_at   TEXT NOT NULL,
    period_anchor TEXT NOT NULL
);
ALTER TABLE sites ADD COLUMN visitor_qa_daily INTEGER;
ALTER TABLE sites ADD COLUMN qa_paused_on TEXT NOT NULL DEFAULT '';
