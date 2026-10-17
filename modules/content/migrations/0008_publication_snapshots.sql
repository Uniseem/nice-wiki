-- 发布快照、署名历史、已发布引用和版本化重定向清单。只追加，不改既有 CHECK。
-- 正文仍在草稿修订文件；published_revision_id 只映射该不可变文件。
-- manifest 每一版是完整快照，供同一事务内的 Move/Update 读取。

CREATE TABLE publication_operation_receipts (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('publish','withdraw','commit')),
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    result_hash TEXT NOT NULL,
    PRIMARY KEY (site_id,data_epoch,operation_id)
);

CREATE TABLE publication_snapshots (
    site_id TEXT NOT NULL,
    published_revision_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    draft_revision_id TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    revision INTEGER NOT NULL CHECK (revision BETWEEN 1 AND 9007199254740991),
    visibility_revision INTEGER NOT NULL CHECK (visibility_revision BETWEEN 1 AND 9007199254740991),
    data_epoch TEXT NOT NULL,
    PRIMARY KEY (site_id,published_revision_id)
);

CREATE TABLE publication_history (
    site_id TEXT NOT NULL,
    history_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    published_revision_id TEXT NOT NULL,
    author_account_id TEXT NOT NULL,
    author_account_name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    commit_mirror_state TEXT NOT NULL CHECK (commit_mirror_state IN ('pending','mirrored','failed')),
    git_commit_id TEXT,
    operation TEXT NOT NULL CHECK (operation IN ('publish','rollback')),
    reference_json TEXT NOT NULL CHECK (json_valid(reference_json)),
    PRIMARY KEY (site_id,history_id)
);

CREATE TABLE publication_resource_references (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    published_revision_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    source_version INTEGER NOT NULL CHECK (source_version BETWEEN 0 AND 9007199254740991),
    kind TEXT NOT NULL CHECK (kind IN ('current_published','history')),
    references_json TEXT NOT NULL CHECK (json_valid(references_json)),
    PRIMARY KEY (site_id,document_id,published_revision_id)
);
CREATE UNIQUE INDEX publication_one_current_reference
    ON publication_resource_references(site_id,document_id) WHERE kind = 'current_published';
CREATE INDEX publication_references_by_document
    ON publication_resource_references(site_id,document_id,kind);

CREATE TABLE redirect_manifest_heads (
    site_id TEXT PRIMARY KEY,
    revision INTEGER NOT NULL CHECK (revision BETWEEN 0 AND 9007199254740991)
);
CREATE TABLE redirect_manifest_entries (
    site_id TEXT NOT NULL,
    manifest_revision INTEGER NOT NULL CHECK (manifest_revision BETWEEN 1 AND 9007199254740991),
    old_path TEXT NOT NULL,
    document_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('redirect','gone')),
    target_document_id TEXT,
    tree_revision INTEGER NOT NULL,
    data_epoch TEXT NOT NULL,
    PRIMARY KEY (site_id,manifest_revision,old_path)
);

CREATE TRIGGER reference_scan_publication_snapshots_insert
AFTER INSERT ON publication_snapshots
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_snapshots_update
AFTER UPDATE ON publication_snapshots
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_snapshots_delete
AFTER DELETE ON publication_snapshots
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_history_insert
AFTER INSERT ON publication_history
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_history_update
AFTER UPDATE ON publication_history
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_history_delete
AFTER DELETE ON publication_history
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_resource_references_insert
AFTER INSERT ON publication_resource_references
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_resource_references_update
AFTER UPDATE ON publication_resource_references
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_publication_resource_references_delete
AFTER DELETE ON publication_resource_references
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
