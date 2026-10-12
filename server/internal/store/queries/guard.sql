-- name: GetActiveBan :one
SELECT * FROM ip_bans WHERE ip_hash = ? AND expires_at > ?;
-- name: ListSuspiciousEvents :many
SELECT * FROM suspicious_events ORDER BY id DESC LIMIT ?;
