-- name: CreateSession :exec
INSERT INTO sessions(token_hash,account_id,created_at,expires_at,last_seen_at) VALUES(?,?,?,?,?);
-- name: GetSession :one
SELECT * FROM sessions WHERE token_hash = ? AND expires_at > ?;
-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = ?;
-- name: DeleteAccountSessions :exec
DELETE FROM sessions WHERE account_id = ?;
-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE account_id = ? AND token_hash != ?;
-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= ?;
-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?;
-- name: GetLoginFailure :one
SELECT * FROM login_failures WHERE scope = ? AND subject_hash = ?;
-- name: SaveLoginFailure :exec
INSERT INTO login_failures(scope,subject_hash,attempts,window_start,locked_until) VALUES(?,?,?,?,?) ON CONFLICT(scope,subject_hash) DO UPDATE SET attempts=excluded.attempts, window_start=excluded.window_start, locked_until=excluded.locked_until;
-- name: DeleteLoginFailure :exec
DELETE FROM login_failures WHERE scope = ? AND subject_hash = ?;
-- name: AddAudit :exec
INSERT INTO audit_log(account_id,account_name,action,target_id,created_at) VALUES(?,?,?,?,?);
-- name: ListAudit :many
SELECT * FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?;
