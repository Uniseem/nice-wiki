-- V1 usage_projection/daily_usage保持原样。OpenStore在迁移事务内拒绝非空旧费用数据，
-- 不补原owner/site/source epoch，不以旧金额猜测完整V2来源。
CREATE TABLE usage_facts_v2(
 data_epoch TEXT NOT NULL,
 change_revision INTEGER NOT NULL CHECK(change_revision>0),
 fact_kind TEXT NOT NULL CHECK(fact_kind IN('ledger','invocation','attempt','reservation')),
 entity_id TEXT NOT NULL,
 entity_revision INTEGER NOT NULL CHECK(entity_revision>0),
 site_id TEXT NOT NULL,
 source_data_epoch TEXT NOT NULL CHECK(source_data_epoch<>''),
 payload_hash TEXT NOT NULL,
 fact_json TEXT NOT NULL,
 PRIMARY KEY(data_epoch,change_revision),
 UNIQUE(data_epoch,fact_kind,entity_id,entity_revision)
);

CREATE TABLE usage_projection_v2(
 data_epoch TEXT NOT NULL,
 ledger_entry_id TEXT NOT NULL,
 ledger_revision INTEGER NOT NULL CHECK(ledger_revision>0),
 change_revision INTEGER NOT NULL CHECK(change_revision>0),
 invocation_id TEXT NOT NULL,
 attempt_id TEXT NOT NULL,
 owner_service TEXT NOT NULL,
 site_id TEXT NOT NULL,
 source_data_epoch TEXT NOT NULL CHECK(source_data_epoch<>''),
 operation_id TEXT NOT NULL,
 root_operation_id TEXT NOT NULL,
 cost_class TEXT NOT NULL CHECK(cost_class IN('qa','background','admin-probe')),
 provider_id TEXT NOT NULL,
 provider_version INTEGER NOT NULL CHECK(provider_version>0),
 model_id TEXT NOT NULL,
 model_version INTEGER NOT NULL CHECK(model_version>0),
 model_snapshot_hash TEXT NOT NULL,
 amount_known INTEGER NOT NULL CHECK(amount_known IN(0,1)),
 amount_micros INTEGER,
 currency TEXT,
 state TEXT NOT NULL CHECK(state IN('settled','reconciled','price_unknown','superseded','voided','deleted')),
 ledger_status TEXT NOT NULL CHECK(ledger_status IN('settled','reconciled','price_unknown')),
 occurred_at TEXT NOT NULL,
 observed_at TEXT NOT NULL,
 capture_kind TEXT NOT NULL CHECK(capture_kind IN('baseline','change')),
 payload_hash TEXT NOT NULL,
 fact_json TEXT NOT NULL,
 PRIMARY KEY(data_epoch,ledger_entry_id),
 FOREIGN KEY(data_epoch,change_revision) REFERENCES usage_facts_v2(data_epoch,change_revision),
 CHECK((amount_known=0 AND amount_micros IS NULL AND currency IS NULL) OR
       (amount_known=1 AND amount_micros IS NOT NULL AND amount_micros>=0 AND amount_micros<=9007199254740991 AND currency IS NOT NULL AND currency='USD')),
 CHECK(ledger_status<>'price_unknown' OR amount_known=0)
);
CREATE INDEX usage_projection_v2_time ON usage_projection_v2(data_epoch,occurred_at,ledger_entry_id);

-- 完整嵌套事实保存在fact_json：invocation的执行/终态、attempt冻结路线/价格引用、
-- reservation held/maximum/unknownUsageCount及deleted墓碑均不损失。
CREATE TABLE usage_observations_v2(
 data_epoch TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN('invocation','attempt','reservation')),
 entity_id TEXT NOT NULL,
 entity_revision INTEGER NOT NULL CHECK(entity_revision>0),
 change_revision INTEGER NOT NULL CHECK(change_revision>0),
 invocation_id TEXT NOT NULL,
 owner_service TEXT NOT NULL,
 site_id TEXT NOT NULL,
 source_data_epoch TEXT NOT NULL CHECK(source_data_epoch<>''),
 operation_id TEXT NOT NULL,
 root_operation_id TEXT NOT NULL,
 cost_class TEXT NOT NULL CHECK(cost_class IN('qa','background','admin-probe')),
 deleted INTEGER NOT NULL CHECK(deleted IN(0,1)),
 observed_at TEXT NOT NULL,
 payload_hash TEXT NOT NULL,
 fact_json TEXT NOT NULL,
 PRIMARY KEY(data_epoch,kind,entity_id),
 FOREIGN KEY(data_epoch,change_revision) REFERENCES usage_facts_v2(data_epoch,change_revision)
);
CREATE INDEX usage_observations_v2_invocation ON usage_observations_v2(data_epoch,invocation_id,kind,entity_id);
