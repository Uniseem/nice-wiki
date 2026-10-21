-- 文档卡片：每篇已发布文档一张概括（摘要、主题、能回答的问题），给问答路由和文档索引视图用。
-- 按正文哈希维护：正文没变不动；正文变了先问决策模型旧卡片是否仍准确，够把握就沿用（origin=kept）。
CREATE TABLE document_cards (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 document_id TEXT NOT NULL,
 source_hash TEXT NOT NULL,
 card_json TEXT NOT NULL,
 card_hash TEXT NOT NULL,
 origin TEXT NOT NULL CHECK (origin IN ('generated', 'kept')),
 updated_at TEXT NOT NULL,
 PRIMARY KEY (site_id, data_epoch, document_id)
);
