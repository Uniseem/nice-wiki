-- 缓存条目的英文问题（前置改写给出）：决策模型（Jev）只读英文，判断新提问能否直接复用某条缓存答案时用。
ALTER TABLE answer_cache ADD COLUMN english_question TEXT NOT NULL DEFAULT '';
