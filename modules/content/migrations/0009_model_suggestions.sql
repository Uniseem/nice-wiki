-- 首次发布时模型给出的路径名、搜索摘要：按任务、档位和输入哈希保存完整文本，同样的输入直接复用，不再付费。
CREATE TABLE model_suggestions (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    task_id TEXT NOT NULL,
    input_key TEXT NOT NULL,
    output_text TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(site_id,data_epoch,task_id,input_key)
);
