CREATE TABLE ip_bans (
 ip_hash TEXT PRIMARY KEY,
 reason TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL
);
CREATE TABLE suspicious_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 ip_hash TEXT NOT NULL,
 kind TEXT NOT NULL,
 severity INTEGER NOT NULL,
 detail_code TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE INDEX suspicious_events_time ON suspicious_events(created_at DESC);
