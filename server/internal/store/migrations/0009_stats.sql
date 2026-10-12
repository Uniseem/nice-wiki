CREATE TABLE page_views (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 document_id TEXT NOT NULL,
 language TEXT NOT NULL,
 country TEXT NOT NULL,
 referrer_domain TEXT NOT NULL,
 visitor_hash TEXT NOT NULL,
 bot INTEGER NOT NULL CHECK(bot IN (0,1)),
 created_at INTEGER NOT NULL
);
CREATE INDEX page_views_time ON page_views(created_at);
CREATE TABLE daily_stats (
 day TEXT NOT NULL,
 dimension TEXT NOT NULL,
 value TEXT NOT NULL,
 views INTEGER NOT NULL,
 visitors INTEGER NOT NULL,
 PRIMARY KEY(day,dimension,value)
);
CREATE TABLE daily_salts (
 day TEXT PRIMARY KEY,
 salt BLOB NOT NULL
);
