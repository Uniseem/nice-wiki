-- 海外网盘授权：state 还要记住是哪个站点、哪个备份目标、哪位用户发起的。
ALTER TABLE oauth_states ADD COLUMN context TEXT NOT NULL DEFAULT '';
