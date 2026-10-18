package analytics

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"time"
)

// ValidateUsageEvent核V2账项金额与状态语义；生产字段和枚举由生成schema校验。
func ValidateUsageEvent(change UsageLedgerChange) error {
	if change.AmountKnown != (change.Amount != nil) {
		return platform.NewError(platform.CodeIntegrityError, false, "费用账项金额已知标志与字段不一致")
	}
	if change.AmountKnown && (change.Amount.Currency != "USD" || change.Amount.Micros < 0 || change.Amount.Micros > 9007199254740991) {
		return platform.NewError(platform.CodeIntegrityError, false, "费用账项真实金额不合法")
	}
	if change.LedgerStatus == "price_unknown" && change.AmountKnown {
		return platform.NewError(platform.CodeIntegrityError, false, "未知价格账项不能带已知金额")
	}
	if change.State != "superseded" && change.State != "voided" && change.State != "deleted" && change.State != change.LedgerStatus {
		return platform.NewError(platform.CodeIntegrityError, false, "费用账项状态与账本状态矛盾")
	}
	return nil
}

// ProjectUsage只接读取边界已核验的完整V2事实，不从事件引用推测金额或修订。
// 事件入口应使用ProjectUsageTx，与平台inbox认领在同一安全写事务提交。
func ProjectUsage(ctx context.Context, op OperationContext, in UsageProjectionInput) (ProjectUsageResult, error) {
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return ProjectUsageResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	result, err := ProjectUsageTx(work, op, tx, in)
	if err != nil {
		return ProjectUsageResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return ProjectUsageResult{}, storageError(err)
	}
	return result, nil
}

