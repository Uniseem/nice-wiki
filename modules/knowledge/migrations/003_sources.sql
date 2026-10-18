ALTER TABLE build_dispatches ADD COLUMN jobs_operation_id TEXT;
CREATE TABLE build_sources (
 build_id TEXT PRIMARY KEY REFERENCES builds(build_id),document_json BLOB NOT NULL,
 content_hash TEXT NOT NULL,body_json BLOB NOT NULL,created_at TEXT NOT NULL
);
