-- 文档引用水位独立于草稿版本；未来发布/撤回也在同doc事务推进。
-- 旧行0只表示没有本轮资格，不从已有文件或JSONL猜补引用水位。
ALTER TABLE documents ADD COLUMN reference_version INTEGER NOT NULL DEFAULT 0
    CHECK (reference_version BETWEEN 0 AND 9007199254740991);
-- 草稿用例意图固定创建/保存材料；正文只在私有不可变文件，不复制到SQL。
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
