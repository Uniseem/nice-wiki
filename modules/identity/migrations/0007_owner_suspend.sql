-- 停用用户时连带下架的站点打上标记，恢复用户时只放回这些站点（举报下架的不放回）。
ALTER TABLE sites ADD COLUMN suspended_with_owner INTEGER NOT NULL DEFAULT 0;