// ProjectUsageTx不新建或提交事务。调用方必须先通过beginWrite取得本域安全写事务，
// 在其中ClaimInboxBeforeWrite，再调用本函数；任何错误均不得提交业务写入。
// usage水位是实际持久事实的最大changeRevision，不是本域自增修订、事件修订或
// 远端尚未收齐的watermark；本函数不建立usage-v2-complete标记。
func ProjectUsageTx(ctx context.Context, op OperationContext, tx *sql.Tx, in UsageProjectionInput) (ProjectUsageResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return ProjectUsageResult{}, err
	}
	if in.DataEpoch != rt.Epoch {
		return ProjectUsageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "费用投影当前代次不匹配")
	}
	var site, epoch string
	var schema int
	if err = tx.QueryRowContext(ctx, `SELECT schema_version,site_id,data_epoch FROM analytics_meta WHERE singleton=1`).Scan(&schema, &site, &epoch); err != nil {
		return ProjectUsageResult{}, storageError(err)
	}
	if site != rt.SiteID || epoch != rt.Epoch {
		return ProjectUsageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "费用投影持久站点或代次已变化")
	}
	if schema != 6 {
		return ProjectUsageResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "完整费用投影要求schemaVersion6")
	}
	result := ProjectUsageResult{}
	var newest int64
	var recorded bool
	for _, change := range in.Changes {
		if err = ValidateUsageEvent(change); err != nil {
			return ProjectUsageResult{}, err
		}
		if change.SiteID != site || change.SourceDataEpoch == "" {
			return ProjectUsageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "账项原站点或来源代次缺失/不匹配")
		}
		raw, e := platform.CanonicalJSON(change)
		if e != nil {
			return ProjectUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "完整账项事实不能规范编码")
		}
		fact := usageStoredFact{Kind: "ledger", EntityID: change.LedgerEntryID, EntityRevision: change.LedgerRevision, ChangeRevision: change.ChangeRevision, SiteID: change.SiteID, SourceDataEpoch: change.SourceDataEpoch, Hash: platform.HashBytes(raw), JSON: string(raw)}
		fresh, e := recordUsageFact(ctx, tx, epoch, fact)
		if e != nil {
			return ProjectUsageResult{}, e
		}
		if fresh {
			applied, e := projectUsageLedger(ctx, tx, epoch, change, fact)
			if e != nil {
				return ProjectUsageResult{}, e
			}
			recorded = true
			result.Applied = result.Applied || applied
		}
		if change.ChangeRevision > newest {
			newest = change.ChangeRevision
		}
	}
	for _, observation := range in.Observations {
		if observation.Origin.SiteID != site || observation.Origin.SourceDataEpoch == "" {
			return ProjectUsageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "未入账观察原站点或来源代次缺失/不匹配")
		}
		// 最大额与held额仅作为原预留事实保留；未知时不编码Money零值。
		if reservation := observation.Reservation; reservation != nil {
			if reservation.AmountKnown != (reservation.MaximumCost != nil) || reservation.AmountKnown != (reservation.HeldCost != nil) {
				return ProjectUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "预留已知金额标记与原金额不一致")
			}
		}
		raw, e := platform.CanonicalJSON(observation)
		if e != nil {
			return ProjectUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "完整未入账观察不能规范编码")
		}
		fact := usageStoredFact{Kind: observation.Kind, EntityID: observation.EntityID, EntityRevision: observation.EntityRevision, ChangeRevision: observation.ChangeRevision, SiteID: observation.Origin.SiteID, SourceDataEpoch: observation.Origin.SourceDataEpoch, Hash: platform.HashBytes(raw), JSON: string(raw)}
		fresh, e := recordUsageFact(ctx, tx, epoch, fact)
		if e != nil {
			return ProjectUsageResult{}, e
		}
		if fresh {
			applied, e := projectUsageObservation(ctx, tx, epoch, observation, fact)
			if e != nil {
				return ProjectUsageResult{}, e
			}
			recorded = true
			result.Applied = result.Applied || applied
		}
		if observation.ChangeRevision > newest {
			newest = observation.ChangeRevision
		}
	}
	if recorded {
		var revision int64
		if err = tx.QueryRowContext(ctx, `UPDATE analytics_meta SET revision=revision+1 WHERE singleton=1 AND revision<9223372036854775807 RETURNING revision`).Scan(&revision); err != nil {
			return ProjectUsageResult{}, storageError(err)
		}
	}
	if newest > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO aggregation_watermarks(source,revision,updated_at) VALUES('usage',?,?) ON CONFLICT(source) DO UPDATE SET revision=excluded.revision,updated_at=excluded.updated_at WHERE excluded.revision>aggregation_watermarks.revision`, newest, time.Now().UTC().Format(analyticsTimeLayout))
		if err != nil {
			return ProjectUsageResult{}, storageError(err)
		}
	}
	return result, nil
}

type usageStoredFact struct {
	Kind, EntityID, SiteID, SourceDataEpoch, Hash, JSON string
	EntityRevision, ChangeRevision                    int64
}

// recordUsageFact保留每个完整历史修订，因此迟到旧修订也不能绕过同修订hash冲突。
func recordUsageFact(ctx context.Context, tx *sql.Tx, epoch string, fact usageStoredFact) (bool, error) {
	var kind, entity, hash string
	var revision int64
	err := tx.QueryRowContext(ctx, `SELECT fact_kind,entity_id,entity_revision,payload_hash FROM usage_facts_v2 WHERE data_epoch=? AND change_revision=?`, epoch, fact.ChangeRevision).Scan(&kind, &entity, &revision, &hash)
	if err == nil {
		if kind != fact.Kind || entity != fact.EntityID || revision != fact.EntityRevision || hash != fact.Hash {
			return false, platform.NewError(platform.CodeConflict, false, "同费用时间线修订对应不同原事实")
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, storageError(err)
	}
	err = tx.QueryRowContext(ctx, `SELECT change_revision,payload_hash FROM usage_facts_v2 WHERE data_epoch=? AND fact_kind=? AND entity_id=? AND entity_revision=?`, epoch, fact.Kind, fact.EntityID, fact.EntityRevision).Scan(&revision, &hash)
	if err == nil {
		return false, platform.NewError(platform.CodeConflict, false, "同费用实体修订对应不同时间线或完整事实")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, storageError(err)
	}
	// 单调关系同时绑定实体修订和全局来源修订；乱序到达只留历史，不使当前行倒退。
	var inconsistent bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_facts_v2 WHERE data_epoch=? AND fact_kind=? AND entity_id=? AND ((entity_revision<? AND change_revision>=?) OR (entity_revision>? AND change_revision<=?)))`, epoch, fact.Kind, fact.EntityID, fact.EntityRevision, fact.ChangeRevision, fact.EntityRevision, fact.ChangeRevision).Scan(&inconsistent)
	if err != nil {
		return false, storageError(err)
	}
	if inconsistent {
		return false, platform.NewError(platform.CodeConflict, false, "费用实体修订与来源时间线顺序矛盾")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO usage_facts_v2(data_epoch,change_revision,fact_kind,entity_id,entity_revision,site_id,source_data_epoch,payload_hash,fact_json) VALUES(?,?,?,?,?,?,?,?,?)`, epoch, fact.ChangeRevision, fact.Kind, fact.EntityID, fact.EntityRevision, fact.SiteID, fact.SourceDataEpoch, fact.Hash, fact.JSON)
	return err == nil, storageError(err)
}

func projectUsageLedger(ctx context.Context, tx *sql.Tx, epoch string, change UsageLedgerChange, fact usageStoredFact) (bool, error) {
	var amount, currency any
	if change.Amount != nil {
		amount, currency = change.Amount.Micros, change.Amount.Currency
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO usage_projection_v2(data_epoch,ledger_entry_id,ledger_revision,change_revision,invocation_id,attempt_id,owner_service,site_id,source_data_epoch,operation_id,root_operation_id,cost_class,provider_id,provider_version,model_id,model_version,model_snapshot_hash,amount_known,amount_micros,currency,state,ledger_status,occurred_at,observed_at,capture_kind,payload_hash,fact_json)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(data_epoch,ledger_entry_id) DO UPDATE SET
 ledger_revision=excluded.ledger_revision,change_revision=excluded.change_revision,
 invocation_id=excluded.invocation_id,attempt_id=excluded.attempt_id,
 owner_service=excluded.owner_service,site_id=excluded.site_id,source_data_epoch=excluded.source_data_epoch,
 operation_id=excluded.operation_id,root_operation_id=excluded.root_operation_id,cost_class=excluded.cost_class,
 provider_id=excluded.provider_id,provider_version=excluded.provider_version,model_id=excluded.model_id,
 model_version=excluded.model_version,model_snapshot_hash=excluded.model_snapshot_hash,
 amount_known=excluded.amount_known,amount_micros=excluded.amount_micros,currency=excluded.currency,
 state=excluded.state,ledger_status=excluded.ledger_status,occurred_at=excluded.occurred_at,
 observed_at=excluded.observed_at,capture_kind=excluded.capture_kind,payload_hash=excluded.payload_hash,fact_json=excluded.fact_json
WHERE excluded.ledger_revision>usage_projection_v2.ledger_revision`, epoch, change.LedgerEntryID, change.LedgerRevision, change.ChangeRevision, change.InvocationID, change.AttemptID, change.OwnerService, change.SiteID, change.SourceDataEpoch, change.OperationID, change.RootOperationID, change.CostClass, change.ProviderID, change.ProviderVersion, change.ModelID, change.ModelVersion, change.ModelSnapshotHash, change.AmountKnown, amount, currency, change.State, change.LedgerStatus, change.OccurredAt, change.ObservedAt, change.CaptureKind, fact.Hash, fact.JSON)
	if err != nil {
		return false, storageError(err)
	}
	count, err := result.RowsAffected()
	return count != 0, storageError(err)
}

