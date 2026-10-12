-- name: ListDocuments :many
SELECT * FROM documents ORDER BY parent_id,sort_key,id;
-- name: GetDocument :one
SELECT * FROM documents WHERE id = ?;
-- name: UpsertDocument :exec
INSERT INTO documents(id,parent_id,title,slug,sort_key,file_path,path,published,revision,content_hash,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET parent_id=excluded.parent_id,title=excluded.title,slug=excluded.slug,sort_key=excluded.sort_key,file_path=excluded.file_path,path=excluded.path,published=excluded.published,revision=excluded.revision,content_hash=excluded.content_hash,updated_at=excluded.updated_at;
-- name: DeleteDocumentIndex :exec
DELETE FROM documents WHERE id = ?;
-- name: DeleteDocumentBlocks :exec
DELETE FROM blocks WHERE document_id = ?;
-- name: InsertBlock :exec
INSERT INTO blocks(id,document_id,parent_id,kind,position,heading_path,text,content_hash) VALUES(?,?,?,?,?,?,?,?);
-- name: ListDocumentBlocks :many
SELECT * FROM blocks WHERE document_id = ? ORDER BY position;
-- name: UpsertAsset :exec
INSERT INTO assets(hash,kind,extension,mime_type,size,original_name,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(hash) DO NOTHING;
-- name: GetAsset :one
SELECT * FROM assets WHERE hash = ?;
-- name: ListAssets :many
SELECT assets.*, (SELECT COUNT(*) FROM asset_references WHERE asset_hash=assets.hash) AS reference_count FROM assets ORDER BY created_at DESC;
-- name: DeleteAsset :exec
DELETE FROM assets WHERE hash = ?;
-- name: ClearAssetReferences :exec
DELETE FROM asset_references;
-- name: AddAssetReference :exec
INSERT OR IGNORE INTO asset_references(asset_hash,document_id,block_id,source) VALUES(?,?,?,?);
