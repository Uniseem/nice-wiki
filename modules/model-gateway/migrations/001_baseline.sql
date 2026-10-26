-- 基线迁移：由历史迁移重放得到的最终结构（001_baseline.sql 至 001_baseline.sql）。
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE runtime_meta (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE TABLE providers (
    provider_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    adapter TEXT NOT NULL,
    base_url TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    credential_version INTEGER NOT NULL CHECK (credential_version >= 0),
    connect_timeout_ms INTEGER NOT NULL CHECK (connect_timeout_ms >= 0),
    request_timeout_ms INTEGER NOT NULL CHECK (request_timeout_ms >= 0),
    display_name TEXT NOT NULL,
    extra_header_names_json TEXT NOT NULL,
    extra_header_secret_refs_json TEXT NOT NULL,
    concurrency_limit INTEGER NOT NULL CHECK (concurrency_limit >= 0),
    stream_idle_timeout_ms INTEGER NOT NULL CHECK (stream_idle_timeout_ms >= 0),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE models (
    model_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    provider_id TEXT NOT NULL REFERENCES providers(provider_id),
    provider_model_version TEXT NOT NULL,
    capabilities_json TEXT NOT NULL,
    dimension INTEGER NOT NULL CHECK (dimension >= 0),
    preprocessor_version TEXT NOT NULL,
    max_input_tokens INTEGER NOT NULL CHECK (max_input_tokens >= 0),
    max_output_tokens INTEGER NOT NULL CHECK (max_output_tokens >= 0),
    catalog_origin TEXT NOT NULL,
    manual_override INTEGER NOT NULL CHECK (manual_override IN (0, 1)),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
, catalog_provider_id TEXT, catalog_model_id TEXT);
CREATE TABLE operation_receipts (
    site_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    function_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    result_json TEXT NOT NULL,
    created_at TEXT NOT NULL, result_hash TEXT,
    PRIMARY KEY (site_id, actor_id, function_id, idempotency_key)
);
CREATE TABLE invocations (
    invocation_id TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    root_operation_id TEXT NOT NULL,
    cost_class TEXT NOT NULL,
    state TEXT NOT NULL,
    deadline TEXT NOT NULL,
    price_snapshot_json TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
, owner_service TEXT, site_id TEXT, created_at TEXT, invocation_json TEXT, invocation_hash TEXT, request_json TEXT, operation_json TEXT, route_json TEXT, profile_json TEXT, fee_binding_json TEXT, active_attempt_id TEXT, lease_id TEXT NOT NULL DEFAULT '', lease_until_ns INTEGER NOT NULL DEFAULT 0, cancel_requested INTEGER NOT NULL DEFAULT 0, created_ns INTEGER, terminal_ns INTEGER NOT NULL DEFAULT 0, terminal_output_json TEXT, execution_hash TEXT, create_command_key TEXT);
CREATE TABLE reservations (
    reservation_id TEXT PRIMARY KEY,
    invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
    maximum_micro_usd INTEGER,
    held_micro_usd INTEGER,
    state TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 0),
    pricing_known INTEGER NOT NULL CHECK (pricing_known IN (0, 1)),
    unknown_usage_count INTEGER NOT NULL CHECK (unknown_usage_count >= 0)
, attempt_id TEXT);
CREATE TABLE ledger_entries (
    entry_id TEXT PRIMARY KEY,
    invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
    provider_request_id TEXT NOT NULL,
    actual_micro_usd INTEGER,
    input_tokens INTEGER NOT NULL CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL CHECK (output_tokens >= 0),
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    pricing_known INTEGER NOT NULL CHECK (pricing_known IN (0, 1)),
    usage_hash TEXT NOT NULL
, supersedes_entry_id TEXT, price_snapshot_json TEXT);
CREATE TABLE stream_events (
    invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    event TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (invocation_id, sequence)
);
CREATE TABLE barriers (
    barrier_id TEXT PRIMARY KEY,
    state TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    deadline TEXT NOT NULL
);
CREATE TABLE credentials (
    provider_id TEXT NOT NULL REFERENCES providers(provider_id),
    credential_version INTEGER NOT NULL CHECK (credential_version >= 1),
    nonce BLOB NOT NULL CHECK (typeof(nonce) = 'blob' AND length(nonce) = 12),
    ciphertext BLOB NOT NULL CHECK (typeof(ciphertext) = 'blob' AND length(ciphertext) > 16),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    PRIMARY KEY (provider_id, credential_version)
);
CREATE TABLE fee_budget_policy (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    site_id TEXT NOT NULL CHECK (length(site_id) > 0),
    version INTEGER NOT NULL CHECK (version BETWEEN 1 AND 9007199254740991),
    hourly_limits_enabled INTEGER NOT NULL CHECK (hourly_limits_enabled IN (0, 1)),
    session_hourly_limit_micro_usd INTEGER NOT NULL CHECK (session_hourly_limit_micro_usd BETWEEN 0 AND 9007199254740991),
    network_multiplier INTEGER NOT NULL CHECK (network_multiplier BETWEEN 1 AND 100),
    daily_qa_limit_micro_usd INTEGER NOT NULL CHECK (daily_qa_limit_micro_usd BETWEEN 0 AND 9007199254740991),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL
, daily_probe_limit_micro_usd INTEGER NOT NULL DEFAULT 0 CHECK(daily_probe_limit_micro_usd BETWEEN 0 AND 9007199254740991));
CREATE TABLE fee_policy_audit (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    policy_version INTEGER NOT NULL CHECK (policy_version BETWEEN 1 AND 9007199254740991),
    actor_id TEXT NOT NULL,
    root_operation_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    daily_limit_changed INTEGER NOT NULL CHECK (daily_limit_changed IN (0, 1)),
    PRIMARY KEY (site_id, data_epoch, policy_version)
);
CREATE TABLE price_book_entries (
    price_snapshot_id TEXT PRIMARY KEY,
    model_id TEXT NOT NULL REFERENCES models(model_id),
    model_version INTEGER NOT NULL CHECK(model_version BETWEEN 1 AND 9007199254740991),
    data_epoch TEXT NOT NULL,
    source TEXT NOT NULL CHECK(source IN ('manual','remote','bundled')),
    effective_seconds INTEGER NOT NULL,
    effective_nanos INTEGER NOT NULL CHECK(effective_nanos BETWEEN 0 AND 999999999),
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    snapshot_hash TEXT NOT NULL,
    UNIQUE(model_id,data_epoch,source,model_version)
);
CREATE TABLE ledger_read_state (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1)
);
CREATE TABLE invocation_attempts(
 attempt_id TEXT PRIMARY KEY,invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),ordinal INTEGER NOT NULL,
 attempt_json TEXT NOT NULL,model_json TEXT NOT NULL,provider_json TEXT NOT NULL,credential_version INTEGER NOT NULL,
 price_snapshot_json TEXT NOT NULL,lease_id TEXT NOT NULL DEFAULT '',lease_until_ns INTEGER NOT NULL DEFAULT 0,
 dispatched_ns INTEGER NOT NULL DEFAULT 0,usage_hash TEXT,input_token_budget INTEGER NOT NULL,output_token_budget INTEGER NOT NULL,UNIQUE(invocation_id,ordinal)
);
CREATE TABLE invocation_usage(
 provider_id TEXT NOT NULL,provider_request_id TEXT NOT NULL,invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
 attempt_id TEXT NOT NULL REFERENCES invocation_attempts(attempt_id),usage_hash TEXT NOT NULL,usage_json TEXT NOT NULL,
 conflict_hash TEXT,created_ns INTEGER NOT NULL,PRIMARY KEY(provider_id,provider_request_id)
);
CREATE TABLE fee_commitments(
 reservation_id TEXT PRIMARY KEY REFERENCES reservations(reservation_id),attempt_id TEXT NOT NULL UNIQUE,
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,cost_class TEXT NOT NULL,profile_id TEXT NOT NULL,root_operation_id TEXT NOT NULL,
 session_scope TEXT NOT NULL,network_scope TEXT NOT NULL,created_ns INTEGER NOT NULL,state TEXT NOT NULL,
 amount_micro_usd INTEGER,pricing_known INTEGER NOT NULL
);
CREATE TABLE execution_receipts(
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,owner_service TEXT NOT NULL,function_id TEXT NOT NULL,idempotency_key TEXT NOT NULL,
 input_hash TEXT NOT NULL,result_json TEXT NOT NULL,result_hash TEXT NOT NULL,created_ns INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,owner_service,function_id,idempotency_key)
);
CREATE TABLE provider_slots(
 lease_id TEXT PRIMARY KEY,provider_id TEXT NOT NULL,invocation_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
 expires_ns INTEGER NOT NULL,UNIQUE(provider_id,invocation_id)
);
CREATE TABLE credential_references(
 attempt_id TEXT PRIMARY KEY REFERENCES invocation_attempts(attempt_id),provider_id TEXT NOT NULL,credential_version INTEGER NOT NULL,
 data_epoch TEXT NOT NULL,released INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE reconciliation_proofs(attempt_id TEXT NOT NULL REFERENCES invocation_attempts(attempt_id),proof_hash TEXT NOT NULL,actor_id TEXT NOT NULL,proof_json TEXT NOT NULL,created_ns INTEGER NOT NULL,PRIMARY KEY(attempt_id,proof_hash));
CREATE TABLE ledger_voids(entry_id TEXT PRIMARY KEY REFERENCES ledger_entries(entry_id),proof_hash TEXT NOT NULL,actor_id TEXT NOT NULL,created_ns INTEGER NOT NULL);
CREATE TABLE route_credential_refs(invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),provider_id TEXT NOT NULL,credential_version INTEGER NOT NULL,data_epoch TEXT NOT NULL,released INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(invocation_id,provider_id,credential_version));
CREATE TABLE header_secrets(ref_id TEXT PRIMARY KEY,provider_id TEXT NOT NULL,provider_version INTEGER NOT NULL,header_name TEXT NOT NULL,data_epoch TEXT NOT NULL,nonce BLOB NOT NULL,ciphertext BLOB NOT NULL, destroyed_at TEXT);
CREATE TABLE route_header_refs(invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),ref_id TEXT NOT NULL REFERENCES header_secrets(ref_id),released INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(invocation_id,ref_id));
CREATE TABLE credential_tracking(singleton INTEGER PRIMARY KEY CHECK(singleton=1),enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN(0,1)));
CREATE TABLE attempt_evidence(attempt_id TEXT PRIMARY KEY REFERENCES invocation_attempts(attempt_id),outcome_json TEXT NOT NULL,outcome_hash TEXT NOT NULL,recorded_ns INTEGER NOT NULL);
CREATE TABLE credential_retirements(provider_id TEXT NOT NULL,credential_version INTEGER NOT NULL,data_epoch TEXT NOT NULL,destroyed_at TEXT NOT NULL,PRIMARY KEY(provider_id,credential_version,data_epoch));
CREATE TABLE model_catalog_downloads(version INTEGER PRIMARY KEY AUTOINCREMENT,data_epoch TEXT NOT NULL,source TEXT NOT NULL CHECK(source IN('remote','bundled')),source_hash TEXT NOT NULL,normalized_hash TEXT NOT NULL,normalized_json TEXT NOT NULL,downloaded_ns INTEGER NOT NULL);
CREATE TABLE model_catalog_bindings(model_id TEXT NOT NULL REFERENCES models(model_id),data_epoch TEXT NOT NULL,provider_id TEXT NOT NULL,source_model_id TEXT NOT NULL,source_hash TEXT NOT NULL,PRIMARY KEY(model_id,data_epoch));
CREATE TABLE invocation_lookup_tracking(singleton INTEGER PRIMARY KEY CHECK(singleton=1),format_version INTEGER NOT NULL CHECK(format_version=1),data_epoch TEXT NOT NULL,proof_kind TEXT NOT NULL CHECK(proof_kind IN('fresh_activation','verified_restore')),proof_hash TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE usage_statistics_heads(entry_id TEXT PRIMARY KEY,revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1));
CREATE TABLE usage_statistics_changes(change_revision INTEGER PRIMARY KEY AUTOINCREMENT,entry_id TEXT NOT NULL,ledger_revision INTEGER NOT NULL,site_id TEXT,source_epoch TEXT,snapshot_json TEXT NOT NULL,observed_at TEXT NOT NULL,UNIQUE(entry_id,ledger_revision));
CREATE TABLE connection_probes(invocation_id TEXT PRIMARY KEY REFERENCES invocations(invocation_id) DEFERRABLE INITIALLY DEFERRED,provider_id TEXT NOT NULL,model_id TEXT NOT NULL,data_epoch TEXT NOT NULL,created_ns INTEGER NOT NULL);
CREATE TABLE database_origin(singleton INTEGER PRIMARY KEY CHECK(singleton=1),proof_json TEXT NOT NULL,proof_hash TEXT NOT NULL);
CREATE TABLE restored_epoch_manifests(manifest_hash TEXT PRIMARY KEY,manifest_json TEXT NOT NULL);
CREATE TABLE restored_epoch_sources(source_epoch TEXT NOT NULL,target_epoch TEXT NOT NULL,manifest_hash TEXT NOT NULL REFERENCES restored_epoch_manifests(manifest_hash),proof_hash TEXT NOT NULL,PRIMARY KEY(source_epoch,target_epoch));
CREATE TABLE snapshot_material_redactions(kind TEXT NOT NULL CHECK(kind IN('credential','header')),record_key TEXT NOT NULL,source_database_hash TEXT NOT NULL,source_material_hash TEXT NOT NULL,PRIMARY KEY(kind,record_key));
CREATE TABLE gateway_snapshot_state(singleton INTEGER PRIMARY KEY CHECK(singleton=1),revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1));
CREATE TABLE usage_observation_heads(kind TEXT NOT NULL,entity_id TEXT NOT NULL,revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1),PRIMARY KEY(kind,entity_id));
CREATE TABLE usage_observation_changes(revision INTEGER PRIMARY KEY AUTOINCREMENT,kind TEXT NOT NULL,entity_id TEXT NOT NULL,entity_revision INTEGER NOT NULL,site_id TEXT,source_epoch TEXT,snapshot_json TEXT NOT NULL,deleted INTEGER NOT NULL CHECK(deleted IN(0,1)),observed_at TEXT NOT NULL,UNIQUE(kind,entity_id,entity_revision));
CREATE TABLE usage_statistics_timeline(sequence INTEGER PRIMARY KEY AUTOINCREMENT,kind TEXT NOT NULL CHECK(kind IN('ledger','observation')),source_revision INTEGER NOT NULL,UNIQUE(kind,source_revision));
CREATE TABLE "task_assignments" (
    task_id TEXT PRIMARY KEY,
    tier TEXT NOT NULL CHECK (tier IN ('fast', 'accurate', 'vision', 'embedding', 'rerank', 'decision')),
    override_model_id TEXT REFERENCES models(model_id),
    version INTEGER NOT NULL CHECK (version >= 1),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL
);
CREATE TABLE "profiles" (
    profile_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    cost_class TEXT NOT NULL CHECK (cost_class IN ('qa', 'background', 'admin-probe')),
    tier TEXT NOT NULL CHECK (tier IN ('fast', 'accurate', 'vision', 'embedding', 'rerank', 'decision')),
    ordered_model_ids_json TEXT NOT NULL,
    maximum_cost_micro_usd INTEGER NOT NULL CHECK (maximum_cost_micro_usd >= 0),
    concurrency INTEGER NOT NULL CHECK (concurrency >= 0),
    daily_limit_cost_micro_usd INTEGER NOT NULL CHECK (daily_limit_cost_micro_usd >= 0),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL,
    UNIQUE (data_epoch, cost_class, tier)
);
CREATE INDEX price_book_model_effective ON price_book_entries(model_id,data_epoch,source,effective_seconds,effective_nanos);
CREATE INDEX ledger_read_order ON ledger_entries(
 (substr(created_at,1,19)||'.'||substr((CASE WHEN substr(created_at,20,1)='.' THEN substr(created_at,21,length(created_at)-21) ELSE '' END)||'000000000',1,9)||'Z'),entry_id
);
CREATE INDEX invocation_root_read ON invocations(root_operation_id,invocation_id);
CREATE UNIQUE INDEX reservation_attempt ON reservations(attempt_id);
CREATE INDEX fee_commitment_scope ON fee_commitments(site_id,data_epoch,cost_class,state,created_ns);
CREATE UNIQUE INDEX ledger_provider_attempt ON ledger_entries(invocation_id,attempt_id,status,usage_hash);
CREATE INDEX model_catalog_latest ON model_catalog_downloads(data_epoch,version DESC);
CREATE UNIQUE INDEX invocation_create_command ON invocations(site_id,data_epoch,owner_service,create_command_key) WHERE create_command_key IS NOT NULL;
CREATE INDEX usage_statistics_site_page ON usage_statistics_changes(site_id,change_revision);
CREATE INDEX usage_statistics_missing_scope ON usage_statistics_changes(change_revision) WHERE site_id IS NULL OR site_id='' OR source_epoch IS NULL OR source_epoch='';
CREATE INDEX ledger_superseded_by ON ledger_entries(supersedes_entry_id);
CREATE INDEX connection_probe_latest ON connection_probes(provider_id,data_epoch,created_ns DESC,invocation_id DESC);
CREATE INDEX usage_observation_site ON usage_observation_changes(site_id,revision);
CREATE VIEW ledger_invalid_timestamps AS SELECT entry_id FROM ledger_entries WHERE (length(created_at) BETWEEN 20 AND 30
 AND substr(created_at,1,19) GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]'
 AND substr(created_at,-1)='Z'
 AND strftime('%Y-%m-%dT%H:%M:%S',substr(created_at,1,19),'+0 seconds') IS substr(created_at,1,19)
 AND (length(created_at)=20 OR (length(created_at) BETWEEN 22 AND 30 AND substr(created_at,20,1)='.' AND substr(created_at,21,length(created_at)-21) NOT GLOB '*[^0-9]*'))) IS NOT 1;
