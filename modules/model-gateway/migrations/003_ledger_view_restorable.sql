-- 原视图用 IS NOT 1 比较布尔和整数，换到 PostgreSQL 后靠 public 里的兼容运算符；pg_dump 写不出
-- IS DISTINCT FROM 用的非系统运算符，带这个视图的整站备份恢复不了。改成 IS NOT TRUE，语义不变。
DROP VIEW IF EXISTS ledger_invalid_timestamps;
CREATE VIEW ledger_invalid_timestamps AS SELECT entry_id FROM ledger_entries WHERE (length(created_at) BETWEEN 20 AND 30
 AND substr(created_at,1,19) GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]'
 AND substr(created_at,-1)='Z'
 AND strftime('%Y-%m-%dT%H:%M:%S',substr(created_at,1,19),'+0 seconds') IS substr(created_at,1,19)
 AND (length(created_at)=20 OR (length(created_at) BETWEEN 22 AND 30 AND substr(created_at,20,1)='.' AND substr(created_at,21,length(created_at)-21) NOT GLOB '*[^0-9]*'))) IS NOT TRUE;
