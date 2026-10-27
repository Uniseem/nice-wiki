-- 站点名可以改，30 天内只能改一次；旧子域名保留 30 天（期间 301 到新地址），到期删掉、站点名放出。
ALTER TABLE sites ADD COLUMN slug_changed_at TEXT;
ALTER TABLE site_domains ADD COLUMN expires_at TEXT;
