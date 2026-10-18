-- 只有完整引用证据才销毁退休材料；原版本与时刻独立审计，未知旧费用凭据保留。
CREATE TABLE credential_retirements(provider_id TEXT NOT NULL,credential_version INTEGER NOT NULL,data_epoch TEXT NOT NULL,destroyed_at TEXT NOT NULL,PRIMARY KEY(provider_id,credential_version,data_epoch));
ALTER TABLE header_secrets ADD COLUMN destroyed_at TEXT;
