-- V1原链不改写；新事实采用版本2完整来源载荷。
ALTER TABLE analytics_meta ADD COLUMN reporting_timezone TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_records ADD COLUMN producer TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_records ADD COLUMN source_operation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_records ADD COLUMN hash_schema_version INTEGER NOT NULL DEFAULT 1 CHECK(hash_schema_version IN(1,2));
ALTER TABLE audit_records ADD COLUMN origin_mode TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE audit_records ADD COLUMN input_hash TEXT NOT NULL DEFAULT '';
CREATE TABLE beacon_period_watermarks(kind TEXT NOT NULL,period_key TEXT NOT NULL,revision INTEGER NOT NULL,PRIMARY KEY(kind,period_key));
ALTER TABLE audit_records ADD COLUMN origin_epoch TEXT NOT NULL DEFAULT '';
UPDATE audit_records SET origin_epoch=data_epoch;
ALTER TABLE audit_records ADD COLUMN origin_site_id TEXT NOT NULL DEFAULT '';
UPDATE audit_records SET origin_site_id=(SELECT site_id FROM analytics_meta WHERE singleton=1);
