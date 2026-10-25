-- 问答与索引不再使用专用决策模型；语言建议沿用已有快速档，清除旧决策模型覆盖。
DELETE FROM task_assignments WHERE task_id IN ('qa.decide', 'qa.english', 'knowledge.decide');
UPDATE task_assignments SET tier = 'fast', override_model_id = NULL, version = version + 1
WHERE task_id = 'localization.suggest' AND tier = 'decision';
-- 保留历史模型身份及费用引用，只停用已有的Jev配置。
UPDATE models SET enabled = 0, version = version + 1
WHERE provider_model_version LIKE 'typesafe/jev-%' AND enabled = 1;
