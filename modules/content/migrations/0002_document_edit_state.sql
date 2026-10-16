-- 文档存在/生命周期与删除fence权威；租约只在内存，不能写入快照。
-- 本迁移不从请求或正文构造文档。CreateDocument与后续生命周期用例尚未交付，
-- 只能由这些用例的完整业务事务插入/改变本表；缺行按NOT_FOUND关闭编辑。
CREATE TABLE documents (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('draft','published','trash','purged')),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    delete_fence INTEGER NOT NULL CHECK (delete_fence BETWEEN 1 AND 9007199254740991),
    PRIMARY KEY (site_id,document_id)
);