CREATE TRIGGER ledger_read_invocations_insert AFTER INSERT ON invocations
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_invocations_update AFTER UPDATE ON invocations
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_invocations_delete AFTER DELETE ON invocations
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_reservations_insert AFTER INSERT ON reservations
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_reservations_update AFTER UPDATE ON reservations
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_reservations_delete AFTER DELETE ON reservations
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_ledger_entries_insert AFTER INSERT ON ledger_entries
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_ledger_entries_update AFTER UPDATE ON ledger_entries
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_ledger_entries_delete AFTER DELETE ON ledger_entries
BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER usage_statistics_entries_insert AFTER INSERT ON ledger_entries
BEGIN
 INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM (SELECT NEW.entry_id AS entry_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.actual_micro_usd AS actual_micro_usd,NEW.pricing_known AS pricing_known,NEW.status AS status,NEW.created_at AS created_at,NEW.supersedes_entry_id AS supersedes_entry_id,NEW.price_snapshot_json AS price_snapshot_json) l WHERE 1
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM (SELECT NEW.entry_id AS entry_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.actual_micro_usd AS actual_micro_usd,NEW.pricing_known AS pricing_known,NEW.status AS status,NEW.created_at AS created_at,NEW.supersedes_entry_id AS supersedes_entry_id,NEW.price_snapshot_json AS price_snapshot_json) l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE 1;
INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=NEW.supersedes_entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=NEW.supersedes_entry_id;
END;
CREATE TRIGGER usage_statistics_entries_update AFTER UPDATE ON ledger_entries
BEGIN
 INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM (SELECT OLD.entry_id AS entry_id,OLD.invocation_id AS invocation_id,OLD.attempt_id AS attempt_id,OLD.actual_micro_usd AS actual_micro_usd,OLD.pricing_known AS pricing_known,OLD.status AS status,OLD.created_at AS created_at,OLD.supersedes_entry_id AS supersedes_entry_id,OLD.price_snapshot_json AS price_snapshot_json) l WHERE OLD.entry_id<>NEW.entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state','deleted','captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM (SELECT OLD.entry_id AS entry_id,OLD.invocation_id AS invocation_id,OLD.attempt_id AS attempt_id,OLD.actual_micro_usd AS actual_micro_usd,OLD.pricing_known AS pricing_known,OLD.status AS status,OLD.created_at AS created_at,OLD.supersedes_entry_id AS supersedes_entry_id,OLD.price_snapshot_json AS price_snapshot_json) l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE OLD.entry_id<>NEW.entry_id;
INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM (SELECT NEW.entry_id AS entry_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.actual_micro_usd AS actual_micro_usd,NEW.pricing_known AS pricing_known,NEW.status AS status,NEW.created_at AS created_at,NEW.supersedes_entry_id AS supersedes_entry_id,NEW.price_snapshot_json AS price_snapshot_json) l WHERE 1
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM (SELECT NEW.entry_id AS entry_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.actual_micro_usd AS actual_micro_usd,NEW.pricing_known AS pricing_known,NEW.status AS status,NEW.created_at AS created_at,NEW.supersedes_entry_id AS supersedes_entry_id,NEW.price_snapshot_json AS price_snapshot_json) l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE 1;
INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=OLD.supersedes_entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=OLD.supersedes_entry_id;
INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=NEW.supersedes_entry_id AND NEW.supersedes_entry_id IS NOT OLD.supersedes_entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=NEW.supersedes_entry_id AND NEW.supersedes_entry_id IS NOT OLD.supersedes_entry_id;
END;
CREATE TRIGGER usage_statistics_entries_delete AFTER DELETE ON ledger_entries
BEGIN
 INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM (SELECT OLD.entry_id AS entry_id,OLD.invocation_id AS invocation_id,OLD.attempt_id AS attempt_id,OLD.actual_micro_usd AS actual_micro_usd,OLD.pricing_known AS pricing_known,OLD.status AS status,OLD.created_at AS created_at,OLD.supersedes_entry_id AS supersedes_entry_id,OLD.price_snapshot_json AS price_snapshot_json) l WHERE 1
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state','deleted','captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM (SELECT OLD.entry_id AS entry_id,OLD.invocation_id AS invocation_id,OLD.attempt_id AS attempt_id,OLD.actual_micro_usd AS actual_micro_usd,OLD.pricing_known AS pricing_known,OLD.status AS status,OLD.created_at AS created_at,OLD.supersedes_entry_id AS supersedes_entry_id,OLD.price_snapshot_json AS price_snapshot_json) l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE 1;
INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=OLD.supersedes_entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=OLD.supersedes_entry_id;
END;
CREATE TRIGGER usage_statistics_void_insert AFTER INSERT ON ledger_voids
BEGIN
 INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=NEW.entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=NEW.entry_id;
