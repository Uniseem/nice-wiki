-- schemaVersion 4：IP禁令与可信入口的会话地址；旧哈希禁令保留至到期。
ALTER TABLE guard_subjects ADD COLUMN ip TEXT NOT NULL DEFAULT '';
ALTER TABLE guard_permits ADD COLUMN ip TEXT NOT NULL DEFAULT '';
ALTER TABLE guard_question_signals ADD COLUMN ip TEXT NOT NULL DEFAULT '';
ALTER TABLE guard_bans ADD COLUMN ip TEXT NOT NULL DEFAULT '';
ALTER TABLE guard_risk_events ADD COLUMN ip TEXT NOT NULL DEFAULT '';
CREATE INDEX guard_permit_ip ON guard_permits(site_id,data_epoch,ip,state);
CREATE INDEX guard_ip_bans ON guard_bans(site_id,data_epoch,ip,state,expires_at_ns);
CREATE TRIGGER guard_risk_ip AFTER INSERT ON guard_risk_events WHEN NEW.ip='' BEGIN UPDATE guard_risk_events SET ip=COALESCE((SELECT ip FROM guard_subjects WHERE site_id=NEW.site_id AND data_epoch=NEW.data_epoch AND network_key=NEW.network_key ORDER BY updated_at_ns DESC LIMIT 1),'') WHERE event_id=NEW.event_id; END;
PRAGMA user_version=4;
