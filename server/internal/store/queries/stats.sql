-- name: RecordPageView :exec
INSERT INTO page_views(document_id,language,country,referrer_domain,visitor_hash,bot,created_at) VALUES(?,?,?,?,?,?,?);
-- name: GetDailySalt :one
SELECT salt FROM daily_salts WHERE day = ?;
-- name: ListDailyStats :many
SELECT * FROM daily_stats WHERE day >= ? AND day <= ? ORDER BY day,dimension,value;
