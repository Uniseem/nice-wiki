-- 原生工具消息独立于访客原话保存，避免把访客JSON误识别成可重放的角色/工具消息。
ALTER TABLE messages ADD COLUMN agent_messages_json BLOB;

-- 旧默认预算升级为agent查阅预算；保留管理员改过的设置。
UPDATE qa_policy
SET version=version+1,
    policy_json=json_set(policy_json,'$.version',version+1,'$.maximumRetrievalRounds',12,'$.maximumToolCalls',48)
WHERE json_extract(policy_json,'$.maximumRetrievalRounds')=3
  AND json_extract(policy_json,'$.maximumToolCalls')=5;
