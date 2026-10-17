-- 语言配置写入的原回执、代次水位与本地屏障读侧。
-- 不播种语言、费用或翻译任务；不修改已发布的 001_initial.sql。
-- AcquireBarrier / ReleaseBarrier 仍未实现。初始状态为 released，
-- 语言写入只拒绝非 released；不得把本表当成屏障协议已经完成。

CREATE TABLE operation_receipts (
    site_id          TEXT NOT NULL,
    actor_id         TEXT NOT NULL,
    function_id      TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    data_epoch       TEXT NOT NULL,
    input_hash       TEXT NOT NULL,
    result_json      BLOB NOT NULL,
    created_at       TEXT NOT NULL,
    PRIMARY KEY (site_id, actor_id, function_id, idempotency_key)
);

CREATE TABLE runtime_meta (
    singleton   INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

INSERT INTO runtime_meta (singleton, data_epoch, updated_at) VALUES (1, '', '');

CREATE TABLE local_barrier (
    singleton   INTEGER PRIMARY KEY CHECK (singleton = 1),
    state       TEXT NOT NULL CHECK (state IN ('draining', 'safe', 'released', 'failed')),
    data_epoch  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

INSERT INTO local_barrier (singleton, state, data_epoch, updated_at) VALUES (1, 'released', '', '');
