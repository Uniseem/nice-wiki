-- 仅过期未完成上传的删除意图；不记录或删除ready资产。
CREATE TABLE upload_expirations (
 upload_id TEXT PRIMARY KEY REFERENCES upload_sessions(upload_id),
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('prepared','removed')),
 decided_at TEXT NOT NULL,
 removed_at TEXT
);
