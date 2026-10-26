-- 基线迁移：由历史迁移重放得到的最终结构（001_initial.sql 至 016_site_overview_english.sql）。
CREATE TABLE index_identity (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), site_id TEXT NOT NULL, data_epoch TEXT NOT NULL
);
CREATE TABLE builds (
 build_id TEXT PRIMARY KEY, site_id TEXT NOT NULL, data_epoch TEXT NOT NULL,
 document_id TEXT NOT NULL, document_json BLOB NOT NULL, space_json BLOB NOT NULL, space_id TEXT NOT NULL,
 language TEXT NOT NULL, embedding_profile_id TEXT NOT NULL, state TEXT NOT NULL, generation INTEGER NOT NULL,
 lease_id TEXT NOT NULL DEFAULT '', lease_until TEXT NOT NULL DEFAULT '',
 cancelled INTEGER NOT NULL DEFAULT 0 CHECK(cancelled IN (0,1)),
 completed_chunks INTEGER NOT NULL DEFAULT 0, total_chunks INTEGER NOT NULL DEFAULT 0,
 failure_code TEXT NOT NULL DEFAULT '', manifest_hash TEXT NOT NULL DEFAULT '',
 generation_state TEXT NOT NULL DEFAULT 'staging', validated_at TEXT NOT NULL DEFAULT '', visibility_revision INTEGER NOT NULL DEFAULT 0, config_json BLOB, created_at TEXT NOT NULL DEFAULT '', retired_at_ns INTEGER NOT NULL DEFAULT 0, workflow_version INTEGER NOT NULL DEFAULT 0,
 UNIQUE(site_id,data_epoch,build_id,generation)
);
CREATE TABLE chunks (
 build_id TEXT NOT NULL REFERENCES builds(build_id), chunk_id TEXT NOT NULL,
 chunk_hash TEXT NOT NULL, chunk_json BLOB NOT NULL, ordinal INTEGER NOT NULL,
 PRIMARY KEY(build_id,chunk_id), UNIQUE(build_id,ordinal)
);
CREATE TABLE terms (
 build_id TEXT NOT NULL, chunk_id TEXT NOT NULL, language TEXT NOT NULL,
 tokenizer_version TEXT NOT NULL, terms_json BLOB NOT NULL, length INTEGER NOT NULL,
 PRIMARY KEY(build_id,chunk_id), FOREIGN KEY(build_id,chunk_id) REFERENCES chunks(build_id,chunk_id)
);
CREATE TABLE vectors (
 build_id TEXT NOT NULL, chunk_id TEXT NOT NULL, chunk_hash TEXT NOT NULL,
 space_json BLOB NOT NULL, vector_json BLOB NOT NULL, vector_hash TEXT NOT NULL,
 PRIMARY KEY(build_id,chunk_id), FOREIGN KEY(build_id,chunk_id) REFERENCES chunks(build_id,chunk_id)
);
CREATE TABLE active_indexes (
 site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, document_id TEXT NOT NULL,
 space_id TEXT NOT NULL, build_id TEXT NOT NULL REFERENCES builds(build_id),
 generation INTEGER NOT NULL, visibility_revision INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,document_id,space_id)
);
CREATE TABLE query_embeddings (
 site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, root_operation_id TEXT NOT NULL,
 query_hash TEXT NOT NULL, space_id TEXT NOT NULL, space_json BLOB NOT NULL,
 operation_id TEXT NOT NULL, result_json BLOB,
 PRIMARY KEY(site_id,data_epoch,root_operation_id,query_hash,space_id)
);
CREATE TABLE query_embedding_keys (
 site_id TEXT NOT NULL, data_epoch TEXT NOT NULL, actor_id TEXT NOT NULL,
 idempotency_key TEXT NOT NULL, payload_hash TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,actor_id,idempotency_key)
);
CREATE TABLE index_config (
 site_id TEXT PRIMARY KEY,data_epoch TEXT NOT NULL,config_id TEXT NOT NULL,
 version INTEGER NOT NULL,config_json BLOB NOT NULL
);
CREATE TABLE operation_receipts (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,actor_id TEXT NOT NULL,
 function_id TEXT NOT NULL,idempotency_key TEXT NOT NULL,input_hash TEXT NOT NULL,
 result_json BLOB NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,actor_id,function_id,idempotency_key)
);
CREATE TABLE build_dispatches (
 build_id TEXT PRIMARY KEY REFERENCES builds(build_id),input_json BLOB NOT NULL,
 input_ref_json BLOB NOT NULL,operation_json BLOB NOT NULL,dedupe_key TEXT NOT NULL,
 job_id TEXT,dispatch_state TEXT NOT NULL DEFAULT 'pending',cancel_requested INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL
, jobs_operation_id TEXT);
CREATE TABLE document_watermarks (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,document_id TEXT NOT NULL,
 visibility_revision INTEGER NOT NULL,withdrawn INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,document_id)
);
CREATE TABLE model_steps (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,owner_id TEXT NOT NULL,step_key TEXT NOT NULL,
 input_hash TEXT NOT NULL,operation_id TEXT NOT NULL,invocation_id TEXT,
 state TEXT NOT NULL,result_json BLOB,created_at TEXT NOT NULL, result_hash TEXT NOT NULL DEFAULT '', semantic_key TEXT NOT NULL DEFAULT '', dispatch_started INTEGER NOT NULL DEFAULT 1 CHECK(dispatch_started IN (0,1)), request_json BLOB, request_hash TEXT NOT NULL DEFAULT '', observation_json BLOB, observation_hash TEXT NOT NULL DEFAULT '', not_accepted_json BLOB, not_accepted_hash TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(site_id,data_epoch,owner_id,step_key)
);
CREATE TABLE generation_watermarks (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,document_id TEXT NOT NULL,space_id TEXT NOT NULL,
 generation INTEGER NOT NULL,PRIMARY KEY(site_id,data_epoch,document_id,space_id)
);
CREATE TABLE build_sources (
 build_id TEXT PRIMARY KEY REFERENCES builds(build_id),document_json BLOB NOT NULL,
 content_hash TEXT NOT NULL,body_json BLOB NOT NULL,created_at TEXT NOT NULL
);
CREATE TABLE chunk_context_intents (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,context_hash TEXT NOT NULL,source_hash TEXT NOT NULL,
 build_id TEXT NOT NULL,root_operation_id TEXT NOT NULL,operation_id TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,context_hash,source_hash)
);
CREATE TABLE chunk_contexts (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,context_hash TEXT NOT NULL,source_hash TEXT NOT NULL,
 chunk_hash TEXT NOT NULL,prefix TEXT NOT NULL,prefix_hash TEXT NOT NULL,
 invocation_id TEXT NOT NULL,created_at TEXT NOT NULL, model_snapshot_hash TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(site_id,data_epoch,context_hash,source_hash)
);
CREATE TABLE extraction_source_views (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,publication_json BLOB NOT NULL,
 block_json BLOB NOT NULL,asset_json BLOB NOT NULL,source_path TEXT NOT NULL,
 source_text TEXT NOT NULL,source_hash TEXT NOT NULL,source_start INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,publication_json,block_json,asset_json,source_path)
);
CREATE TABLE build_context_plans (
 build_id TEXT PRIMARY KEY,plan_json BLOB NOT NULL,plan_hash TEXT NOT NULL,created_at TEXT NOT NULL
);
CREATE TABLE extractions (
 extraction_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
 cache_key TEXT NOT NULL,resource_key TEXT NOT NULL,owner_build_id TEXT NOT NULL,source_json BLOB NOT NULL,plan_json BLOB NOT NULL,
 workflow_version INTEGER NOT NULL DEFAULT 1,state TEXT NOT NULL,result_json BLOB,result_hash TEXT NOT NULL DEFAULT '',
 lease_id TEXT NOT NULL DEFAULT '',lease_until TEXT NOT NULL DEFAULT '',
 cancelled INTEGER NOT NULL DEFAULT 0,failure_code TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL
);
CREATE TABLE extraction_cache_intents (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,cache_key TEXT NOT NULL,extraction_id TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,cache_key)
);
CREATE TABLE extraction_origins (
 extraction_id TEXT NOT NULL,build_id TEXT NOT NULL,source_hash TEXT NOT NULL,source_json BLOB NOT NULL,
 PRIMARY KEY(extraction_id,build_id,source_hash)
);
CREATE TABLE extraction_dispatches (
 extraction_id TEXT PRIMARY KEY REFERENCES extractions(extraction_id),
 input_json BLOB NOT NULL,input_ref_json BLOB NOT NULL,operation_json BLOB NOT NULL,
 dedupe_key TEXT NOT NULL,job_id TEXT,jobs_operation_id TEXT NOT NULL DEFAULT '',
 dispatch_state TEXT NOT NULL DEFAULT 'pending',cancel_requested INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL
);
CREATE TABLE execution_records (
 job_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
 job_type TEXT NOT NULL,object_id TEXT NOT NULL,input_ref_json BLOB NOT NULL,input_json BLOB NOT NULL,
 operation_json BLOB NOT NULL,worker_id TEXT NOT NULL,fence INTEGER NOT NULL,attempt INTEGER NOT NULL,
 lease_until TEXT NOT NULL,cancel_requested INTEGER NOT NULL DEFAULT 0,state TEXT NOT NULL,
 result_json BLOB,result_ref_json BLOB,paid_refs_json BLOB,
 UNIQUE(site_id,data_epoch,job_type,object_id)
);
CREATE TABLE build_execution_plans (
 build_id TEXT NOT NULL,phase TEXT NOT NULL,plan_json BLOB NOT NULL,plan_hash TEXT NOT NULL,
 PRIMARY KEY(build_id,phase)
);
CREATE TABLE extraction_recovered_results (
 extraction_id TEXT PRIMARY KEY REFERENCES extractions(extraction_id),
 artifact_json BLOB NOT NULL,artifact_hash TEXT NOT NULL,
 model_step_key TEXT NOT NULL,model_result_hash TEXT NOT NULL,created_at TEXT NOT NULL
);
CREATE TABLE rebuild_plans (
 plan_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,input_hash TEXT NOT NULL,snapshot_json BLOB NOT NULL,snapshot_hash TEXT NOT NULL,
 result_json BLOB NOT NULL,expires_at TEXT NOT NULL,
 approved_by TEXT,approved_at TEXT,approved_root TEXT,approved_result_json BLOB,build_ids_json BLOB,
 UNIQUE(site_id,data_epoch,actor_id,operation_id)
);
CREATE TABLE document_cards (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 document_id TEXT NOT NULL,
 source_hash TEXT NOT NULL,
 card_json TEXT NOT NULL,
 card_hash TEXT NOT NULL,
 origin TEXT NOT NULL CHECK (origin IN ('generated', 'kept')),
 updated_at TEXT NOT NULL,
 PRIMARY KEY (site_id, data_epoch, document_id)
);
CREATE TABLE tree_summaries (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 document_id TEXT NOT NULL,
 input_hash TEXT NOT NULL,
 summary TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY (site_id, data_epoch, document_id)
);
CREATE TABLE site_overview (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 input_hash TEXT NOT NULL,
 overview TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 checked_at TEXT NOT NULL, overview_en TEXT NOT NULL DEFAULT '',
 PRIMARY KEY (site_id, data_epoch)
);
CREATE INDEX builds_source ON builds(site_id,data_epoch,document_id,space_id,generation);
CREATE UNIQUE INDEX model_step_semantic ON model_steps(site_id,data_epoch,owner_id,semantic_key) WHERE semantic_key<>'';
