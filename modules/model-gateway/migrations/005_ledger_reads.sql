-- 不为历史调用猜填归属或时间；缺前置的旧行由读取端失败关闭。
ALTER TABLE invocations ADD COLUMN owner_service TEXT;
ALTER TABLE invocations ADD COLUMN site_id TEXT;
ALTER TABLE invocations ADD COLUMN created_at TEXT;
CREATE TABLE ledger_read_state (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 revision INTEGER NOT NULL CHECK(typeof(revision)='integer' AND revision>=1)
);
INSERT INTO ledger_read_state(singleton,revision) VALUES(1,1);
-- UTC秒和补齐九位纳秒作为精确排序键，避免RFC3339可变小数位的字符串排序问题。
CREATE INDEX ledger_read_order ON ledger_entries(
 (substr(created_at,1,19)||'.'||substr((CASE WHEN substr(created_at,20,1)='.' THEN substr(created_at,21,length(created_at)-21) ELSE '' END)||'000000000',1,9)||'Z'),entry_id
);
-- 日期/整秒仅用SQLite内建+0 seconds规范化往返；小数独立保留，避免毫秒舍入跨秒。
-- 只索引时间元数据，任一非法值（包括NULL判定）不能被时间窗WHERE隐去。
CREATE VIEW ledger_invalid_timestamps AS SELECT entry_id FROM ledger_entries WHERE (length(created_at) BETWEEN 20 AND 30
 AND substr(created_at,1,19) GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]'
 AND substr(created_at,-1)='Z'
 AND strftime('%Y-%m-%dT%H:%M:%S',substr(created_at,1,19),'+0 seconds') IS substr(created_at,1,19)
 AND (length(created_at)=20 OR (length(created_at) BETWEEN 22 AND 30 AND substr(created_at,20,1)='.' AND substr(created_at,21,length(created_at)-21) NOT GLOB '*[^0-9]*'))) IS NOT 1;
CREATE INDEX ledger_invalid_time ON ledger_entries(entry_id) WHERE (length(created_at) BETWEEN 20 AND 30
 AND substr(created_at,1,19) GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]'
 AND substr(created_at,-1)='Z'
 AND strftime('%Y-%m-%dT%H:%M:%S',substr(created_at,1,19),'+0 seconds') IS substr(created_at,1,19)
 AND (length(created_at)=20 OR (length(created_at) BETWEEN 22 AND 30 AND substr(created_at,20,1)='.' AND substr(created_at,21,length(created_at)-21) NOT GLOB '*[^0-9]*'))) IS NOT 1;
CREATE INDEX invocation_root_read ON invocations(root_operation_id,invocation_id);
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
