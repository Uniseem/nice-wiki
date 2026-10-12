CREATE TABLE retrieval_units (
 id TEXT PRIMARY KEY,
 document_id TEXT NOT NULL,
 kind TEXT NOT NULL,
 text TEXT NOT NULL,
 context TEXT NOT NULL DEFAULT '',
 content_hash TEXT NOT NULL,
 embedding_model TEXT NOT NULL DEFAULT '',
 updated_at INTEGER NOT NULL
);
CREATE TABLE retrieval_unit_blocks (
 unit_id TEXT NOT NULL REFERENCES retrieval_units(id) ON DELETE CASCADE,
 block_id TEXT NOT NULL,
 position INTEGER NOT NULL,
 PRIMARY KEY(unit_id,block_id)
);
CREATE TABLE vectors (
 unit_id TEXT PRIMARY KEY REFERENCES retrieval_units(id) ON DELETE CASCADE,
 dimensions INTEGER NOT NULL CHECK(dimensions > 0),
 vector BLOB NOT NULL,
 model TEXT NOT NULL,
 content_hash TEXT NOT NULL
);
CREATE VIRTUAL TABLE keyword_index USING fts5(unit_id UNINDEXED, document_id UNINDEXED, text, tokenize='unicode61');
CREATE TABLE description_cache (
 asset_hash TEXT NOT NULL,
 context_hash TEXT NOT NULL,
 task TEXT NOT NULL,
 description TEXT NOT NULL,
 model TEXT NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(asset_hash,context_hash,task)
);
