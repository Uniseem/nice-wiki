-- 纯Agent问答不再使用前置改写、快速旁路和共享答案缓存。历史调用保留。
DELETE FROM task_assignments WHERE task_id IN ('qa.preflight','qa.answer-fast','qa.cache-match','qa.cache-translate');
