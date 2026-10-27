-- 已登录用户绑定第三方账号：授权状态记下是谁发起的绑定，登录流程不接受绑定用的状态。
ALTER TABLE oauth_states ADD COLUMN link_user_id TEXT NOT NULL DEFAULT '';
