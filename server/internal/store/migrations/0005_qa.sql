CREATE TABLE conversations (
 id TEXT PRIMARY KEY,
 session_hash TEXT NOT NULL,
 language TEXT NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL
);
CREATE TABLE qa_records (
 id TEXT PRIMARY KEY,
 conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
 question TEXT NOT NULL,
 normalized_question TEXT NOT NULL,
 answer TEXT NOT NULL,
 citations TEXT NOT NULL DEFAULT '[]',
 language TEXT NOT NULL,
 status TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE TABLE qa_feedback (
 record_id TEXT NOT NULL REFERENCES qa_records(id) ON DELETE CASCADE,
 session_hash TEXT NOT NULL,
 helpful INTEGER NOT NULL CHECK(helpful IN (0,1)),
 created_at INTEGER NOT NULL,
 PRIMARY KEY(record_id,session_hash)
);
CREATE TABLE answer_cache (
 id TEXT PRIMARY KEY,
 question_hash TEXT NOT NULL UNIQUE,
 question TEXT NOT NULL,
 answer TEXT NOT NULL,
 citations TEXT NOT NULL,
 material_hash TEXT NOT NULL,
 memory_hash TEXT NOT NULL,
 hits INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL
);
CREATE TABLE cached_answers (
 cache_id TEXT NOT NULL REFERENCES answer_cache(id) ON DELETE CASCADE,
 language TEXT NOT NULL,
 answer TEXT NOT NULL,
 citations TEXT NOT NULL,
 source_hash TEXT NOT NULL,
 PRIMARY KEY(cache_id,language)
);
CREATE TABLE featured_answers (
 id TEXT PRIMARY KEY,
 record_id TEXT NOT NULL REFERENCES qa_records(id) ON DELETE CASCADE,
 title TEXT NOT NULL,
 sort_key TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
