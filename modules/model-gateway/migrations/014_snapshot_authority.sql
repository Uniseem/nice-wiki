-- 只有实际O_EXCL新建文件的可信启动路径生产birth；迁移绝不从空行创建首次历史证明。
CREATE TABLE database_origin(singleton INTEGER PRIMARY KEY CHECK(singleton=1),proof_json BLOB NOT NULL,proof_hash TEXT NOT NULL);
CREATE TRIGGER database_origin_immutable_update BEFORE UPDATE ON database_origin BEGIN SELECT RAISE(ABORT,'database_origin_immutable'); END;
CREATE TRIGGER database_origin_immutable_delete BEFORE DELETE ON database_origin BEGIN SELECT RAISE(ABORT,'database_origin_immutable'); END;
-- 恢复保持原费用/执行/输出hash和source epoch；单独记录完整已认证source→target只读授权。
CREATE TABLE restored_epoch_manifests(manifest_hash TEXT PRIMARY KEY,manifest_json BLOB NOT NULL);
CREATE TABLE restored_epoch_sources(source_epoch TEXT NOT NULL,target_epoch TEXT NOT NULL,manifest_hash TEXT NOT NULL REFERENCES restored_epoch_manifests(manifest_hash),proof_hash TEXT NOT NULL,PRIMARY KEY(source_epoch,target_epoch));
CREATE TABLE snapshot_material_redactions(kind TEXT NOT NULL CHECK(kind IN('credential','header')),record_key TEXT NOT NULL,source_database_hash TEXT NOT NULL,source_material_hash TEXT NOT NULL,PRIMARY KEY(kind,record_key));
