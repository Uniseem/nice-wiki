-- name: ListTranslationStatus :many
SELECT * FROM translation_status WHERE language = ? AND document_id = ? ORDER BY block_id;
-- name: TranslationProgress :many
SELECT status,COUNT(*) AS total FROM translation_status WHERE language = ? GROUP BY status;
-- name: UpsertTranslationStatus :exec
INSERT INTO translation_status(language,document_id,block_id,source_hash,status,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(language,document_id,block_id) DO UPDATE SET source_hash=excluded.source_hash,status=excluded.status,updated_at=excluded.updated_at;
-- name: DeleteDocumentTranslationStatus :exec
DELETE FROM translation_status WHERE language = ? AND document_id = ?;
