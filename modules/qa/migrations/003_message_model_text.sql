-- 访客问题当时实际发给回答模型的内容（当轮当前页、检索资料和问题）。后续轮次按原样重放，
-- 让每轮请求都是上一轮请求的延长，命中供应方的前缀缓存；对访客显示的仍是 text。
ALTER TABLE messages ADD COLUMN model_text TEXT NOT NULL DEFAULT '';
