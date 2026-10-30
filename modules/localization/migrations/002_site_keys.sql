-- 语言建议缓存键只由访客信号和启用语言算出，各站点会重复：主键并入 site_id。
ALTER TABLE language_suggestions DROP CONSTRAINT language_suggestions_pkey;
ALTER TABLE language_suggestions ADD PRIMARY KEY (site_id, cache_key);
