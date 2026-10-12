-- name: RecordPageView :exec
INSERT INTO page_views(document_id,language,country,referrer_domain,visitor_hash,bot,ai_bot,created_at) VALUES(?,?,?,?,?,?,?,?);
-- name: GetDailySalt :one
SELECT salt FROM daily_salts WHERE day = ?;
-- name: ListDailyStats :many
SELECT * FROM daily_stats WHERE day >= ? AND day <= ? ORDER BY day,dimension,value;
-- name: PutDailySalt :exec
INSERT INTO daily_salts(day,salt) VALUES(?,?) ON CONFLICT(day) DO UPDATE SET salt=excluded.salt;
-- name: ListPageViewsBetween :many
SELECT * FROM page_views WHERE created_at >= ? AND created_at < ? ORDER BY created_at;
-- name: DeletePageViewsBefore :exec
DELETE FROM page_views WHERE created_at < ?;
-- name: DeleteDailyStatsDay :exec
DELETE FROM daily_stats WHERE day = ?;
-- name: PutDailyStat :exec
INSERT INTO daily_stats(day,dimension,value,views,visitors) VALUES(?,?,?,?,?) ON CONFLICT(day,dimension,value) DO UPDATE SET views=excluded.views,visitors=excluded.visitors;
