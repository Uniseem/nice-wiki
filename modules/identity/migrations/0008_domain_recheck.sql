-- 已验证的自定义域名每天复核一次：记下上次复核时间和连续失败次数，连续 3 次查不到验证记录就标成失效。
ALTER TABLE site_domains ADD COLUMN checked_at TEXT;
ALTER TABLE site_domains ADD COLUMN check_failures INTEGER NOT NULL DEFAULT 0;
