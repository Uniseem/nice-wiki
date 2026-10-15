-- 事件信封保持不变；投递元数据与回执只追加。旧事件仍以envelope_json为权威。
ALTER TABLE events ADD COLUMN producer TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN event_type TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN event_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE events ADD COLUMN subject_id TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN envelope_hash TEXT NOT NULL DEFAULT '';
UPDATE events SET producer=json_extract(envelope_json,'$.producer'),
    event_type=json_extract(envelope_json,'$.eventType'),
    event_version=json_extract(envelope_json,'$.eventVersion'),
    subject_id=json_extract(envelope_json,'$.subjectId');
CREATE INDEX idx_events_epoch_subject ON events(data_epoch,producer,subject_id,event_seq);

-- 全局序号独立于可清理历史，不能在事件清空后归零或复用。
CREATE TABLE event_sequence (
    singleton INTEGER PRIMARY KEY CHECK (singleton=1),
    last_seq INTEGER NOT NULL CHECK (last_seq>=0)
);
INSERT INTO event_sequence(singleton,last_seq) SELECT 1,COALESCE(MAX(event_seq),0) FROM events;

ALTER TABLE deliveries ADD COLUMN delivery_id TEXT NOT NULL DEFAULT '';
UPDATE deliveries SET delivery_id=lower(hex(randomblob(16)));
CREATE UNIQUE INDEX idx_deliveries_id ON deliveries(delivery_id);
ALTER TABLE deliveries ADD COLUMN lease_expires_at TEXT;
ALTER TABLE deliveries ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK (revision>=1);
ALTER TABLE deliveries ADD COLUMN last_error_code TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_deliveries_available ON deliveries(consumer_id,state,retry_at,event_id);

CREATE TABLE consumer_offsets (
    consumer_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    acknowledged_seq INTEGER NOT NULL CHECK (acknowledged_seq>=0),
    PRIMARY KEY(consumer_id,data_epoch)
);
CREATE TABLE event_operations (
    caller TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('publish','ack','replay')),
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(caller,data_epoch,operation_kind,operation_id)
);
