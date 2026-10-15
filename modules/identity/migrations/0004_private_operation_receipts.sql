-- 私有口令操作摘要固定绑定签名密钥版本，轮换后不得当新操作执行。
ALTER TABLE operation_receipts ADD COLUMN digest_key_id TEXT NOT NULL DEFAULT '';
