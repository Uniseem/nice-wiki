-- 基线迁移：由历史迁移重放得到的最终结构（0001_baseline.sql 至 0001_baseline.sql）。
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    sha256 TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE store_identity (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch TEXT NOT NULL,
    site_id TEXT
);
CREATE TABLE revision_preparations (
    commit_id TEXT PRIMARY KEY,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('preparing', 'prepared', 'committed', 'aborted', 'quarantined')),
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    UNIQUE (caller_service, actor_id, site_id, data_epoch, operation_id),
    UNIQUE (site_id, revision_id)
);
CREATE TABLE committed_revisions (
    commit_id TEXT PRIMARY KEY REFERENCES revision_preparations(commit_id),
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('committed', 'quarantined')),
    UNIQUE (site_id, revision_id)
);
CREATE TABLE draft_commit_receipts (
    commit_id TEXT PRIMARY KEY REFERENCES committed_revisions(commit_id),
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    UNIQUE (caller_service, actor_id, site_id, data_epoch, operation_id)
);
CREATE TABLE draft_heads (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    draft_version INTEGER NOT NULL CHECK (draft_version > 0),
    PRIMARY KEY (site_id, document_id)
);
CREATE TABLE document_quarantines (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    quarantined_at TEXT NOT NULL,
    PRIMARY KEY (site_id, document_id)
);
CREATE TABLE documents (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('draft','published','trash','purged')),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    delete_fence INTEGER NOT NULL CHECK (delete_fence BETWEEN 1 AND 9007199254740991), reference_version INTEGER NOT NULL DEFAULT 0
    CHECK (reference_version BETWEEN 0 AND 9007199254740991),
    PRIMARY KEY (site_id,document_id)
);
CREATE TABLE edit_lock_receipts (
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    scope TEXT NOT NULL,
    document_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    result_hash TEXT NOT NULL,
    PRIMARY KEY (caller_service,actor_id,site_id,data_epoch,operation_id,scope)
);
CREATE TABLE edit_lock_event_revisions (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision BETWEEN 1 AND 9007199254740991),
    PRIMARY KEY (site_id,document_id)
);
CREATE TABLE edit_lock_audit (
    event_id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_kind TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    server_boot_id TEXT NOT NULL,
    fencing_token INTEGER NOT NULL,
    action TEXT NOT NULL,
    affected_account_id TEXT,
    occurred_at TEXT NOT NULL
);
CREATE TABLE content_tree_state (
    site_id TEXT PRIMARY KEY,
    revision INTEGER NOT NULL CHECK (revision BETWEEN 0 AND 9007199254740991)
);
CREATE TABLE document_metadata (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    metadata_json TEXT NOT NULL CHECK (json_valid(metadata_json)),
    PRIMARY KEY (site_id,document_id),
    FOREIGN KEY (site_id,document_id) REFERENCES documents(site_id,document_id)
);
CREATE TABLE draft_operation_intents (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('create','save')),
    request_hash TEXT NOT NULL,
    plan_json TEXT NOT NULL CHECK (json_valid(plan_json)),
    state TEXT NOT NULL CHECK (state IN ('pending','committed','aborted','rejected')),
    pin_id TEXT,
    result_json TEXT CHECK (result_json IS NULL OR json_valid(result_json)),
    PRIMARY KEY (site_id,data_epoch,operation_id)
);
CREATE TABLE document_id_ledger (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    stable_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('active','retired')),
    introduced_revision_id TEXT NOT NULL,
    retired_revision_id TEXT,
    PRIMARY KEY (site_id,document_id,stable_id),
    FOREIGN KEY (site_id,document_id) REFERENCES documents(site_id,document_id)
);
CREATE TABLE draft_resource_references (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    source_version INTEGER NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('current_draft','history')),
    references_json TEXT NOT NULL CHECK (json_valid(references_json)),
    PRIMARY KEY (site_id,document_id,revision_id),
    FOREIGN KEY (site_id,document_id) REFERENCES documents(site_id,document_id)
);
CREATE TABLE reference_scan_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    revision INTEGER NOT NULL CHECK (revision BETWEEN 1 AND 9007199254740991)
);
CREATE TABLE tree_operation_receipts (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('move','reorder')),
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    PRIMARY KEY (site_id,data_epoch,operation_id)
);
CREATE TABLE metadata_operation_receipts (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK(json_valid(result_json)),
    result_hash TEXT NOT NULL,
    PRIMARY KEY(site_id,data_epoch,operation_id)
);
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
CREATE TABLE model_suggestions (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    task_id TEXT NOT NULL,
    input_key TEXT NOT NULL,
    output_text TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(site_id,data_epoch,task_id,input_key)
);
CREATE TABLE purged_reference_releases (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    PRIMARY KEY(site_id,document_id)
);
CREATE UNIQUE INDEX publication_one_current_reference
    ON publication_resource_references(site_id,document_id) WHERE kind = 'current_published';
CREATE INDEX publication_references_by_document
    ON publication_resource_references(site_id,document_id,kind);
INSERT INTO "reference_scan_state"("singleton","revision") VALUES(1,1);
