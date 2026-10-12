-- name: GetConversation :one
SELECT * FROM conversations WHERE id = ? AND session_hash = ? AND expires_at > ?;
-- name: ListConversationRecords :many
SELECT * FROM qa_records WHERE conversation_id = ? ORDER BY created_at;
-- name: GetExactAnswerCache :one
SELECT * FROM answer_cache WHERE question_hash = ? AND expires_at > ?;
-- name: GetCachedLanguageAnswer :one
SELECT * FROM cached_answers WHERE cache_id = ? AND language = ?;
-- name: CreateConversation :exec
INSERT INTO conversations(id,session_hash,language,summary,created_at,expires_at) VALUES(?,?,?,?,?,?);
-- name: UpdateConversationSummary :exec
UPDATE conversations SET summary = ? WHERE id = ? AND session_hash = ?;
-- name: DeleteConversation :exec
DELETE FROM conversations WHERE id = ? AND session_hash = ?;
-- name: InsertQARecord :exec
INSERT INTO qa_records(id,conversation_id,question,normalized_question,answer,citations,language,status,created_at) VALUES(?,?,?,?,?,?,?,?,?);
-- name: UpsertQAFeedback :exec
INSERT INTO qa_feedback(record_id,session_hash,helpful,created_at) VALUES(?,?,?,?) ON CONFLICT(record_id,session_hash) DO UPDATE SET helpful=excluded.helpful,created_at=excluded.created_at;
-- name: ListQARecords :many
SELECT * FROM qa_records ORDER BY created_at DESC LIMIT ? OFFSET ?;
-- name: PutAnswerCache :exec
INSERT INTO answer_cache(id,question_hash,question,answer,citations,material_hash,memory_hash,hits,created_at,expires_at) VALUES(?,?,?,?,?,?,?,0,?,?) ON CONFLICT(question_hash) DO UPDATE SET answer=excluded.answer,citations=excluded.citations,material_hash=excluded.material_hash,memory_hash=excluded.memory_hash,created_at=excluded.created_at,expires_at=excluded.expires_at;
-- name: IncrementAnswerCacheHit :exec
UPDATE answer_cache SET hits = hits + 1 WHERE id = ?;
-- name: DeleteExpiredAnswerCache :exec
DELETE FROM answer_cache WHERE expires_at <= ?;
-- name: DeleteAnswerCacheAll :exec
DELETE FROM answer_cache;
-- name: UpsertFeaturedAnswer :exec
INSERT INTO featured_answers(id,record_id,title,sort_key,created_at) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET record_id=excluded.record_id,title=excluded.title,sort_key=excluded.sort_key;
-- name: DeleteFeaturedAnswer :exec
DELETE FROM featured_answers WHERE id = ?;
-- name: ListFeaturedAnswers :many
SELECT * FROM featured_answers ORDER BY sort_key,id;
