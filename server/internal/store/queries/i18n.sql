-- name: ListTranslationStatus :many
SELECT * FROM translation_status WHERE language = ? AND document_id = ? ORDER BY block_id;
-- name: TranslationProgress :many
SELECT status,COUNT(*) AS total FROM translation_status WHERE language = ? GROUP BY status;