func projectUsageObservation(ctx context.Context, tx *sql.Tx, epoch string, observation UsageObservation, fact usageStoredFact) (bool, error) {
	origin := observation.Origin
	result, err := tx.ExecContext(ctx, `INSERT INTO usage_observations_v2(data_epoch,kind,entity_id,entity_revision,change_revision,invocation_id,owner_service,site_id,source_data_epoch,operation_id,root_operation_id,cost_class,deleted,observed_at,payload_hash,fact_json)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(data_epoch,kind,entity_id) DO UPDATE SET
 entity_revision=excluded.entity_revision,change_revision=excluded.change_revision,
 invocation_id=excluded.invocation_id,owner_service=excluded.owner_service,site_id=excluded.site_id,
 source_data_epoch=excluded.source_data_epoch,operation_id=excluded.operation_id,
 root_operation_id=excluded.root_operation_id,cost_class=excluded.cost_class,
 deleted=excluded.deleted,observed_at=excluded.observed_at,payload_hash=excluded.payload_hash,fact_json=excluded.fact_json
WHERE excluded.entity_revision>usage_observations_v2.entity_revision`, epoch, observation.Kind, observation.EntityID, observation.EntityRevision, observation.ChangeRevision, origin.InvocationID, origin.OwnerService, origin.SiteID, origin.SourceDataEpoch, origin.OperationID, origin.RootOperationID, origin.CostClass, observation.Deleted, observation.ObservedAt, fact.Hash, fact.JSON)
	if err != nil {
		return false, storageError(err)
	}
	count, err := result.RowsAffected()
	return count != 0, storageError(err)
}
