-- 首页等文档 ID 在各站点相同：页面计数和爬虫计数的主键并入 site_id，否则同一小时两个站点的首页计数互相冲突。
ALTER TABLE hourly_page_counts DROP CONSTRAINT hourly_page_counts_pkey;
ALTER TABLE hourly_page_counts ADD PRIMARY KEY (site_id, hour, document_id, locale);
ALTER TABLE daily_page_counts DROP CONSTRAINT daily_page_counts_pkey;
ALTER TABLE daily_page_counts ADD PRIMARY KEY (site_id, day, timezone, document_id, locale);
ALTER TABLE daily_page_dimensions DROP CONSTRAINT daily_page_dimensions_pkey;
ALTER TABLE daily_page_dimensions ADD PRIMARY KEY (site_id, day, timezone, document_id, locale, source_domain, country_code);
ALTER TABLE crawler_counts DROP CONSTRAINT crawler_counts_pkey;
ALTER TABLE crawler_counts ADD PRIMARY KEY (site_id, day, document_id, locale, crawler_family);
