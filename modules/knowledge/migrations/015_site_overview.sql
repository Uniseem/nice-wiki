-- 层级摘要：每篇已发布文档一段摘要，有下级文档的同时概括其下级；按输入哈希复用，输入没变不重新调模型。
CREATE TABLE tree_summaries (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 document_id TEXT NOT NULL,
 input_hash TEXT NOT NULL,
 summary TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY (site_id, data_epoch, document_id)
);
-- 全站概括：由顶层文档的层级摘要合成，每天重算一次。
CREATE TABLE site_overview (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 input_hash TEXT NOT NULL,
 overview TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 checked_at TEXT NOT NULL,
 PRIMARY KEY (site_id, data_epoch)
);
