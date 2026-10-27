-- 备份目标新增三种海外网盘：Google Drive、OneDrive、Dropbox。
ALTER TABLE backup_targets DROP CONSTRAINT IF EXISTS backup_targets_kind_check;
ALTER TABLE backup_targets ADD CONSTRAINT backup_targets_kind_check CHECK (kind IN ('s3','webdav','gdrive','onedrive','dropbox'));
