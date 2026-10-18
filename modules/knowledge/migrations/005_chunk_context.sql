-- 背景是机器派生结果，不改人工原文；跨构建同来源只拥有一个原付费意图。
CREATE TABLE chunk_context_intents (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,context_hash TEXT NOT NULL,source_hash TEXT NOT NULL,
 build_id TEXT NOT NULL,root_operation_id TEXT NOT NULL,operation_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,context_hash,source_hash)
);
CREATE TABLE chunk_contexts (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,context_hash TEXT NOT NULL,source_hash TEXT NOT NULL,
 chunk_hash TEXT NOT NULL,prefix TEXT NOT NULL,prefix_hash TEXT NOT NULL,
 invocation_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,context_hash,source_hash)
);
