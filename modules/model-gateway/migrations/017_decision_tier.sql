-- 新增“决策”档位（只返回带概率选择结果的模型，如 TypeSafe Jev）：重建两张表放宽档位约束，数据原样搬过去。
CREATE TABLE task_assignments_rebuilt (
    task_id TEXT PRIMARY KEY,
    tier TEXT NOT NULL CHECK (tier IN ('fast', 'accurate', 'vision', 'embedding', 'rerank', 'decision')),
    override_model_id TEXT REFERENCES models(model_id),
    version INTEGER NOT NULL CHECK (version >= 1),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL
);
INSERT INTO task_assignments_rebuilt (task_id, tier, override_model_id, version, data_epoch, updated_at)
SELECT task_id, tier, override_model_id, version, data_epoch, updated_at FROM task_assignments;
DROP TABLE task_assignments;
ALTER TABLE task_assignments_rebuilt RENAME TO task_assignments;

CREATE TABLE profiles_rebuilt (
    profile_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    cost_class TEXT NOT NULL CHECK (cost_class IN ('qa', 'background', 'admin-probe')),
    tier TEXT NOT NULL CHECK (tier IN ('fast', 'accurate', 'vision', 'embedding', 'rerank', 'decision')),
    ordered_model_ids_json BLOB NOT NULL,
    maximum_cost_micro_usd INTEGER NOT NULL CHECK (maximum_cost_micro_usd >= 0),
    concurrency INTEGER NOT NULL CHECK (concurrency >= 0),
    daily_limit_cost_micro_usd INTEGER NOT NULL CHECK (daily_limit_cost_micro_usd >= 0),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL,
    UNIQUE (data_epoch, cost_class, tier)
);
INSERT INTO profiles_rebuilt (profile_id, version, cost_class, tier, ordered_model_ids_json, maximum_cost_micro_usd, concurrency, daily_limit_cost_micro_usd, data_epoch, updated_at)
SELECT profile_id, version, cost_class, tier, ordered_model_ids_json, maximum_cost_micro_usd, concurrency, daily_limit_cost_micro_usd, data_epoch, updated_at FROM profiles;
DROP TABLE profiles;
ALTER TABLE profiles_rebuilt RENAME TO profiles;

-- 语言切换建议改由决策档位承担：已有分配跟着目录改档位，原先指定的对话模型不再适用。
UPDATE task_assignments SET tier = 'decision', override_model_id = NULL, version = version + 1 WHERE task_id = 'localization.suggest';
