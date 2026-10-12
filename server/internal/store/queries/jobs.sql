-- name: MarkJobGeneration :exec
INSERT INTO job_merge(kind,subject_id,generation,queued_generation,updated_at) VALUES(?,?,1,0,?) ON CONFLICT(kind,subject_id) DO UPDATE SET generation=generation+1,updated_at=excluded.updated_at;
-- name: GetJobGeneration :one
SELECT * FROM job_merge WHERE kind = ? AND subject_id = ?;
-- name: SetQueuedGeneration :exec
UPDATE job_merge SET queued_generation = ? WHERE kind = ? AND subject_id = ?;
-- name: ClearQueuedGeneration :exec
UPDATE job_merge SET queued_generation = 0 WHERE kind = ? AND subject_id = ?;
