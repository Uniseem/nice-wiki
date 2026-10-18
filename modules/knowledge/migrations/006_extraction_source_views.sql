-- 资源派生证据的不可变原文视图与精确发布/原块授权关联；与人工正文视图分开。
CREATE TABLE extraction_source_views (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,publication_json BLOB NOT NULL,
 block_json BLOB NOT NULL,asset_json BLOB NOT NULL,source_path TEXT NOT NULL,
 source_text TEXT NOT NULL,source_hash TEXT NOT NULL,source_start INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,publication_json,block_json,asset_json,source_path)
);
