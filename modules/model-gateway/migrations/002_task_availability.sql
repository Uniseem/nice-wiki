-- 只追加配置持久结构，不插入默认任务、模型、凭据或费用记录。
CREATE TABLE task_assignments (
    task_id TEXT PRIMARY KEY,
    tier TEXT NOT NULL CHECK (tier IN ('fast', 'accurate', 'vision', 'embedding', 'rerank')),
    override_model_id TEXT REFERENCES models(model_id),
    version INTEGER NOT NULL CHECK (version >= 1),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL
);

CREATE TABLE profiles (
    profile_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    cost_class TEXT NOT NULL CHECK (cost_class IN ('qa', 'background', 'admin-probe')),
    tier TEXT NOT NULL CHECK (tier IN ('fast', 'accurate', 'vision', 'embedding', 'rerank')),
    ordered_model_ids_json BLOB NOT NULL,
    maximum_cost_micro_usd INTEGER NOT NULL CHECK (maximum_cost_micro_usd >= 0),
    concurrency INTEGER NOT NULL CHECK (concurrency >= 0),
    daily_limit_cost_micro_usd INTEGER NOT NULL CHECK (daily_limit_cost_micro_usd >= 0),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL,
    UNIQUE (data_epoch, cost_class, tier)
);

-- 密文由后续真正的PutCredential/RotateCredential按providerId/version作AAD写入。
-- 免费读取只能查询存在性、代次和长度，不选取nonce/ciphertext正文。
CREATE TABLE credentials (
    provider_id TEXT NOT NULL REFERENCES providers(provider_id),
    credential_version INTEGER NOT NULL CHECK (credential_version >= 1),
    nonce BLOB NOT NULL CHECK (typeof(nonce) = 'blob' AND length(nonce) = 12),
    ciphertext BLOB NOT NULL CHECK (typeof(ciphertext) = 'blob' AND length(ciphertext) > 16),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    PRIMARY KEY (provider_id, credential_version)
);
