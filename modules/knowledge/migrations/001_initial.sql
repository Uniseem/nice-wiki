-- 本域私有索引 schemaVersion 1；control 屏障与平台 outbox/inbox 不在本迁移复制。
CREATE TABLE IF NOT EXISTS index_identity (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), site_id TEXT NOT NULL, data_epoch TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS builds (
 build_id TEXT PRIMARY KEY, site_id TEXT NOT NULL, data_epoch TEXT NOT NULL,
 document_id TEXT NOT NULL, document_json BLOB NOT NULL, space_json BLOB NOT NULL, space_id TEXT NOT NULL,
 language TEXT NOT NULL, embedding_profile_id TEXT NOT NULL, state TEXT NOT NULL, generation INTEGER NOT NULL,
 lease_id TEXT NOT NULL DEFAULT '', lease_until TEXT NOT NULL DEFAULT '',
 cancelled INTEGER NOT NULL DEFAULT 0 CHECK(cancelled IN (0,1)),
 completed_chunks INTEGER NOT NULL DEFAULT 0, total_chunks INTEGER NOT NULL DEFAULT 0,
 failure_code TEXT NOT NULL DEFAULT '', manifest_hash TEXT NOT NULL DEFAULT '',
 generation_state TEXT NOT NULL DEFAULT 'staging', validated_at TEXT NOT NULL DEFAULT '', visibility_revision INTEGER NOT NULL DEFAULT 0,
 UNIQUE(site_id,data_epoch,build_id,generation)
);
CREATE TABLE IF NOT EXISTS chunks (
 build_id TEXT NOT NULL REFERENCES builds(build_id), chunk_id TEXT NOT NULL,
 chunk_hash TEXT NOT NULL, chunk_json BLOB NOT NULL, ordinal INTEGER NOT NULL,
 PRIMARY KEY(build_id,chunk_id), UNIQUE(build_id,ordinal)
);
CREATE TABLE IF NOT EXISTS terms (
 build_id TEXT NOT NULL, chunk_id TEXT NOT NULL, language TEXT NOT NULL,
 tokenizer_version TEXT NOT NULL, terms_json BLOB NOT NULL, length INTEGER NOT NULL,
 PRIMARY KEY(build_id,chunk_id), FOREIGN KEY(build_id,chunk_id) REFERENCES chunks(build_id,chunk_id)
);
CREATE TABLE IF NOT EXISTS vectors (
 build_id TEXT NOT NULL, chunk_id TEXT NOT NULL, chunk_hash TEXT NOT NULL,
 space_json BLOB NOT NULL, vector_json BLOB NOT NULL, vector_hash TEXT NOT NULL,
 PRIMARY KEY(build_id,chunk_id), FOREIGN KEY(build_id,chunk_id) REFERENCES chunks(build_id,chunk_id)
);
CREATE TABLE IF NOT EXISTS active_indexes (
 site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, document_id TEXT NOT NULL,
 space_id TEXT NOT NULL, build_id TEXT NOT NULL REFERENCES builds(build_id),
 generation INTEGER NOT NULL, visibility_revision INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,document_id,space_id)
);
CREATE TABLE IF NOT EXISTS query_embeddings (
 site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, root_operation_id TEXT NOT NULL,
 query_hash TEXT NOT NULL, space_id TEXT NOT NULL, space_json BLOB NOT NULL,
 operation_id TEXT NOT NULL, result_json BLOB,
 PRIMARY KEY(site_id,data_epoch,root_operation_id,query_hash,space_id)
);
CREATE TABLE IF NOT EXISTS query_embedding_keys (
 site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, actor_id TEXT NOT NULL,
 idempotency_key TEXT NOT NULL, payload_hash TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,actor_id,idempotency_key)
);
