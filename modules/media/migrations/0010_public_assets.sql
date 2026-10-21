-- 原图→变体依赖：变体随原图公开；原图有当前引用时，变体也受清理保护。
CREATE TABLE asset_variants (
 parent_asset_id TEXT NOT NULL,
 variant_asset_id TEXT NOT NULL,
 PRIMARY KEY(parent_asset_id,variant_asset_id)
);
CREATE INDEX idx_asset_variants_variant ON asset_variants(variant_asset_id);
INSERT OR IGNORE INTO asset_variants(parent_asset_id,variant_asset_id)
 SELECT a.asset_id,json_extract(v.value,'$.asset.assetId') FROM assets a,json_each(a.asset_json,'$.imageVariants') v
 WHERE json_extract(v.value,'$.asset.assetId') IS NOT NULL;

-- 出现过在已发布页面上的资源版本。只有这些（和站点配置正在用的资源）允许匿名按 sha 读取、进公共缓存；
-- 草稿引用只防清理，不代表公开。撤回后不删：字节可能已经在公共缓存里。
CREATE TABLE public_assets (
 asset_id TEXT NOT NULL,
 sha256 TEXT NOT NULL,
 first_public_at TEXT NOT NULL,
 PRIMARY KEY(asset_id,sha256)
);

-- 清理保护用的当前引用：资源自身的当前引用，加上原图的当前引用（对变体）。
CREATE VIEW asset_current_references AS
 SELECT asset_id FROM reference_entries WHERE kind='current'
 UNION ALL
 SELECT v.variant_asset_id FROM asset_variants v JOIN reference_entries e ON e.asset_id=v.parent_asset_id WHERE e.kind='current';
