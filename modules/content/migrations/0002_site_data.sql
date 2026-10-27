-- 站点数据：备份目标与备份记录、Git 同步配置（密钥经主密钥加密后存放，不记明文）。
CREATE TABLE backup_targets (
  target_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('s3','webdav')),
  name TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  bucket TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  path_prefix TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  secret_nonce BLOB,
  secret_cipher BLOB,
  schedule TEXT NOT NULL DEFAULT 'off' CHECK (schedule IN ('off','daily','weekly')),
  keep INTEGER NOT NULL DEFAULT 7,
  last_run_at TEXT NOT NULL DEFAULT '',
  last_status TEXT NOT NULL DEFAULT '',
  last_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX backup_targets_site ON backup_targets(site_id);
CREATE TABLE backup_runs (
  run_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL,
  target_id TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  object_name TEXT NOT NULL DEFAULT '',
  bytes INTEGER NOT NULL DEFAULT 0,
  message TEXT NOT NULL DEFAULT ''
);
CREATE INDEX backup_runs_site ON backup_runs(site_id, started_at);
CREATE TABLE git_remotes (
  site_id TEXT PRIMARY KEY,
  url TEXT NOT NULL,
  branch TEXT NOT NULL DEFAULT 'main',
  username TEXT NOT NULL DEFAULT '',
  token_nonce BLOB,
  token_cipher BLOB,
  push_on_publish INTEGER NOT NULL DEFAULT 0,
  last_push_at TEXT NOT NULL DEFAULT '',
  last_pull_at TEXT NOT NULL DEFAULT '',
  last_status TEXT NOT NULL DEFAULT '',
  last_message TEXT NOT NULL DEFAULT '',
  last_commit TEXT NOT NULL DEFAULT '',
  last_hash TEXT NOT NULL DEFAULT ''
);
