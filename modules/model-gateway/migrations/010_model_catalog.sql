-- 显式公开目录身份与不可变下载版本，不猜旧来源、不创建渠道/凭据/默认模型。
ALTER TABLE models ADD COLUMN catalog_provider_id TEXT;
ALTER TABLE models ADD COLUMN catalog_model_id TEXT;
CREATE TABLE model_catalog_downloads(version INTEGER PRIMARY KEY AUTOINCREMENT,data_epoch TEXT NOT NULL,source TEXT NOT NULL CHECK(source IN('remote','bundled')),source_hash TEXT NOT NULL,normalized_hash TEXT NOT NULL,normalized_json BLOB NOT NULL,downloaded_ns INTEGER NOT NULL);
CREATE INDEX model_catalog_latest ON model_catalog_downloads(data_epoch,version DESC);
CREATE TABLE model_catalog_bindings(model_id TEXT NOT NULL REFERENCES models(model_id),data_epoch TEXT NOT NULL,provider_id TEXT NOT NULL,source_model_id TEXT NOT NULL,source_hash TEXT NOT NULL,PRIMARY KEY(model_id,data_epoch));
