package analytics

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"strings"
	"time"
)

type trustedAuditFact struct {
	Record     AuditRecord
	Historical bool
}

// AppendAudit只在固定producer边界接收已提交事实，不拿服务主体冒充原用户。
func AppendAudit(ctx context.Context, op OperationContext, in AppendAuditInput) (AppendAuditResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return AppendAuditResult{}, err
	}
	producer, ok := auditActionProducer[in.Action]
	if !ok || producer != op.CallerService {
		return AppendAuditResult{}, platform.NewError(platform.CodeForbidden, false, "审计动作不属于该生产者")
	}
	present := 0
	if in.ActorKind != nil {
		present++
	}
	if in.OccurredAt != nil {
		present++
	}
	if in.SourceOperationID != nil {
		present++
	}
	if present != 0 && present != 3 {
		return AppendAuditResult{}, platform.NewError(platform.CodeValidation, false, "历史审计来源字段必须完整同现")
	}
	historical := present == 3
	actorKind := op.Actor.Kind
	occurredAt := time.Now().UTC().Format(analyticsTimeLayout)
	sourceOperationID := op.RootOperationID
	if historical {
		if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService {
			return AppendAuditResult{}, platform.NewError(platform.CodeForbidden, false, "历史审计必须由原固定生产者纯服务提交")
		}
		actorKind = *in.ActorKind
		occurredAt = *in.OccurredAt
		sourceOperationID = *in.SourceOperationID
	} else if in.ActorID != op.Actor.SubjectID {
		return AppendAuditResult{}, platform.NewError(platform.CodeForbidden, false, "实时审计主体与可信委托不同")
	}
	payload := AuditRecordedPayload{AuditID: in.AuditID, ActorID: in.ActorID, ActorKind: actorKind, OccurredAt: occurredAt, SourceOperationID: sourceOperationID, SiteID: rt.SiteID, Action: in.Action, ResourceID: in.ResourceID, ResultCode: in.ResultCode, Metadata: in.Metadata, DataEpoch: op.DataEpoch}
	record, err := normalizeAuditFact(ctx, op, producer, payload, historical)
	if err != nil {
		return AppendAuditResult{}, err
	}
	result, _, err := appendTrustedAudit(ctx, op, trustedAuditFact{Record: record, Historical: historical}, nil)
	return result, err
}

// appendTrustedAudit的origin只来自已经核验的HTTP producer或事件envelope，不重写op.CallerService。
func appendTrustedAudit(ctx context.Context, op OperationContext, fact trustedAuditFact, envelope *platform.EventEnvelope) (AppendAuditResult, bool, error) {
	record := fact.Record
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return AppendAuditResult{}, false, err
	}
	defer cancel()
	defer tx.Rollback()
	newInbox := false
	if envelope != nil {
		claimed, claimErr := inbox.ClaimInbox(tx, *envelope)
		if claimErr != nil {
			return AppendAuditResult{}, false, claimErr
		}
		if !claimed {
			if err = tx.Commit(); err != nil {
				return AppendAuditResult{}, false, storageError(err)
			}
			return AppendAuditResult{}, false, nil
		}
		newInbox = true
	}
	// 实时旧协议的接收时间/root由首次接收固定，重试不把新传输时间当不同原事实。
	digestRecord := record
	if !fact.Historical {
		digestRecord.OccurredAt = ""
		digestRecord.SourceOperationID = ""
	}
	digestJSON, _ := platform.CanonicalJSON(struct {
		Record     AuditRecord
		Historical bool
	}{digestRecord, fact.Historical})
	digest := platform.HashBytes(digestJSON)
	var oldDigest, oldHash string
	var seq int64
	err = tx.QueryRowContext(work, `SELECT seq,input_hash,hash FROM audit_records WHERE audit_id=?`, record.AuditID).Scan(&seq, &oldDigest, &oldHash)
	if err == nil {
		if oldDigest != digest {
			return AppendAuditResult{}, false, platform.NewError(platform.CodeConflict, false, "审计ID已用于不同原事实")
		}
		if newInbox {
			if _, err = advanceWatermark(work, tx, "audit-inbox"); err != nil {
				return AppendAuditResult{}, false, err
			}
		}
		if err = tx.Commit(); err != nil {
			return AppendAuditResult{}, false, storageError(err)
		}
		return AppendAuditResult{Seq: seq, Hash: oldHash}, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return AppendAuditResult{}, false, storageError(err)
	}
	result, err := insertAuditRecord(work, op, tx, record, fact.Historical, envelope, digest)
	if err != nil {
		return AppendAuditResult{}, false, err
	}
	if _, err = advanceWatermark(work, tx, "audits"); err != nil {
		return AppendAuditResult{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return AppendAuditResult{}, false, storageError(err)
	}
	return result, true, nil
}

// insertAuditRecord只写已认证事实，与调用方保留期锚/业务动作同事务。
func insertAuditRecord(work context.Context, op OperationContext, tx *sql.Tx, record AuditRecord, historical bool, envelope *platform.EventEnvelope, digest string) (AppendAuditResult, error) {
	var err error
	var seq int64
	record.PreviousHash = strings.Repeat("0", 64)
	err = tx.QueryRowContext(work, `SELECT hash FROM audit_records ORDER BY seq DESC LIMIT 1`).Scan(&record.PreviousHash)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(work, `SELECT hash FROM audit_anchors ORDER BY last_seq DESC LIMIT 1`).Scan(&record.PreviousHash)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
	}
	if err != nil {
		return AppendAuditResult{}, storageError(err)
	}
	hashed, err := ComputeAuditHash(work, op, ComputeAuditHashInput{PreviousHash: record.PreviousHash, AuditJSON: record})
	if err != nil {
		return AppendAuditResult{}, err
	}
	record.Hash = hashed.Hash
	mode := "live"
	if historical {
		mode = "historical"
	}
	var eventID any
	if envelope != nil {
		eventID = envelope.EventID
	}
	err = tx.QueryRowContext(work, `INSERT INTO audit_records(audit_id,event_id,actor_id,actor_kind,action,resource_ref,result_code,occurred_at,hash,previous_hash,metadata,data_epoch,producer,source_operation_id,hash_schema_version,origin_mode,input_hash,origin_epoch,origin_site_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,2,?,?,?,?) RETURNING seq`, record.AuditID, eventID, record.ActorID, record.ActorKind, record.Action, record.ResourceID, record.ResultCode, record.OccurredAt, record.Hash, record.PreviousHash, record.Metadata, op.DataEpoch, record.Producer, record.SourceOperationID, mode, digest, record.OriginEpoch, record.SiteID).Scan(&seq)
	if err != nil {
		return AppendAuditResult{}, storageError(err)
	}
	return AppendAuditResult{Seq: seq, Hash: record.Hash}, nil
}