END;
CREATE TRIGGER usage_statistics_void_update AFTER UPDATE ON ledger_voids
BEGIN
 INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=NEW.entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=NEW.entry_id;
INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=OLD.entry_id AND OLD.entry_id<>NEW.entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=OLD.entry_id AND OLD.entry_id<>NEW.entry_id;
END;
CREATE TRIGGER usage_statistics_void_delete AFTER DELETE ON ledger_voids
BEGIN
 INSERT INTO usage_statistics_heads(entry_id,revision) SELECT l.entry_id,1 FROM ledger_entries l WHERE l.entry_id=OLD.entry_id
 ON CONFLICT(entry_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'statistics_entry_revision_exhausted') END;
 INSERT INTO usage_statistics_changes(entry_id,ledger_revision,site_id,source_epoch,snapshot_json,observed_at)
 SELECT l.entry_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('ledgerEntryId',l.entry_id,'invocationId',l.invocation_id,'attemptId',l.attempt_id,'actualMicros',l.actual_micro_usd,'amountKnown',l.pricing_known,'ledgerStatus',l.status,'occurredAt',l.created_at,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'ownerService',i.owner_service,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'costClass',i.cost_class,'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(l.price_snapshot_json AS TEXT)) THEN CAST(l.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'executionHash',i.execution_hash,'state',CASE WHEN EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) THEN 'voided' WHEN EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) THEN 'superseded' ELSE l.status END,'captureKind','change'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
 FROM ledger_entries l JOIN usage_statistics_heads h ON h.entry_id=l.entry_id LEFT JOIN invocations i ON i.invocation_id=l.invocation_id LEFT JOIN invocation_attempts a ON a.attempt_id=l.attempt_id WHERE l.entry_id=OLD.entry_id;
END;
CREATE TRIGGER usage_statistics_changes_no_update BEFORE UPDATE ON usage_statistics_changes BEGIN SELECT RAISE(ABORT,'statistics_history_append_only'); END;
CREATE TRIGGER usage_statistics_changes_no_delete BEFORE DELETE ON usage_statistics_changes BEGIN SELECT RAISE(ABORT,'statistics_history_append_only'); END;
CREATE TRIGGER ledger_read_voids_insert AFTER INSERT ON ledger_voids BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_voids_update AFTER UPDATE ON ledger_voids BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER ledger_read_voids_delete AFTER DELETE ON ledger_voids BEGIN
 SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM ledger_read_state WHERE singleton=1) THEN RAISE(ABORT,'ledger_read_revision_missing') END;
 UPDATE ledger_read_state SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'ledger_read_revision_exhausted') END WHERE singleton=1;
