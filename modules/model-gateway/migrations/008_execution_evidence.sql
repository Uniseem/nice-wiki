-- schemaVersion8：仅持久脱敏派发证据，明示未接收才能释放；不得用超时作证明。
CREATE TABLE attempt_evidence(attempt_id TEXT PRIMARY KEY REFERENCES invocation_attempts(attempt_id),outcome_json BLOB NOT NULL,outcome_hash TEXT NOT NULL,recorded_ns INTEGER NOT NULL);
ALTER TABLE invocations ADD COLUMN execution_hash TEXT;
