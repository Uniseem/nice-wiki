-- 全站概括的英文版：决策模型（Jev）只读英文，问答判断问题是否属于本站、能否复用缓存时参考。
ALTER TABLE site_overview ADD COLUMN overview_en TEXT NOT NULL DEFAULT '';
