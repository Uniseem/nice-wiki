-- schemaVersion 7：移除答案缓存与前置策略；默认问答记录保留15天。
DROP TABLE cache_vectors;
DROP TABLE cache_calibration;
DROP TABLE answer_cache;
UPDATE qa_policy SET policy_json=json_remove(policy_json,'$.cacheEnabled','$.cacheTtlDays','$.semanticMode','$.semanticThreshold','$.preflightOutputTokens','$.inspectionOutputTokens','$.maximumSubqueries');
UPDATE qa_policy SET policy_json=json_set(policy_json,'$.recordRetentionDays',15) WHERE json_extract(policy_json,'$.recordRetentionDays')=30;
UPDATE conversations SET expires_at=strftime('%Y-%m-%dT%H:%M:%fZ',(SELECT MIN(created_ns)/1000000000 FROM turns WHERE turns.conversation_id=conversations.conversation_id),'unixepoch','+15 days') WHERE state='active' AND EXISTS(SELECT 1 FROM turns WHERE turns.conversation_id=conversations.conversation_id) AND (SELECT json_extract(policy_json,'$.recordRetentionDays') FROM qa_policy)=15;
DELETE FROM qa_receipts WHERE function_id LIKE '%Cache%';
UPDATE qa_policy SET version=version+1,policy_json=json_set(policy_json,'$.version',version+1);
UPDATE turns SET answer_kind=state WHERE state IN ('failed','cancelled') AND answer_kind='';
PRAGMA user_version=7;
