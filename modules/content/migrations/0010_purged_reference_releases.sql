-- 已向资源服务发出“空引用集合”释放事件的彻底删除文档；引用列表只跳过这些文档，未释放的旧文档由事件转发循环补发。
CREATE TABLE purged_reference_releases (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    PRIMARY KEY(site_id,document_id)
);
