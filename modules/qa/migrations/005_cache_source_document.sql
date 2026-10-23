-- 缓存答案是访客在哪篇文档上问出来的（没有当前页时为空）：Jev 判断复用时一并参考，
-- 避免“这一页讲了什么”这类在不同页面意思不同的问题串用答案。
ALTER TABLE answer_cache ADD COLUMN source_document TEXT NOT NULL DEFAULT '';
