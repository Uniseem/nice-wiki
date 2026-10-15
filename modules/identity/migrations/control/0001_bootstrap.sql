-- control区不随业务恢复替换；只允许bootstrap状态单向收敛。
CREATE TABLE control_schema (singleton INTEGER PRIMARY KEY CHECK(singleton=1), schema_version INTEGER NOT NULL);
INSERT INTO control_schema VALUES(1,1);
CREATE TABLE bootstrap_claim (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 schema_version INTEGER NOT NULL CHECK(schema_version=1),
 operation_id TEXT NOT NULL, caller_service TEXT NOT NULL,
 actor_id TEXT NOT NULL, actor_site_id TEXT NOT NULL,
 request_digest TEXT NOT NULL, digest_key_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL, account_id TEXT NOT NULL, site_id TEXT NOT NULL,
 claimed_at TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('reserved','committed'))
);
