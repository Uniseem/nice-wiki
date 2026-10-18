-- 原创建键与完整tracking来源独立持久；旧行NULL不猜填，空库不自动形成证明。
ALTER TABLE invocations ADD COLUMN create_command_key TEXT;
CREATE UNIQUE INDEX invocation_create_command ON invocations(site_id,data_epoch,owner_service,create_command_key) WHERE create_command_key IS NOT NULL;
CREATE TABLE invocation_lookup_tracking(singleton INTEGER PRIMARY KEY CHECK(singleton=1),format_version INTEGER NOT NULL CHECK(format_version=1),data_epoch TEXT NOT NULL,proof_kind TEXT NOT NULL CHECK(proof_kind IN('fresh_activation','verified_restore')),proof_hash TEXT NOT NULL,created_at TEXT NOT NULL);