END;
CREATE TRIGGER database_origin_immutable_update BEFORE UPDATE ON database_origin BEGIN SELECT RAISE(ABORT,'database_origin_immutable'); END;
CREATE TRIGGER database_origin_immutable_delete BEFORE DELETE ON database_origin BEGIN SELECT RAISE(ABORT,'database_origin_immutable'); END;
CREATE TRIGGER usage_timeline_ledger AFTER INSERT ON usage_statistics_changes BEGIN INSERT INTO usage_statistics_timeline(kind,source_revision) VALUES('ledger',NEW.change_revision); END;
CREATE TRIGGER usage_timeline_observation AFTER INSERT ON usage_observation_changes BEGIN INSERT INTO usage_statistics_timeline(kind,source_revision) VALUES('observation',NEW.revision); END;
CREATE TRIGGER usage_observation_invocation_insert AFTER INSERT ON invocations BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'invocation',i.invocation_id,1 FROM (SELECT NEW.invocation_id AS invocation_id,NEW.root_operation_id AS root_operation_id,NEW.owner_service AS owner_service,NEW.site_id AS site_id,NEW.operation_json AS operation_json,NEW.cost_class AS cost_class,NEW.execution_hash AS execution_hash,NEW.state AS state,NEW.created_at AS created_at,NEW.terminal_ns AS terminal_ns,NEW.lease_id AS lease_id,NEW.lease_until_ns AS lease_until_ns,NEW.invocation_json AS invocation_json) i WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'invocation',i.invocation_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('state',i.state,'createdAt',i.created_at,'terminalNS',i.terminal_ns,'leaseID',i.lease_id,'leaseUntilNS',i.lease_until_ns,'attemptCount',(SELECT COUNT(*) FROM invocation_attempts x WHERE x.invocation_id=i.invocation_id),'allAttemptsFinal',NOT EXISTS(SELECT 1 FROM invocation_attempts x WHERE x.invocation_id=i.invocation_id AND COALESCE(json_extract(CASE WHEN json_valid(CAST(x.attempt_json AS TEXT)) THEN CAST(x.attempt_json AS TEXT) ELSE '{}' END,'$.state'),'unknown') NOT IN('succeeded','failed','reconciled')))),0,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT NEW.invocation_id AS invocation_id,NEW.root_operation_id AS root_operation_id,NEW.owner_service AS owner_service,NEW.site_id AS site_id,NEW.operation_json AS operation_json,NEW.cost_class AS cost_class,NEW.execution_hash AS execution_hash,NEW.state AS state,NEW.created_at AS created_at,NEW.terminal_ns AS terminal_ns,NEW.lease_id AS lease_id,NEW.lease_until_ns AS lease_until_ns,NEW.invocation_json AS invocation_json) i JOIN usage_observation_heads h ON h.kind='invocation' AND h.entity_id=i.invocation_id WHERE 1;
END;
CREATE TRIGGER usage_observation_invocation_update AFTER UPDATE ON invocations BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'invocation',i.invocation_id,1 FROM (SELECT NEW.invocation_id AS invocation_id,NEW.root_operation_id AS root_operation_id,NEW.owner_service AS owner_service,NEW.site_id AS site_id,NEW.operation_json AS operation_json,NEW.cost_class AS cost_class,NEW.execution_hash AS execution_hash,NEW.state AS state,NEW.created_at AS created_at,NEW.terminal_ns AS terminal_ns,NEW.lease_id AS lease_id,NEW.lease_until_ns AS lease_until_ns,NEW.invocation_json AS invocation_json) i WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'invocation',i.invocation_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('state',i.state,'createdAt',i.created_at,'terminalNS',i.terminal_ns,'leaseID',i.lease_id,'leaseUntilNS',i.lease_until_ns,'attemptCount',(SELECT COUNT(*) FROM invocation_attempts x WHERE x.invocation_id=i.invocation_id),'allAttemptsFinal',NOT EXISTS(SELECT 1 FROM invocation_attempts x WHERE x.invocation_id=i.invocation_id AND COALESCE(json_extract(CASE WHEN json_valid(CAST(x.attempt_json AS TEXT)) THEN CAST(x.attempt_json AS TEXT) ELSE '{}' END,'$.state'),'unknown') NOT IN('succeeded','failed','reconciled')))),0,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT NEW.invocation_id AS invocation_id,NEW.root_operation_id AS root_operation_id,NEW.owner_service AS owner_service,NEW.site_id AS site_id,NEW.operation_json AS operation_json,NEW.cost_class AS cost_class,NEW.execution_hash AS execution_hash,NEW.state AS state,NEW.created_at AS created_at,NEW.terminal_ns AS terminal_ns,NEW.lease_id AS lease_id,NEW.lease_until_ns AS lease_until_ns,NEW.invocation_json AS invocation_json) i JOIN usage_observation_heads h ON h.kind='invocation' AND h.entity_id=i.invocation_id WHERE 1;
END;
CREATE TRIGGER usage_observation_invocation_delete AFTER DELETE ON invocations BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'invocation',i.invocation_id,1 FROM (SELECT OLD.invocation_id AS invocation_id,OLD.root_operation_id AS root_operation_id,OLD.owner_service AS owner_service,OLD.site_id AS site_id,OLD.operation_json AS operation_json,OLD.cost_class AS cost_class,OLD.execution_hash AS execution_hash,OLD.state AS state,OLD.created_at AS created_at,OLD.terminal_ns AS terminal_ns,OLD.lease_id AS lease_id,OLD.lease_until_ns AS lease_until_ns,OLD.invocation_json AS invocation_json) i WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'invocation',i.invocation_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('state',i.state,'createdAt',i.created_at,'terminalNS',i.terminal_ns,'leaseID',i.lease_id,'leaseUntilNS',i.lease_until_ns,'attemptCount',(SELECT COUNT(*) FROM invocation_attempts x WHERE x.invocation_id=i.invocation_id),'allAttemptsFinal',NOT EXISTS(SELECT 1 FROM invocation_attempts x WHERE x.invocation_id=i.invocation_id AND COALESCE(json_extract(CASE WHEN json_valid(CAST(x.attempt_json AS TEXT)) THEN CAST(x.attempt_json AS TEXT) ELSE '{}' END,'$.state'),'unknown') NOT IN('succeeded','failed','reconciled')))),1,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT OLD.invocation_id AS invocation_id,OLD.root_operation_id AS root_operation_id,OLD.owner_service AS owner_service,OLD.site_id AS site_id,OLD.operation_json AS operation_json,OLD.cost_class AS cost_class,OLD.execution_hash AS execution_hash,OLD.state AS state,OLD.created_at AS created_at,OLD.terminal_ns AS terminal_ns,OLD.lease_id AS lease_id,OLD.lease_until_ns AS lease_until_ns,OLD.invocation_json AS invocation_json) i JOIN usage_observation_heads h ON h.kind='invocation' AND h.entity_id=i.invocation_id WHERE 1;
END;
CREATE TRIGGER usage_observation_attempt_insert AFTER INSERT ON invocation_attempts BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'attempt',a.attempt_id,1 FROM (SELECT NEW.attempt_id AS attempt_id,NEW.invocation_id AS invocation_id,NEW.attempt_json AS attempt_json,NEW.provider_json AS provider_json,NEW.model_json AS model_json,NEW.price_snapshot_json AS price_snapshot_json,NEW.dispatched_ns AS dispatched_ns) a WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'attempt',a.attempt_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('attemptId',a.attempt_id,'state',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.state'),'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(a.price_snapshot_json AS TEXT)) THEN CAST(a.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'dispatchedNS',a.dispatched_ns)),0,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT NEW.attempt_id AS attempt_id,NEW.invocation_id AS invocation_id,NEW.attempt_json AS attempt_json,NEW.provider_json AS provider_json,NEW.model_json AS model_json,NEW.price_snapshot_json AS price_snapshot_json,NEW.dispatched_ns AS dispatched_ns) a LEFT JOIN invocations i ON i.invocation_id=a.invocation_id JOIN usage_observation_heads h ON h.kind='attempt' AND h.entity_id=a.attempt_id WHERE 1;
END;
CREATE TRIGGER usage_observation_attempt_update AFTER UPDATE ON invocation_attempts BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'attempt',a.attempt_id,1 FROM (SELECT NEW.attempt_id AS attempt_id,NEW.invocation_id AS invocation_id,NEW.attempt_json AS attempt_json,NEW.provider_json AS provider_json,NEW.model_json AS model_json,NEW.price_snapshot_json AS price_snapshot_json,NEW.dispatched_ns AS dispatched_ns) a WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'attempt',a.attempt_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('attemptId',a.attempt_id,'state',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.state'),'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(a.price_snapshot_json AS TEXT)) THEN CAST(a.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'dispatchedNS',a.dispatched_ns)),0,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT NEW.attempt_id AS attempt_id,NEW.invocation_id AS invocation_id,NEW.attempt_json AS attempt_json,NEW.provider_json AS provider_json,NEW.model_json AS model_json,NEW.price_snapshot_json AS price_snapshot_json,NEW.dispatched_ns AS dispatched_ns) a LEFT JOIN invocations i ON i.invocation_id=a.invocation_id JOIN usage_observation_heads h ON h.kind='attempt' AND h.entity_id=a.attempt_id WHERE 1;
END;
CREATE TRIGGER usage_observation_attempt_delete AFTER DELETE ON invocation_attempts BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'attempt',a.attempt_id,1 FROM (SELECT OLD.attempt_id AS attempt_id,OLD.invocation_id AS invocation_id,OLD.attempt_json AS attempt_json,OLD.provider_json AS provider_json,OLD.model_json AS model_json,OLD.price_snapshot_json AS price_snapshot_json,OLD.dispatched_ns AS dispatched_ns) a WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'attempt',a.attempt_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('attemptId',a.attempt_id,'state',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.state'),'providerId',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.providerId'),'providerVersion',json_extract(CASE WHEN json_valid(CAST(a.provider_json AS TEXT)) THEN CAST(a.provider_json AS TEXT) ELSE '{}' END,'$.version'),'modelId',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.modelId'),'modelVersion',json_extract(CASE WHEN json_valid(CAST(a.model_json AS TEXT)) THEN CAST(a.model_json AS TEXT) ELSE '{}' END,'$.version'),'modelSnapshotHash',json_extract(CASE WHEN json_valid(CAST(a.attempt_json AS TEXT)) THEN CAST(a.attempt_json AS TEXT) ELSE '{}' END,'$.modelSnapshotHash'),'priceSnapshotId',json_extract(CASE WHEN json_valid(CAST(a.price_snapshot_json AS TEXT)) THEN CAST(a.price_snapshot_json AS TEXT) ELSE '{}' END,'$.priceSnapshotId'),'dispatchedNS',a.dispatched_ns)),1,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT OLD.attempt_id AS attempt_id,OLD.invocation_id AS invocation_id,OLD.attempt_json AS attempt_json,OLD.provider_json AS provider_json,OLD.model_json AS model_json,OLD.price_snapshot_json AS price_snapshot_json,OLD.dispatched_ns AS dispatched_ns) a LEFT JOIN invocations i ON i.invocation_id=a.invocation_id JOIN usage_observation_heads h ON h.kind='attempt' AND h.entity_id=a.attempt_id WHERE 1;
END;
CREATE TRIGGER usage_observation_reservation_insert AFTER INSERT ON reservations BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'reservation',r.reservation_id,1 FROM (SELECT NEW.reservation_id AS reservation_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.state AS state,NEW.version AS version,NEW.pricing_known AS pricing_known,NEW.maximum_micro_usd AS maximum_micro_usd,NEW.held_micro_usd AS held_micro_usd,NEW.unknown_usage_count AS unknown_usage_count) r WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'reservation',r.reservation_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('reservationId',r.reservation_id,'attemptId',r.attempt_id,'state',r.state,'version',r.version,'pricingKnown',r.pricing_known,'maximumMicros',r.maximum_micro_usd,'heldMicros',r.held_micro_usd,'unknownUsageCount',r.unknown_usage_count)),0,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT NEW.reservation_id AS reservation_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.state AS state,NEW.version AS version,NEW.pricing_known AS pricing_known,NEW.maximum_micro_usd AS maximum_micro_usd,NEW.held_micro_usd AS held_micro_usd,NEW.unknown_usage_count AS unknown_usage_count) r LEFT JOIN invocations i ON i.invocation_id=r.invocation_id JOIN usage_observation_heads h ON h.kind='reservation' AND h.entity_id=r.reservation_id WHERE 1;
END;
CREATE TRIGGER usage_observation_reservation_update AFTER UPDATE ON reservations BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'reservation',r.reservation_id,1 FROM (SELECT NEW.reservation_id AS reservation_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.state AS state,NEW.version AS version,NEW.pricing_known AS pricing_known,NEW.maximum_micro_usd AS maximum_micro_usd,NEW.held_micro_usd AS held_micro_usd,NEW.unknown_usage_count AS unknown_usage_count) r WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'reservation',r.reservation_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('reservationId',r.reservation_id,'attemptId',r.attempt_id,'state',r.state,'version',r.version,'pricingKnown',r.pricing_known,'maximumMicros',r.maximum_micro_usd,'heldMicros',r.held_micro_usd,'unknownUsageCount',r.unknown_usage_count)),0,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT NEW.reservation_id AS reservation_id,NEW.invocation_id AS invocation_id,NEW.attempt_id AS attempt_id,NEW.state AS state,NEW.version AS version,NEW.pricing_known AS pricing_known,NEW.maximum_micro_usd AS maximum_micro_usd,NEW.held_micro_usd AS held_micro_usd,NEW.unknown_usage_count AS unknown_usage_count) r LEFT JOIN invocations i ON i.invocation_id=r.invocation_id JOIN usage_observation_heads h ON h.kind='reservation' AND h.entity_id=r.reservation_id WHERE 1;
END;
CREATE TRIGGER usage_observation_reservation_delete AFTER DELETE ON reservations BEGIN
INSERT INTO usage_observation_heads(kind,entity_id,revision) SELECT 'reservation',r.reservation_id,1 FROM (SELECT OLD.reservation_id AS reservation_id,OLD.invocation_id AS invocation_id,OLD.attempt_id AS attempt_id,OLD.state AS state,OLD.version AS version,OLD.pricing_known AS pricing_known,OLD.maximum_micro_usd AS maximum_micro_usd,OLD.held_micro_usd AS held_micro_usd,OLD.unknown_usage_count AS unknown_usage_count) r WHERE 1 ON CONFLICT(kind,entity_id) DO UPDATE SET revision=CASE WHEN revision<9223372036854775807 THEN revision+1 ELSE RAISE(ABORT,'observation_revision_exhausted') END;
INSERT INTO usage_observation_changes(kind,entity_id,entity_revision,site_id,source_epoch,snapshot_json,deleted,observed_at) SELECT 'reservation',r.reservation_id,h.revision,i.site_id,json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),json_object('origin',json_object('invocationId',i.invocation_id,'operationId',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.operationId'),'rootOperationId',i.root_operation_id,'ownerService',i.owner_service,'siteId',i.site_id,'sourceDataEpoch',json_extract(CASE WHEN json_valid(CAST(i.operation_json AS TEXT)) THEN CAST(i.operation_json AS TEXT) ELSE '{}' END,'$.dataEpoch'),'costClass',i.cost_class,'executionHash',i.execution_hash),'fact',json_object('reservationId',r.reservation_id,'attemptId',r.attempt_id,'state',r.state,'version',r.version,'pricingKnown',r.pricing_known,'maximumMicros',r.maximum_micro_usd,'heldMicros',r.held_micro_usd,'unknownUsageCount',r.unknown_usage_count)),1,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM (SELECT OLD.reservation_id AS reservation_id,OLD.invocation_id AS invocation_id,OLD.attempt_id AS attempt_id,OLD.state AS state,OLD.version AS version,OLD.pricing_known AS pricing_known,OLD.maximum_micro_usd AS maximum_micro_usd,OLD.held_micro_usd AS held_micro_usd,OLD.unknown_usage_count AS unknown_usage_count) r LEFT JOIN invocations i ON i.invocation_id=r.invocation_id JOIN usage_observation_heads h ON h.kind='reservation' AND h.entity_id=r.reservation_id WHERE 1;
END;
CREATE TRIGGER usage_retain_invocation BEFORE DELETE ON invocations BEGIN SELECT RAISE(ABORT,'invocation_financial_authority_retained'); END;
CREATE TRIGGER usage_retain_attempt BEFORE DELETE ON invocation_attempts BEGIN SELECT RAISE(ABORT,'attempt_financial_authority_retained'); END;
CREATE TRIGGER usage_retain_held_reservation BEFORE DELETE ON reservations WHEN OLD.state='held' BEGIN SELECT RAISE(ABORT,'held_reservation_authority_retained'); END;
CREATE TRIGGER usage_observation_immutable_update BEFORE UPDATE ON usage_observation_changes BEGIN SELECT RAISE(ABORT,'observation_history_immutable'); END;
CREATE TRIGGER usage_observation_immutable_delete BEFORE DELETE ON usage_observation_changes BEGIN SELECT RAISE(ABORT,'observation_history_immutable'); END;
CREATE TRIGGER usage_timeline_immutable_update BEFORE UPDATE ON usage_statistics_timeline BEGIN SELECT RAISE(ABORT,'statistics_timeline_immutable'); END;
CREATE TRIGGER usage_timeline_immutable_delete BEFORE DELETE ON usage_statistics_timeline BEGIN SELECT RAISE(ABORT,'statistics_timeline_immutable'); END;
CREATE TRIGGER usage_stable_invocations BEFORE UPDATE ON invocations WHEN OLD.invocation_id<>NEW.invocation_id BEGIN SELECT RAISE(ABORT,'financial_identity_immutable'); END;
CREATE TRIGGER usage_stable_invocation_attempts BEFORE UPDATE ON invocation_attempts WHEN OLD.attempt_id<>NEW.attempt_id BEGIN SELECT RAISE(ABORT,'financial_identity_immutable'); END;
CREATE TRIGGER usage_stable_reservations BEFORE UPDATE ON reservations WHEN OLD.reservation_id<>NEW.reservation_id BEGIN SELECT RAISE(ABORT,'financial_identity_immutable'); END;
INSERT INTO "ledger_read_state"("singleton","revision") VALUES(1,1);
INSERT INTO "credential_tracking"("singleton","enabled") VALUES(1,0);
INSERT INTO "gateway_snapshot_state"("singleton","revision") VALUES(1,1);
