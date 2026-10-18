-- 稳定blockId跨译文/语言复用，身份必须包含translation_id。
ALTER TABLE translation_units RENAME TO translation_units_legacy;
CREATE TABLE translation_units (
 unit_id TEXT NOT NULL,translation_id TEXT NOT NULL,document_id TEXT NOT NULL,
 source_hash TEXT NOT NULL,source_text TEXT NOT NULL,target_language TEXT NOT NULL,
 target_file_ref TEXT,target_text TEXT NOT NULL,target_hash TEXT NOT NULL,
 origin TEXT NOT NULL CHECK(origin IN('machine','human')),protected INTEGER NOT NULL CHECK(protected IN(0,1)),
 version INTEGER NOT NULL CHECK(version>=0),human_generation INTEGER NOT NULL CHECK(human_generation>=0),
 status TEXT NOT NULL CHECK(status IN('normal','review','fallback')),previous_target_text TEXT,
 data_epoch TEXT NOT NULL,updated_at TEXT NOT NULL,PRIMARY KEY(translation_id,unit_id)
);
INSERT INTO translation_units SELECT
 unit_id,CASE WHEN translation_id<>'' AND EXISTS(SELECT 1 FROM translation_manifests m WHERE m.translation_id=translation_units_legacy.translation_id AND json_extract(m.publication_ref,'$.documentId')=translation_units_legacy.document_id) THEN translation_id ELSE NULL END,
 document_id,source_hash,source_text,target_language,target_file_ref,target_text,target_hash,origin,protected,version,human_generation,status,previous_target_text,data_epoch,updated_at
 FROM translation_units_legacy;
DROP TABLE translation_units_legacy;
CREATE INDEX idx_translation_units_translation ON translation_units(translation_id,unit_id);
ALTER TABLE translation_manifests ADD COLUMN file_ref TEXT;
