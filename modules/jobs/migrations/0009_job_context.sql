-- schemaVersion 9：队列保存安全触发上下文；历史任务保留空来源，不猜测。
ALTER TABLE jobs ADD COLUMN trigger_event TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN target_language TEXT NOT NULL DEFAULT '';
CREATE INDEX jobs_management_recent ON jobs(data_epoch,created_at DESC,job_id DESC);
PRAGMA user_version=9;
