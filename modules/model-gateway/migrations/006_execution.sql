-- schemaVersion 6：真实调用和独立attempt的费用/许可/凭据引用；旧行不猜填。
ALTER TABLE invocations ADD COLUMN invocation_json BLOB;
ALTER TABLE invocations ADD COLUMN invocation_hash TEXT;
ALTER TABLE invocations ADD COLUMN request_json BLOB;
ALTER TABLE invocations ADD COLUMN operation_json BLOB;
ALTER TABLE invocations ADD COLUMN route_json BLOB;
ALTER TABLE invocations ADD COLUMN profile_json BLOB;
ALTER TABLE invocations ADD COLUMN fee_binding_json BLOB;
ALTER TABLE invocations ADD COLUMN active_attempt_id TEXT;
ALTER TABLE invocations ADD COLUMN lease_id TEXT NOT NULL DEFAULT '';
ALTER TABLE invocations ADD COLUMN lease_until_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE invocations ADD COLUMN cancel_requested INTEGER NOT NULL DEFAULT 0;
ALTER TABLE invocations ADD COLUMN created_ns INTEGER;
ALTER TABLE invocations ADD COLUMN terminal_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE invocations ADD COLUMN terminal_output_json BLOB;
ALTER TABLE reservations ADD COLUMN attempt_id TEXT;
CREATE UNIQUE INDEX reservation_attempt ON reservations(attempt_id);
CREATE TABLE invocation_attempts(
 attempt_id TEXT PRIMARY KEY,invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),ordinal INTEGER NOT NULL,
 attempt_json BLOB NOT NULL,model_json BLOB NOT NULL,provider_json BLOB NOT NULL,credential_version INTEGER NOT NULL,
 price_snapshot_json BLOB NOT NULL,lease_id TEXT NOT NULL DEFAULT '',lease_until_ns INTEGER NOT NULL DEFAULT 0,
 dispatched_ns INTEGER NOT NULL DEFAULT 0,usage_hash TEXT,input_token_budget INTEGER NOT NULL,output_token_budget INTEGER NOT NULL,UNIQUE(invocation_id,ordinal)
);
CREATE TABLE invocation_usage(
 provider_id TEXT NOT NULL,provider_request_id TEXT NOT NULL,invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
 attempt_id TEXT NOT NULL REFERENCES invocation_attempts(attempt_id),usage_hash TEXT NOT NULL,usage_json BLOB NOT NULL,
 conflict_hash TEXT,created_ns INTEGER NOT NULL,PRIMARY KEY(provider_id,provider_request_id)
);
CREATE TABLE fee_commitments(
 reservation_id TEXT PRIMARY KEY REFERENCES reservations(reservation_id),attempt_id TEXT NOT NULL UNIQUE,
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,cost_class TEXT NOT NULL,profile_id TEXT NOT NULL,root_operation_id TEXT NOT NULL,
 session_scope TEXT NOT NULL,network_scope TEXT NOT NULL,created_ns INTEGER NOT NULL,state TEXT NOT NULL,
 amount_micro_usd INTEGER,pricing_known INTEGER NOT NULL
);
CREATE INDEX fee_commitment_scope ON fee_commitments(site_id,data_epoch,cost_class,state,created_ns);
CREATE TABLE execution_receipts(
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,owner_service TEXT NOT NULL,function_id TEXT NOT NULL,idempotency_key TEXT NOT NULL,
 input_hash TEXT NOT NULL,result_json BLOB NOT NULL,result_hash TEXT NOT NULL,created_ns INTEGER NOT NULL,
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
ALTER TABLE ledger_entries ADD COLUMN supersedes_entry_id TEXT;
CREATE UNIQUE INDEX ledger_provider_attempt ON ledger_entries(invocation_id,attempt_id,status,usage_hash);
ALTER TABLE ledger_entries ADD COLUMN price_snapshot_json BLOB;
CREATE TABLE reconciliation_proofs(attempt_id TEXT NOT NULL REFERENCES invocation_attempts(attempt_id),proof_hash TEXT NOT NULL,actor_id TEXT NOT NULL,proof_json BLOB NOT NULL,created_ns INTEGER NOT NULL,PRIMARY KEY(attempt_id,proof_hash));
-- 不收费证明只追加审计，不伪造零token/费用账单；聚合投影排除被明确否定的原条目。
CREATE TABLE ledger_voids(entry_id TEXT PRIMARY KEY REFERENCES ledger_entries(entry_id),proof_hash TEXT NOT NULL,actor_id TEXT NOT NULL,created_ns INTEGER NOT NULL);
