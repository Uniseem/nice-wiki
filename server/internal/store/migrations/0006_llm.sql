CREATE TABLE llm_usage (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 channel_id TEXT NOT NULL,
 model TEXT NOT NULL,
 task TEXT NOT NULL,
 input_tokens INTEGER NOT NULL,
 output_tokens INTEGER NOT NULL,
 cached_tokens INTEGER NOT NULL DEFAULT 0,
 cost_usd REAL NOT NULL CHECK(cost_usd >= 0),
 status TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE INDEX llm_usage_day ON llm_usage(created_at,task);
CREATE TABLE locale_cache (
 signal_hash TEXT PRIMARY KEY,
 language TEXT NOT NULL,
 reason TEXT NOT NULL,
 expires_at INTEGER NOT NULL
);
CREATE TABLE model_catalog (
 channel_id TEXT NOT NULL,
 model_id TEXT NOT NULL,
 capabilities TEXT NOT NULL,
 prices TEXT NOT NULL,
 fetched_at INTEGER NOT NULL,
 PRIMARY KEY(channel_id,model_id)
);
CREATE TABLE qa_debug (
 id TEXT PRIMARY KEY,
 account_id TEXT NOT NULL,
 stages TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL
);
