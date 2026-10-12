CREATE TABLE documents (
 id TEXT PRIMARY KEY,
 parent_id TEXT NOT NULL DEFAULT '',
 title TEXT NOT NULL,
 slug TEXT NOT NULL,
 sort_key TEXT NOT NULL,
 file_path TEXT NOT NULL UNIQUE,
 path TEXT NOT NULL UNIQUE,
 published INTEGER NOT NULL DEFAULT 0 CHECK(published IN (0,1)),
 revision INTEGER NOT NULL DEFAULT 1,
 content_hash TEXT NOT NULL,
 updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX documents_sibling_slug ON documents(parent_id,slug);
CREATE TABLE blocks (
 id TEXT PRIMARY KEY,
 document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
 parent_id TEXT NOT NULL DEFAULT '',
 kind TEXT NOT NULL,
 position INTEGER NOT NULL,
 heading_path TEXT NOT NULL DEFAULT '',
 text TEXT NOT NULL,
 content_hash TEXT NOT NULL
);
CREATE INDEX blocks_document ON blocks(document_id,position);
CREATE TABLE assets (
 hash TEXT PRIMARY KEY,
 kind TEXT NOT NULL,
 extension TEXT NOT NULL,
 mime_type TEXT NOT NULL,
 size INTEGER NOT NULL CHECK(size >= 0),
 original_name TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE TABLE asset_references (
 asset_hash TEXT NOT NULL REFERENCES assets(hash),
 document_id TEXT NOT NULL,
 block_id TEXT NOT NULL,
 source TEXT NOT NULL,
 PRIMARY KEY(asset_hash,document_id,block_id,source)
);
