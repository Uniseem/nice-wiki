-- 媒体文件可以放在 S3 兼容存储：local 在本机，pending 等后台搬到存储桶，s3 已在存储桶（本机副本已删）。
ALTER TABLE assets ADD COLUMN storage TEXT NOT NULL DEFAULT 'local';
