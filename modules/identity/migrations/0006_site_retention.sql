-- 删站保留 30 天：记下删除时间和删除前的状态，期内恢复回原状态；到期清除后站点名放出。
ALTER TABLE sites ADD COLUMN deleted_at TEXT;
ALTER TABLE sites ADD COLUMN restore_status TEXT;
-- 之前就删掉的站点按最后修改时间算删除时间。
UPDATE sites SET deleted_at = updated_at, restore_status = 'active' WHERE status = 'deleted';
