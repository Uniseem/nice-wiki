-- name: RecordUsage :exec
INSERT INTO llm_usage(channel_id,model,task,input_tokens,output_tokens,cached_tokens,cost_usd,status,created_at) VALUES(?,?,?,?,?,?,?,?,?);
-- name: UsageSince :many
SELECT * FROM llm_usage WHERE created_at >= ? ORDER BY id;
-- name: DeleteExpiredLocaleCache :exec
DELETE FROM locale_cache WHERE expires_at <= ?;
