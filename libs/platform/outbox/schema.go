package outbox

// SQL触发器覆盖所有行变更，包括未来重试/leased/quarantine；不依赖轮询due子集。
const observationSchemaSQL = `CREATE TABLE IF NOT EXISTS nw_outbox_control(id INTEGER PRIMARY KEY CHECK(id=1), revision INTEGER NOT NULL CHECK(revision>0), sealed_hash TEXT NOT NULL DEFAULT '', last_seal_hash TEXT NOT NULL DEFAULT '');
INSERT OR IGNORE INTO nw_outbox_control(id,revision,sealed_hash) VALUES(1,1,'');
CREATE TRIGGER IF NOT EXISTS nw_outbox_gate_insert BEFORE INSERT ON nw_outbox WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_gate_update BEFORE UPDATE ON nw_outbox WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_gate_delete BEFORE DELETE ON nw_outbox WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_revision_insert AFTER INSERT ON nw_outbox BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_revision_update AFTER UPDATE ON nw_outbox BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_revision_delete AFTER DELETE ON nw_outbox BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;`
