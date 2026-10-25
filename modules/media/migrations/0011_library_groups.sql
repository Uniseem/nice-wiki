-- schemaVersion 11：资源分组是管理元数据；物理文件由启动链迁移为sha256.后缀，引用保持稳定。
CREATE TABLE asset_groups(group_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,name TEXT NOT NULL,created_at TEXT NOT NULL,UNIQUE(site_id,name));
CREATE INDEX asset_group_members ON assets(site_id,json_extract(asset_json,'$.groupId'));
PRAGMA user_version=11;
