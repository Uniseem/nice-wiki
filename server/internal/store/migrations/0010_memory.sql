CREATE TABLE memory_conversations (
 id TEXT PRIMARY KEY,
 account_id TEXT NOT NULL,
 messages TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
);
CREATE TABLE memory_proposals (
 id TEXT PRIMARY KEY,
 conversation_id TEXT NOT NULL REFERENCES memory_conversations(id) ON DELETE CASCADE,
 operation TEXT NOT NULL,
 entry_id TEXT NOT NULL,
 before_hash TEXT NOT NULL,
 content TEXT NOT NULL,
 status TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
