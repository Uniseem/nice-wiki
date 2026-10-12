-- name: GetMemoryConversation :one
SELECT * FROM memory_conversations WHERE id = ? AND account_id = ?;
-- name: ListPendingProposals :many
SELECT * FROM memory_proposals WHERE conversation_id = ? AND status = 'pending' ORDER BY created_at;
