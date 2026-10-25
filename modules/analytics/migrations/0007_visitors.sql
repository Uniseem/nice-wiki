-- schemaVersion 7：访客以站内随机ID统计；活动明细保留30天。
CREATE TABLE visitor_activities(activity_id TEXT PRIMARY KEY,visitor_id TEXT NOT NULL,document_id TEXT NOT NULL,locale TEXT NOT NULL,event_type TEXT NOT NULL,ip TEXT NOT NULL,user_agent TEXT NOT NULL,country TEXT NOT NULL,referrer_domain TEXT NOT NULL,occurred_at TEXT NOT NULL,data_epoch TEXT NOT NULL);
CREATE INDEX visitor_activity_latest ON visitor_activities(data_epoch,occurred_at DESC,activity_id DESC);
CREATE INDEX visitor_activity_history ON visitor_activities(data_epoch,visitor_id,occurred_at DESC,activity_id DESC);
UPDATE analytics_meta SET schema_version=7 WHERE singleton=1;
PRAGMA user_version=7;
