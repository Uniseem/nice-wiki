-- name: RecordUsage :exec
INSERT INTO llm_usage(channel_id,model,task,input_tokens,output_tokens,cached_tokens,cost_usd,status,created_at) VALUES(?,?,?,?,?,?,?,?,?);
-- name: UsageSince :many
SELECT * FROM llm_usage WHERE created_at >= ? ORDER BY id;
-- name: GetLocaleCache :one
SELECT * FROM locale_cache WHERE signal_hash = ? AND expires_at > ?;
-- name: PutLocaleCache :exec
INSERT INTO locale_cache(signal_hash,language,reason,expires_at) VALUES(?,?,?,?) ON CONFLICT(signal_hash) DO UPDATE SET language=excluded.language,reason=excluded.reason,expires_at=excluded.expires_at;
-- name: DeleteExpiredLocaleCache :exec
DELETE FROM locale_cache WHERE expires_at <= ?;
