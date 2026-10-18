package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"slices"
	"time"
)

type ReadUsageStatisticsPageInput struct {
	AfterRevision int64  `json:"afterRevision,omitempty"`
	Watermark     int64  `json:"watermark,omitempty"`
	Cursor        string `json:"cursor,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}
type UsageLedgerChange struct {
	ChangeRevision    int64      `json:"changeRevision"`
	LedgerEntryID     string     `json:"ledgerEntryId"`
	LedgerRevision    int64      `json:"ledgerRevision"`
	InvocationID      string     `json:"invocationId"`
	AttemptID         string     `json:"attemptId"`
	OperationID       string     `json:"operationId"`
	RootOperationID   string     `json:"rootOperationId"`
	OwnerService      string     `json:"ownerService"`
	SiteID            string     `json:"siteId"`
	SourceDataEpoch   string     `json:"sourceDataEpoch"`
	CostClass         string     `json:"costClass"`
	ProviderID        string     `json:"providerId"`
	ProviderVersion   int        `json:"providerVersion"`
	ModelID           string     `json:"modelId"`
	ModelVersion      int        `json:"modelVersion"`
	ModelSnapshotHash string     `json:"modelSnapshotHash"`
	AmountKnown       bool       `json:"amountKnown"`
	Amount            *gen.Money `json:"amount,omitempty"`
	State             string     `json:"state"`
	LedgerStatus      string     `json:"ledgerStatus"`
	OccurredAt        string     `json:"occurredAt"`
	ObservedAt        string     `json:"observedAt"`
	CaptureKind       string     `json:"captureKind"`
}
type ReadUsageStatisticsPageResult struct {
	SchemaVersion       int                 `json:"schemaVersion"`
	Watermark           int64               `json:"watermark"`
	ThroughRevision     int64               `json:"throughRevision"`
	Changes             []UsageLedgerChange `json:"changes"`
	Observations        []UsageObservation  `json:"observations"`
	NextCursor          string              `json:"nextCursor"`
	FeeCoverage         string              `json:"feeCoverage"`
	CompleteFeeCoverage bool                `json:"completeFeeCoverage"`
	PageHash            string              `json:"pageHash,omitempty"`
}
type statisticsCursor struct {
	Version int    `json:"version"`
	Since   int64  `json:"since"`
	Through int64  `json:"through"`
	Upper   int64  `json:"upper"`
	Binding string `json:"binding"`
	Expires int64  `json:"expires"`
}
type storedUsageChange struct {
	LedgerEntryID     string  `json:"ledgerEntryId"`
	InvocationID      string  `json:"invocationId"`
	AttemptID         string  `json:"attemptId"`
	ActualMicros      *int64  `json:"actualMicros"`
	AmountKnown       *int    `json:"amountKnown"`
	LedgerStatus      string  `json:"ledgerStatus"`
	OccurredAt        string  `json:"occurredAt"`
	SiteID            string  `json:"siteId"`
	SourceDataEpoch   string  `json:"sourceDataEpoch"`
	OwnerService      string  `json:"ownerService"`
	OperationID       string  `json:"operationId"`
	RootOperationID   string  `json:"rootOperationId"`
	CostClass         string  `json:"costClass"`
	ProviderID        string  `json:"providerId"`
	ProviderVersion   int     `json:"providerVersion"`
	ModelID           string  `json:"modelId"`
	ModelVersion      int     `json:"modelVersion"`
	ModelSnapshotHash *string `json:"modelSnapshotHash"`
	PriceSnapshotID   string  `json:"priceSnapshotId"`
	ExecutionHash     string  `json:"executionHash"`
	State             string  `json:"state"`
	CaptureKind       string  `json:"captureKind"`
}

// ReadUsageStatisticsPage纯读统一不可变上界，完整保留posted及无账项未知负债，金额未知不补0。
func ReadUsageStatisticsPage(ctx context.Context, op OperationContext, input ReadUsageStatisticsPageInput) (ReadUsageStatisticsPageResult, error) {
	if op.CallerService != "analytics" || op.Actor.Kind != "service" || op.Actor.SubjectID != "analytics" {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeForbidden, false, "安全统计账页只允许analytics纯服务")
	}
	options, err := gatewayruntime.ExecutionDependencies()
	if err != nil {
		return ReadUsageStatisticsPageResult{}, err
	}
	if options.SiteID == "" {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "网关当前真实站点尚未装配")
	}
	if op.Actor.SiteID != options.SiteID {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeForbidden, false, "安全统计站点不是当前实际站点")
	}
	runtime, err := ledgerRuntime(ctx, op)
	if err != nil {
		return ReadUsageStatisticsPageResult{}, err
	}
	work, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	tx, err := beginAdminSnapshot(work, op)
	if err != nil {
		return ReadUsageStatisticsPageResult{}, err
	}
	defer tx.Rollback()
	var format int
	var trackingEpoch, trackingKind, trackingHash string
	err = tx.QueryRowContext(work, `SELECT format_version,data_epoch,proof_kind,proof_hash FROM invocation_lookup_tracking WHERE singleton=1`).Scan(&format, &trackingEpoch, &trackingKind, &trackingHash)
	if err == sql.ErrNoRows {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "完整财务来源尚缺真实首次创建或认证恢复证明")
	}
	if err != nil {
		return ReadUsageStatisticsPageResult{}, adminStorageError(err)
	}
	if format != 1 || trackingEpoch != op.DataEpoch || !slices.Contains([]string{"fresh_activation", "verified_restore"}, trackingKind) || !platform.ValidHash(trackingHash) {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeIntegrityError, false, "完整财务来源tracking证据损坏")
	}
	if err = validateLedgerCoverage(work, op, tx); err != nil {
		return ReadUsageStatisticsPageResult{}, err
	}
	limit := input.Limit
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 100 {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeValidation, false, "安全统计页大小必须1至100")
	}
	bindingRaw, _ := platform.CanonicalJSON([]any{op.CallerService, op.Actor.SubjectID, options.SiteID, op.DataEpoch, input.AfterRevision, limit})
	binding := platform.HashBytes(bindingRaw)
	scope := "model-gateway.statistics.v2:" + binding
	var current int64
	if err = tx.QueryRowContext(work, `SELECT coalesce(max(sequence),0) FROM usage_statistics_timeline`).Scan(&current); err != nil {
		return ReadUsageStatisticsPageResult{}, adminStorageError(err)
	}
	upper := input.Watermark
	if upper == 0 {
		upper = current
	}
	cursor := statisticsCursor{Version: 2, Since: input.AfterRevision, Through: input.AfterRevision, Upper: upper, Binding: binding, Expires: time.Now().Add(24 * time.Hour).Unix()}
	if input.Cursor != "" {
		raw, e := serviceauth.VerifyCursor(runtime.Key, scope, input.Cursor)
		if e != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Version != 2 || cursor.Binding != binding || cursor.Since != input.AfterRevision || cursor.Through < cursor.Since || cursor.Through > cursor.Upper || cursor.Expires <= time.Now().Unix() || (input.Watermark != 0 && input.Watermark != cursor.Upper) {
			return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeStaleResult, false, "统计游标过期或站点代次/上界/筛选已改变")
		}
	}
	if cursor.Upper > current || cursor.Since < 0 || cursor.Upper < cursor.Since {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeStaleResult, false, "统计历史上界不存在或已不相容")
	}
	var missing bool
	if err = tx.QueryRowContext(work, `SELECT EXISTS(SELECT 1 FROM usage_statistics_timeline t LEFT JOIN usage_statistics_changes l ON t.kind='ledger' AND l.change_revision=t.source_revision LEFT JOIN usage_observation_changes o ON t.kind='observation' AND o.revision=t.source_revision WHERE t.sequence<=? AND (coalesce(l.site_id,o.site_id,'')='' OR coalesce(l.source_epoch,o.source_epoch,'')=''))`, cursor.Upper).Scan(&missing); err != nil {
		return ReadUsageStatisticsPageResult{}, adminStorageError(err)
	}
	if missing {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "历史费用/无账项观察缺真实站点与来源代次，不能过滤成空页")
	}
	rows, err := tx.QueryContext(work, `SELECT t.sequence,t.kind,coalesce(l.entry_id,o.entity_id),coalesce(l.ledger_revision,o.entity_revision),coalesce(l.snapshot_json,o.snapshot_json),coalesce(l.observed_at,o.observed_at),coalesce(o.kind,''),coalesce(o.deleted,0) FROM usage_statistics_timeline t LEFT JOIN usage_statistics_changes l ON t.kind='ledger' AND l.change_revision=t.source_revision LEFT JOIN usage_observation_changes o ON t.kind='observation' AND o.revision=t.source_revision WHERE coalesce(l.site_id,o.site_id)=? AND t.sequence>? AND t.sequence<=? ORDER BY t.sequence LIMIT ?`, options.SiteID, cursor.Through, cursor.Upper, limit+1)
	if err != nil {
		return ReadUsageStatisticsPageResult{}, adminStorageError(err)
	}
	type row struct {
		Change, Version                     int64
		Kind, ID, Observed, ObservationKind string
		Raw                                 []byte
		Deleted                             bool
	}
	page := []row{}
	for rows.Next() {
		var r row
		if err = rows.Scan(&r.Change, &r.Kind, &r.ID, &r.Version, &r.Raw, &r.Observed, &r.ObservationKind, &r.Deleted); err != nil {
			break
		}
		page = append(page, r)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return ReadUsageStatisticsPageResult{}, adminStorageError(err)
	}
	result := ReadUsageStatisticsPageResult{SchemaVersion: 2, Watermark: cursor.Upper, ThroughRevision: cursor.Through, Changes: []UsageLedgerChange{}, Observations: []UsageObservation{}, FeeCoverage: "posted-and-inflight", CompleteFeeCoverage: true}
	more := len(page) > limit
	if more {
		page = page[:limit]
	}
	cache := map[string]statisticsLineage{}
	for _, r := range page {
		if r.Kind == "observation" {
			observation, e := ReadUsageObservation(work, op, tx, r.ObservationKind, r.ID, r.Raw, r.Change, r.Version, r.Deleted, r.Observed)
			if e != nil {
				return ReadUsageStatisticsPageResult{}, e
			}
			result.Observations = append(result.Observations, observation)
		} else if r.Kind == "ledger" {
			if len(r.Raw) > 16384 {
				return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "统计原安全快照超过16KiB")
			}
			var stored storedUsageChange
			if json.Unmarshal(r.Raw, &stored) != nil || stored.LedgerEntryID != r.ID {
				return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeIntegrityError, false, "统计原账项快照损坏")
			}
			if stored.OwnerService == "" || stored.OperationID == "" || stored.RootOperationID == "" || stored.ExecutionHash == "" || stored.ProviderID == "" || stored.ModelID == "" || stored.ProviderVersion < 1 || stored.ModelVersion < 1 || stored.PriceSnapshotID == "" {
				return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "旧账项缺必要冻结来源，不能补当前配置或空值")
			}
			canonical, _ := platform.CanonicalJSON(stored)
			original, e := platform.CanonicalJSON(json.RawMessage(r.Raw))
			if e != nil || platform.HashBytes(canonical) != platform.HashBytes(original) {
				return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeIntegrityError, false, "统计原快照含未知字段，不能损失信息读取")
			}
			lineage, ok := cache[stored.AttemptID]
			if !ok {
				lineage, err = readStatisticsLineage(work, op, tx, stored)
				if err != nil {
					return ReadUsageStatisticsPageResult{}, err
				}
				cache[stored.AttemptID] = lineage
			}
			change, e := validateUsageChange(stored, lineage, r.Change, r.Version, r.Observed)
			if e != nil {
				return ReadUsageStatisticsPageResult{}, e
			}
			result.Changes = append(result.Changes, change)
		} else {
			return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeIntegrityError, false, "未知统计timeline种类")
		}
		cursor.Through = r.Change
		result.ThroughRevision = r.Change
	}
	if more {
		raw, _ := platform.CanonicalJSON(cursor)
		result.NextCursor, err = serviceauth.SignCursor(runtime.Key, scope, raw)
		if err != nil {
			return ReadUsageStatisticsPageResult{}, err
		}
	} else {
		result.ThroughRevision = cursor.Upper
	}
	payload, e := platform.CanonicalJSON(result)
	if e != nil {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeIntegrityError, false, "完整统计页不能规范编码")
	}
	if len(payload) > (2<<20)-128 {
		return ReadUsageStatisticsPageResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "完整统计页超过2MiB响应预算")
	}
	result.PageHash = platform.HashBytes(payload)
	return result, nil
}

type statisticsLineage struct {
	Record        gatewayruntime.InvocationRecord
	Attempt       gatewayruntime.AttemptRecord
	ModelHash     string
	ExecutionHash string
}

// readStatisticsLineage只核原调用/原attempt冻结材料，绝不查当前models/providers补历史。
func readStatisticsLineage(ctx context.Context, op OperationContext, tx *sql.Tx, stored storedUsageChange) (statisticsLineage, error) {
	projected, e := gatewayruntime.IsRestoredEpoch(ctx, tx, stored.SourceDataEpoch, op.DataEpoch)
	if e != nil {
		return statisticsLineage{}, e
	}
	if !projected {
		return statisticsLineage{}, platform.NewError(platform.CodeDependencyUnavailable, false, "旧费用来源缺认证target只读投影")
	}
	execution := &gatewayruntime.ExecutionTransaction{Tx: tx, Context: ctx}
	record, err := gatewayruntime.ReadStatisticsInvocationRecord(execution, op, stored.InvocationID)
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			return statisticsLineage{}, platform.NewError(platform.CodeIntegrityError, false, "统计原调用事实缺失")
		}
		return statisticsLineage{}, err
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(execution, stored.AttemptID)
	if err != nil {
		return statisticsLineage{}, err
	}
	if attempt.Attempt.InvocationId != record.Invocation.InvocationId {
		return statisticsLineage{}, platform.NewError(platform.CodeIntegrityError, false, "统计原attempt不属于原调用")
	}
	var fixed gatewayruntime.FixedRoute
	if json.Unmarshal(record.Route, &fixed) != nil {
		return statisticsLineage{}, platform.NewError(platform.CodeIntegrityError, false, "统计原冻结路线损坏")
	}
	matched := false
	for _, candidate := range fixed.Candidates {
		if candidate.Model.ModelId == attempt.Model.ModelId {
			a, _ := platform.CanonicalJSON(candidate)
			b, _ := platform.CanonicalJSON(gatewayruntime.FixedCandidate{Model: attempt.Model, Provider: attempt.Provider, CredentialVersion: attempt.CredentialVersion, Price: attempt.Price})
			matched = platform.HashBytes(a) == platform.HashBytes(b)
		}
	}
	if !matched {
		return statisticsLineage{}, platform.NewError(platform.CodeIntegrityError, false, "统计attempt冻结模型/渠道/价目与原路线不同")
	}
	hash, err := gatewayruntime.ModelSnapshotHash(attempt.Model)
	if err != nil {
		return statisticsLineage{}, err
	}
	var executionHash string
	if err = tx.QueryRowContext(ctx, `SELECT execution_hash FROM invocations WHERE invocation_id=?`, record.Invocation.InvocationId).Scan(&executionHash); err != nil {
		return statisticsLineage{}, adminStorageError(err)
	}
	return statisticsLineage{Record: record, Attempt: attempt, ModelHash: hash, ExecutionHash: executionHash}, nil
}

func validateUsageChange(stored storedUsageChange, lineage statisticsLineage, revision, version int64, observed string) (UsageLedgerChange, error) {
	record, attempt := lineage.Record, lineage.Attempt
	if stored.InvocationID != record.Invocation.InvocationId || stored.AttemptID != attempt.Attempt.AttemptId {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计原调用/attempt关联与冻结来源不同")
	}
	if revision < 1 || version < 1 || stored.SiteID != record.Operation.Actor.SiteID || stored.SourceDataEpoch != record.Operation.DataEpoch || stored.OwnerService != record.Operation.CallerService || stored.OperationID != record.Operation.OperationID || stored.RootOperationID != record.Operation.RootOperationID || stored.CostClass != record.Operation.CostClass || stored.ExecutionHash != lineage.ExecutionHash || stored.ProviderID != attempt.Provider.ProviderId || stored.ProviderVersion != attempt.Provider.Version || stored.ModelID != attempt.Model.ModelId || stored.ModelVersion != attempt.Model.Version || stored.PriceSnapshotID != attempt.Price.PriceSnapshotId {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计原安全字段与原固定执行事实不同")
	}
	if !platform.ValidIdentifier(stored.LedgerEntryID) || !slices.Contains([]string{"settled", "reconciled", "price_unknown"}, stored.LedgerStatus) || !slices.Contains([]string{"settled", "reconciled", "price_unknown", "superseded", "voided", "deleted"}, stored.State) || (stored.State != "superseded" && stored.State != "voided" && stored.State != "deleted" && stored.State != stored.LedgerStatus) || (stored.CaptureKind != "baseline" && stored.CaptureKind != "change") || stored.AmountKnown == nil || (*stored.AmountKnown != 0 && *stored.AmountKnown != 1) {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计账项身份/状态/真实版本不合法")
	}
	if stored.ModelSnapshotHash != nil && *stored.ModelSnapshotHash != lineage.ModelHash {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计模型快照hash与原冻结模型不一致")
	}
	known := *stored.AmountKnown == 1
	if known != (stored.ActualMicros != nil) || (stored.LedgerStatus == "price_unknown" && known) || (stored.ActualMicros != nil && (*stored.ActualMicros < 0 || *stored.ActualMicros > 9007199254740991)) {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计真实金额未知状态矛盾")
	}
	at, e := platform.ParseTimestamp(stored.OccurredAt)
	if e != nil {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计原账本时间损坏")
	}
	seen, e := platform.ParseTimestamp(observed)
	if e != nil {
		return UsageLedgerChange{}, platform.NewError(platform.CodeIntegrityError, false, "统计观察时刻损坏")
	}
	result := UsageLedgerChange{ChangeRevision: revision, LedgerEntryID: stored.LedgerEntryID, LedgerRevision: version, InvocationID: stored.InvocationID, AttemptID: stored.AttemptID, OperationID: stored.OperationID, RootOperationID: stored.RootOperationID, OwnerService: stored.OwnerService, SiteID: stored.SiteID, SourceDataEpoch: stored.SourceDataEpoch, CostClass: stored.CostClass, ProviderID: stored.ProviderID, ProviderVersion: stored.ProviderVersion, ModelID: stored.ModelID, ModelVersion: stored.ModelVersion, ModelSnapshotHash: lineage.ModelHash, AmountKnown: known, State: stored.State, LedgerStatus: stored.LedgerStatus, OccurredAt: at.UTC().Format(ledgerTimeLayout), ObservedAt: seen.UTC().Format(ledgerTimeLayout), CaptureKind: stored.CaptureKind}
	if known {
		result.Amount = &gen.Money{Currency: "USD", Micros: int(*stored.ActualMicros)}
	}
	return result, nil
}
