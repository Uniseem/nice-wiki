-- name: GetConversation :one
SELECT * FROM conversations WHERE id = ? AND session_hash = ? AND expires_at > ?;
-- name: ListConversationRecords :many
SELECT * FROM qa_records WHERE conversation_id = ? ORDER BY created_at;
-- name: GetExactAnswerCache :one
SELECT * FROM answer_cache WHERE question_hash = ? AND expires_at > ?;
-- name: GetCachedLanguageAnswer :one
SELECT * FROM cached_answers WHERE cache_id = ? AND language = ?;
