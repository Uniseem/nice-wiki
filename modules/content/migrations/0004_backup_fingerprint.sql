ALTER TABLE backup_targets ADD COLUMN last_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE backup_targets ADD COLUMN last_object TEXT NOT NULL DEFAULT '';
