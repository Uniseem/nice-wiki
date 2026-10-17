-- 全站来源扫描水位；只固定分页一致性，不持久化可写读快照，不从正文猜补引用。
-- 任一相关权威变化均在原业务事务内推进；修订上限耗尽即回滚原写入。
CREATE TABLE reference_scan_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    revision INTEGER NOT NULL CHECK (revision BETWEEN 1 AND 9007199254740991)
);
INSERT INTO reference_scan_state(singleton,revision) VALUES(1,1);
CREATE TRIGGER reference_scan_documents_insert
AFTER INSERT ON documents
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_documents_update
AFTER UPDATE ON documents
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_documents_delete
AFTER DELETE ON documents
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_document_metadata_insert
AFTER INSERT ON document_metadata
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_document_metadata_update
AFTER UPDATE ON document_metadata
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_document_metadata_delete
AFTER DELETE ON document_metadata
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_heads_insert
AFTER INSERT ON draft_heads
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_heads_update
AFTER UPDATE ON draft_heads
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_heads_delete
AFTER DELETE ON draft_heads
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_resource_references_insert
AFTER INSERT ON draft_resource_references
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_resource_references_update
AFTER UPDATE ON draft_resource_references
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_resource_references_delete
AFTER DELETE ON draft_resource_references
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_operation_intents_insert
AFTER INSERT ON draft_operation_intents
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_operation_intents_update
AFTER UPDATE ON draft_operation_intents
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_operation_intents_delete
AFTER DELETE ON draft_operation_intents
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_committed_revisions_insert
AFTER INSERT ON committed_revisions
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_committed_revisions_update
AFTER UPDATE ON committed_revisions
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_committed_revisions_delete
AFTER DELETE ON committed_revisions
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_commit_receipts_insert
AFTER INSERT ON draft_commit_receipts
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_commit_receipts_update
AFTER UPDATE ON draft_commit_receipts
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_draft_commit_receipts_delete
AFTER DELETE ON draft_commit_receipts
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_revision_preparations_insert
AFTER INSERT ON revision_preparations
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_revision_preparations_update
AFTER UPDATE ON revision_preparations
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_revision_preparations_delete
AFTER DELETE ON revision_preparations
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_document_quarantines_insert
AFTER INSERT ON document_quarantines
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_document_quarantines_update
AFTER UPDATE ON document_quarantines
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_document_quarantines_delete
AFTER DELETE ON document_quarantines
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_store_identity_insert
AFTER INSERT ON store_identity
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_store_identity_update
AFTER UPDATE ON store_identity
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
CREATE TRIGGER reference_scan_store_identity_delete
AFTER DELETE ON store_identity
BEGIN
    UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1;
END;
