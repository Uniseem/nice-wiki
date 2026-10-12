-- name: ListRetrievalUnits :many
SELECT * FROM retrieval_units WHERE document_id = ? ORDER BY id;
-- name: GetDescription :one
SELECT * FROM description_cache WHERE asset_hash = ? AND context_hash = ? AND task = ?;
-- name: DeleteDocumentUnits :exec
DELETE FROM retrieval_units WHERE document_id = ?;
