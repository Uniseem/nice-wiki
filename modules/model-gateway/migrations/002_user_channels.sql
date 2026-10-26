-- 用户自带的 AI 渠道（BYOK）。渠道本身是普通的 providers/models/credentials 行（id 以 byok- 开头），
-- 这里记录归属与可用任务；费用承担方记在 fee_commitments.payer：
--   platform = 平台渠道，计入站点所有者的 AI 额度；user = 用户自带 key，平台不付钱；infra = 平台自己承担（嵌入、站主自己的站点）。
CREATE TABLE user_channels (
    channel_id        TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL,
    name              TEXT NOT NULL,
    model_id          TEXT NOT NULL,
    model_name        TEXT NOT NULL,
    base_url          TEXT NOT NULL,
    key_last4         TEXT NOT NULL,
    tasks_json        TEXT NOT NULL DEFAULT '[]',
    capabilities_json TEXT NOT NULL,
    position          INTEGER NOT NULL DEFAULT 0,
    probe_json        TEXT NOT NULL DEFAULT '{}',
    created_at        TEXT NOT NULL
);
CREATE INDEX user_channels_user ON user_channels (user_id, position);
ALTER TABLE fee_commitments ADD COLUMN payer TEXT NOT NULL DEFAULT 'platform';
CREATE INDEX fee_commitments_payer ON fee_commitments (payer, site_id, created_ns);
