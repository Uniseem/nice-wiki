CREATE TABLE translation_status (
 document_id TEXT NOT NULL,
 language TEXT NOT NULL,
 block_id TEXT NOT NULL,
 source_hash TEXT NOT NULL,
 translated_hash TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL,
 source TEXT NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(document_id,language,block_id)
);
CREATE INDEX translation_status_progress ON translation_status(language,status);
