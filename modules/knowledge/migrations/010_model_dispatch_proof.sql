-- 旧记录保守视为已经尝试RPC；新记录只有显式prepared才可证明尚未派发。
ALTER TABLE model_steps ADD COLUMN dispatch_started INTEGER NOT NULL DEFAULT 1 CHECK(dispatch_started IN (0,1));
