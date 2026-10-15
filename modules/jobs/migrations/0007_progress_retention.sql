-- 只追加可信进度时刻；旧行不能借任务创建时间伪造7天重放窗口。
ALTER TABLE progress ADD COLUMN created_at TEXT;

-- 同job/seq/epoch的每个持久来源都参与核对：不完整或矛盾来源产生NULL，
-- 不通过过滤坏来源后挑一个“好时间”。公共outbox由唯一EnsureSchema创建。
WITH source_times AS (
    SELECT p.job_id, p.seq,
      CASE WHEN json_valid(CAST(o.payload AS TEXT))=1 THEN
        CASE WHEN json_extract(CAST(o.payload AS TEXT),'$.jobId')=p.job_id AND json_extract(CAST(o.payload AS TEXT),'$.seq')=p.seq
          AND json_extract(CAST(o.payload AS TEXT),'$.phase')=p.phase
          AND json_extract(CAST(o.payload AS TEXT),'$.completedUnits')=p.completed_units
          AND json_extract(CAST(o.payload AS TEXT),'$.totalUnits')=p.total_units THEN o.occurred_at END
      END AS at
    FROM progress p JOIN jobs j ON j.job_id=p.job_id JOIN nw_outbox o
      ON o.producer='jobs' AND o.event_type='jobs.job-progress.v1' AND o.event_version=1
      AND o.subject_id=p.job_id AND o.revision=p.seq AND o.data_epoch=j.data_epoch
    UNION ALL
    SELECT p.job_id, p.seq,
      CASE WHEN json_valid(CAST(e.envelope_json AS TEXT))=1 THEN
        CASE WHEN json_extract(CAST(e.envelope_json AS TEXT),'$.producer')='jobs'
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.eventType')='jobs.job-progress.v1'
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.eventVersion')=1
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.subjectId')=p.job_id
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.revision')=p.seq
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.dataEpoch')=j.data_epoch
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.payload.jobId')=p.job_id
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.payload.seq')=p.seq
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.payload.phase')=p.phase
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.payload.completedUnits')=p.completed_units
          AND json_extract(CAST(e.envelope_json AS TEXT),'$.payload.totalUnits')=p.total_units
          AND json_type(CAST(e.envelope_json AS TEXT),'$.occurredAt')='text'
        THEN json_extract(CAST(e.envelope_json AS TEXT),'$.occurredAt') END
      END AS at
    FROM progress p JOIN jobs j ON j.job_id=p.job_id JOIN events e
      ON e.producer='jobs' AND e.event_type='jobs.job-progress.v1' AND e.event_version=1
      AND e.subject_id=p.job_id AND e.source_revision=p.seq AND e.data_epoch=j.data_epoch
), consistent AS (
    SELECT job_id,seq,MIN(at) AS at FROM source_times GROUP BY job_id,seq
    HAVING COUNT(*)=COUNT(at) AND COUNT(DISTINCT at)=1 AND MIN(at)<>''
)
UPDATE progress SET created_at=(SELECT at FROM consistent s WHERE s.job_id=progress.job_id AND s.seq=progress.seq);
