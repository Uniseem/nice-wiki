-- model-gateway 本域 active 数据库初始迁移。
-- secret 明文、提示正文和访客 IP 不进入本域数据库。
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runtime_meta (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS providers (
    provider_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    adapter TEXT NOT NULL,
    base_url TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    credential_version INTEGER NOT NULL CHECK (credential_version >= 0),
    connect_timeout_ms INTEGER NOT NULL CHECK (connect_timeout_ms >= 0),
    request_timeout_ms INTEGER NOT NULL CHECK (request_timeout_ms >= 0),
    display_name TEXT NOT NULL,
    extra_header_names_json BLOB NOT NULL,
    extra_header_secret_refs_json BLOB NOT NULL,
    concurrency_limit INTEGER NOT NULL CHECK (concurrency_limit >= 0),
    stream_idle_timeout_ms INTEGER NOT NULL CHECK (stream_idle_timeout_ms >= 0),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS models (
    model_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    provider_id TEXT NOT NULL REFERENCES providers(provider_id),
    provider_model_version TEXT NOT NULL,
    capabilities_json BLOB NOT NULL,
    dimension INTEGER NOT NULL CHECK (dimension >= 0),
    preprocessor_version TEXT NOT NULL,
    max_input_tokens INTEGER NOT NULL CHECK (max_input_tokens >= 0),
    max_output_tokens INTEGER NOT NULL CHECK (max_output_tokens >= 0),
    catalog_origin TEXT NOT NULL,
    manual_override INTEGER NOT NULL CHECK (manual_override IN (0, 1)),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS operation_receipts (
    site_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    function_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (site_id, actor_id, function_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS invocations (
    invocation_id TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    root_operation_id TEXT NOT NULL,
    cost_class TEXT NOT NULL,
    state TEXT NOT NULL,
    deadline TEXT NOT NULL,
    price_snapshot_json BLOB NOT NULL,
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS reservations (
    reservation_id TEXT PRIMARY KEY,
    invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
    maximum_micro_usd INTEGER,
    held_micro_usd INTEGER,
    state TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 0),
    pricing_known INTEGER NOT NULL CHECK (pricing_known IN (0, 1)),
    unknown_usage_count INTEGER NOT NULL CHECK (unknown_usage_count >= 0)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
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
);

CREATE TABLE IF NOT EXISTS stream_events (
    invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    event TEXT NOT NULL,
    payload_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (invocation_id, sequence)
);

CREATE TABLE IF NOT EXISTS barriers (
    barrier_id TEXT PRIMARY KEY,
    state TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    deadline TEXT NOT NULL
);
