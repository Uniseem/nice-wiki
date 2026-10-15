-- 旧上传缺创建epoch，不推测归属；仅新创建会话可以继续分块。
ALTER TABLE upload_sessions ADD COLUMN data_epoch TEXT;
CREATE TABLE upload_chunks (
    operation_id TEXT PRIMARY KEY,
    upload_id TEXT NOT NULL REFERENCES upload_sessions(upload_id),
    site_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    expected_revision INTEGER NOT NULL CHECK(expected_revision>=0),
    byte_offset INTEGER NOT NULL CHECK(byte_offset>=0),
    byte_length INTEGER NOT NULL CHECK(byte_length BETWEEN 1 AND 8388608),
    sha256 TEXT NOT NULL,
    stage_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('PREPARED','COMMITTED')),
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    CHECK((state='PREPARED' AND result_json IS NULL AND result_hash IS NULL) OR
          (state='COMMITTED' AND result_json IS NOT NULL AND result_hash IS NOT NULL))
);
CREATE INDEX idx_upload_chunks_upload ON upload_chunks(upload_id,state,byte_offset);
CREATE UNIQUE INDEX idx_upload_chunks_committed_revision ON upload_chunks(upload_id,expected_revision) WHERE state='COMMITTED';
CREATE UNIQUE INDEX idx_upload_chunks_committed_offset ON upload_chunks(upload_id,byte_offset) WHERE state='COMMITTED';
-- 旧Begin回执不从mutable session推测首次结果；缺快照要求新操作。
ALTER TABLE upload_receipts ADD COLUMN result_json TEXT;
ALTER TABLE upload_receipts ADD COLUMN result_hash TEXT;
