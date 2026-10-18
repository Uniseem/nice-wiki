-- 软删除及不可回退的全站代数；不改动已应用迁移。
ALTER TABLE glossary ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0 CHECK(deleted IN(0,1));
CREATE UNIQUE INDEX idx_glossary_active_scope ON glossary(source_language,target_language,scope,COALESCE(document_id,''),source_term) WHERE deleted=0;
CREATE TABLE domain_versions (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),glossary_generation INTEGER NOT NULL,catalog_generation INTEGER NOT NULL
);
INSERT INTO domain_versions SELECT 1,COALESCE((SELECT MAX(generation) FROM glossary),0),COALESCE((SELECT MAX(generation) FROM interface_catalog),0);
CREATE TABLE interface_sources (
 key TEXT PRIMARY KEY,value_text TEXT NOT NULL,source_hash TEXT NOT NULL,placeholders TEXT NOT NULL
);
ALTER TABLE approved_proposals ADD COLUMN site_id TEXT NOT NULL DEFAULT '';
ALTER TABLE approved_proposals ADD COLUMN root_operation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE approved_proposals ADD COLUMN result_generation INTEGER NOT NULL DEFAULT 0;
