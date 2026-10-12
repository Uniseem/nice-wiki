CREATE TABLE job_merge (
 kind TEXT NOT NULL,
 subject_id TEXT NOT NULL,
 generation INTEGER NOT NULL DEFAULT 1,
 queued_generation INTEGER NOT NULL DEFAULT 0,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(kind,subject_id)
);
