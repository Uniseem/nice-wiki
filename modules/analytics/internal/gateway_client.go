package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strconv"
	"time"

	"nice-wiki/libs/platform"
)

type usageGatewayKey struct{}

const (
	usageGatewayPageBytes     = 2 << 20
	usageGatewaySnapshotBytes = 32 << 20
)

// WithUsageGateway将可信只读端口装入本次调用链；不创建服务身份、不接收浏览器指定的网关。
func WithUsageGateway(ctx context.Context, gateway *UsageGateway) context.Context {
	return context.WithValue(ctx, usageGatewayKey{}, gateway)
}

// FetchUsagePage收齐一段固定V2上界的全部页；任一页失败均返回零值，不激活或改写投影。
func FetchUsagePage(ctx context.Context, op OperationContext, in UsageGatewayFetchInput) (UsageGatewaySnapshot, error) {
	gateway, ok := ctx.Value(usageGatewayKey{}).(*UsageGateway)
	if !ok || gateway == nil || gateway.Reader == nil || gateway.SiteID == "" || gateway.DataEpoch == "" {
		return UsageGatewaySnapshot{}, platform.NewError(platform.CodeDependencyUnavailable, false, "费用统计只读端口未装配")
	}
	if in.DataEpoch != op.DataEpoch || gateway.DataEpoch != op.DataEpoch || gateway.SiteID != op.Actor.SiteID {
		return UsageGatewaySnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "费用统计站点或当前代次不一致")
	}
	if op.Actor.Kind != "service" || op.Actor.SubjectID != "analytics" {
		return UsageGatewaySnapshot{}, platform.NewError(platform.CodeForbidden, false, "费用统计读取只接受本域纯服务操作")
	}
	if in.AfterRevision < 0 || in.Watermark < 0 || (in.Watermark != 0 && in.Watermark < in.AfterRevision) {
		return UsageGatewaySnapshot{}, platform.NewError(platform.CodeValidation, false, "费用统计固定上界早于起点")
	}
	work, cancel := context.WithTimeout(platform.WithOperationContext(ctx, op), 60*time.Second)
	defer cancel()
	result := UsageGatewaySnapshot{SchemaVersion: 2, AfterRevision: in.AfterRevision, Pages: []UsageGatewayPage{}, FeeCoverage: "posted-and-inflight"}
	upper, through, cursor := in.Watermark, in.AfterRevision, ""
	seen := map[string]bool{}
	var totalBytes int64
	for {
		query := url.Values{"afterRevision": {strconv.FormatInt(in.AfterRevision, 10)}, "limit": {"100"}}
		if upper != 0 {
			query.Set("watermark", strconv.FormatInt(upper, 10))
		}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		pageContext, pageCancel := context.WithTimeout(work, 2500*time.Millisecond)
		raw, err := gateway.Reader.ReadUsageStatisticsPage(pageContext, query)
		pageCancel()
		if err != nil {
			return UsageGatewaySnapshot{}, err
		}
		totalBytes += int64(len(raw))
		if totalBytes > usageGatewaySnapshotBytes {
			return UsageGatewaySnapshot{}, platform.NewError(platform.CodeBudgetExceeded, false, "费用统计完整范围超过32MiB原始JSON预算")
		}
		page, err := readUsageGatewayPage(raw)
		if err != nil {
			return UsageGatewaySnapshot{}, err
		}
		if len(result.Pages) == 0 && upper == 0 {
			upper = page.Watermark
		}
		if err = validateUsageGatewayPage(page, gateway.SiteID, upper, through); err != nil {
			return UsageGatewaySnapshot{}, err
		}
		if page.NextCursor != "" && (page.NextCursor == cursor || seen[page.NextCursor]) {
			return UsageGatewaySnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计分页游标重复")
		}
		result.Pages = append(result.Pages, page)
		through = page.ThroughRevision
		if page.NextCursor == "" {
			result.Watermark, result.ThroughRevision = upper, through
			result.CompleteFeeCoverage = true
			return result, nil
		}
		cursor = page.NextCursor
		seen[cursor] = true
	}
}

// readUsageGatewayPage在HTTP响应边界核对完整canonical摘要、显式V2结构和本地模型无损性。
func readUsageGatewayPage(raw json.RawMessage) (UsageGatewayPage, error) {
	if len(raw) > usageGatewayPageBytes {
		return UsageGatewayPage{}, platform.NewError(platform.CodeBudgetExceeded, false, "费用统计页超过2MiB")
	}
	var fields map[string]json.RawMessage
	var pageHash string
	if json.Unmarshal(raw, &fields) != nil || json.Unmarshal(fields["pageHash"], &pageHash) != nil || !platform.ValidHash(pageHash) {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页摘要缺失或非法")
	}
	if err := validateUsageGatewayRawShape(fields); err != nil {
		return UsageGatewayPage{}, err
	}
	delete(fields, "pageHash")
	canonical, err := platform.CanonicalJSON(fields)
	if err != nil || platform.HashBytes(canonical) != pageHash {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页完整摘要不一致")
	}
	for _, key := range []string{"schemaVersion", "watermark", "throughRevision", "changes", "observations", "nextCursor", "feeCoverage", "completeFeeCoverage", "pageHash"} {
		if _, exists := fields[key]; !exists {
			return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页缺少完整性字段")
		}
	}
	var page UsageGatewayPage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&page); err != nil {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页不能无损读取")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页包含额外JSON")
	}
	withoutHash := page
	withoutHash.PageHash = ""
	local, err := platform.CanonicalJSON(withoutHash)
	if err != nil || !bytes.Equal(local, canonical) {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页包含本地模型不能保留的事实")
	}
	return page, nil
}

// validateUsageGatewayRawShape固定生产方required字段、非null数组及未知金额省略语义。
func validateUsageGatewayRawShape(fields map[string]json.RawMessage) error {
	if err := requireUsageJSONFields(fields, "schemaVersion", "watermark", "throughRevision", "changes", "observations", "nextCursor", "feeCoverage", "completeFeeCoverage", "pageHash"); err != nil {
		return err
	}
	var changes, observations []json.RawMessage
	changesJSON := bytes.TrimSpace(fields["changes"])
	observationsJSON := bytes.TrimSpace(fields["observations"])
	if json.Unmarshal(changesJSON, &changes) != nil || len(changesJSON) == 0 || changesJSON[0] != '[' {
		return platform.NewError(platform.CodeIntegrityError, false, "费用统计changes必须为数组")
	}
	if json.Unmarshal(observationsJSON, &observations) != nil || len(observationsJSON) == 0 || observationsJSON[0] != '[' {
		return platform.NewError(platform.CodeIntegrityError, false, "费用统计observations必须为数组")
	}
	for _, raw := range changes {
		var change map[string]json.RawMessage
		if json.Unmarshal(raw, &change) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "费用统计账项不是对象")
		}
		if err := requireUsageJSONFields(change, "ledgerEntryId", "invocationId", "attemptId", "operationId", "rootOperationId", "ownerService", "siteId", "sourceDataEpoch", "providerId", "modelId", "changeRevision", "ledgerRevision", "providerVersion", "modelVersion", "modelSnapshotHash", "costClass", "amountKnown", "state", "ledgerStatus", "occurredAt", "observedAt", "captureKind"); err != nil {
			return err
		}
		amount, amountPresent := change["amount"]
		var amountKnown bool
		if json.Unmarshal(change["amountKnown"], &amountKnown) != nil || amountKnown != amountPresent || amountPresent && isJSONNull(amount) || !amountPresent && amountKnown {
			return platform.NewError(platform.CodeIntegrityError, false, "费用统计账项金额已知标志与字段不一致")
		}
	}
	for _, raw := range observations {
		var observation map[string]json.RawMessage
		if json.Unmarshal(raw, &observation) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "费用统计观察不是对象")
		}
		if err := requireUsageJSONFields(observation, "changeRevision", "entityRevision", "kind", "entityId", "origin", "deleted", "observedAt"); err != nil {
			return err
		}
		var origin map[string]json.RawMessage
		if json.Unmarshal(observation["origin"], &origin) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "费用统计观察来源不是对象")
		}
		if err := requireUsageJSONFields(origin, "invocationId", "operationId", "rootOperationId", "ownerService", "siteId", "sourceDataEpoch", "costClass"); err != nil {
			return err
		}
		var kind string
		if json.Unmarshal(observation["kind"], &kind) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "费用统计观察种类不可读")
		}
		switch kind {
		case "invocation":
			var fact map[string]json.RawMessage
			if json.Unmarshal(observation["invocation"], &fact) != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "费用调用观察不是对象")
			}
			if err := requireUsageJSONFields(fact, "state", "createdAt", "terminal", "activeExecution", "attemptCount", "allAttemptsFinal"); err != nil {
				return err
			}
		case "attempt":
			var fact map[string]json.RawMessage
			if json.Unmarshal(observation["attempt"], &fact) != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "费用尝试观察不是对象")
			}
			if err := requireUsageJSONFields(fact, "attemptId", "state", "providerId", "providerVersion", "modelId", "modelVersion", "modelSnapshotHash", "priceSnapshotId", "dispatched"); err != nil {
				return err
			}
		case "reservation":
			var fact map[string]json.RawMessage
			if json.Unmarshal(observation["reservation"], &fact) != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "费用预留观察不是对象")
			}
			if err := requireUsageJSONFields(fact, "reservationId", "attemptId", "state", "version", "amountKnown", "unknownUsageCount"); err != nil {
				return err
			}
			var known bool
			if json.Unmarshal(fact["amountKnown"], &known) != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "费用预留金额已知标志不可读")
			}
			_, maximumPresent := fact["maximumCost"]
			_, heldPresent := fact["heldCost"]
			if known != (maximumPresent && heldPresent) || known != (!maximumPresent && !heldPresent) ||
				maximumPresent && isJSONNull(fact["maximumCost"]) ||
				heldPresent && isJSONNull(fact["heldCost"]) {
				return platform.NewError(platform.CodeIntegrityError, false, "费用预留金额已知标志与字段不一致")
			}
		default:
			return platform.NewError(platform.CodeUnsupportedSchema, false, "费用观察种类未登记")
		}
	}
	return nil
}

func requireUsageJSONFields(fields map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		raw, exists := fields[name]
		if !exists || isJSONNull(raw) {
			return platform.NewError(platform.CodeIntegrityError, false, "费用统计页缺少完整性字段")
		}
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// validateUsageGatewayPage核固定时间线上界、跨数组顺序及原引用；历史sourceDataEpoch只保留，不猜当前代次。
func validateUsageGatewayPage(page UsageGatewayPage, siteID string, upper, through int64) error {
	if page.SchemaVersion != 2 || page.FeeCoverage != "posted-and-inflight" || !page.CompleteFeeCoverage || page.Changes == nil || page.Observations == nil {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "费用统计页缺完整V2来源闭包")
	}
	if page.Watermark != upper || page.ThroughRevision < through || page.ThroughRevision > upper {
		return platform.NewError(platform.CodeStaleResult, false, "费用统计固定上界或分页水位改变")
	}
	revisions := make([]int64, 0, len(page.Changes)+len(page.Observations))
	if len(page.Changes)+len(page.Observations) > 100 {
		return platform.NewError(platform.CodeIntegrityError, false, "费用统计两类事实合计超过100")
	}
	validMoney := func(value *Money) bool {
		return value != nil && value.Currency == "USD" && value.Micros >= 0 && value.Micros <= 9007199254740991
	}
	validCostClass := func(value string) bool {
		return value == platform.CostClassQA || value == platform.CostClassBackground || value == platform.CostClassAdminProbe
	}
	validTime := func(value string) bool {
		_, err := platform.ParseTimestamp(value)
		return err == nil
	}
	validChangeState := func(value string) bool {
		return value == "settled" || value == "reconciled" || value == "price_unknown" || value == "superseded" || value == "voided" || value == "deleted"
	}
	validLedgerStatus := func(value string) bool {
		return value == "settled" || value == "reconciled" || value == "price_unknown"
	}
	last := through
	for _, change := range page.Changes {
		if change.ChangeRevision <= last ||
			change.LedgerRevision < 1 ||
			!platform.ValidIdentifier(change.LedgerEntryID) ||
			!platform.ValidIdentifier(change.InvocationID) ||
			!platform.ValidIdentifier(change.AttemptID) ||
			!platform.ValidIdentifier(change.OperationID) ||
			!platform.ValidIdentifier(change.RootOperationID) ||
			!platform.ValidIdentifier(change.OwnerService) ||
			change.SiteID != siteID ||
			!platform.ValidIdentifier(change.SourceDataEpoch) ||
			!platform.ValidIdentifier(change.ProviderID) ||
			!platform.ValidIdentifier(change.ModelID) ||
			change.ProviderVersion < 1 ||
			change.ModelVersion < 1 ||
			!platform.ValidHash(change.ModelSnapshotHash) ||
			!validCostClass(change.CostClass) ||
			!validChangeState(change.State) ||
			!validLedgerStatus(change.LedgerStatus) ||
			!validTime(change.OccurredAt) ||
			!validTime(change.ObservedAt) ||
			(change.CaptureKind != "baseline" && change.CaptureKind != "change") ||
			change.AmountKnown != (change.Amount != nil) ||
			(change.AmountKnown && !validMoney(change.Amount)) ||
			(change.LedgerStatus == "price_unknown" && change.AmountKnown) ||
			(change.State != "superseded" && change.State != "voided" && change.State != "deleted" && change.State != change.LedgerStatus) {
			return platform.NewError(platform.CodeIntegrityError, false, "费用账项原来源、独立版本或金额状态矛盾")
		}
		revisions = append(revisions, change.ChangeRevision)
		last = change.ChangeRevision
	}
	last = through
	for _, observation := range page.Observations {
		origin := observation.Origin
		if observation.ChangeRevision <= last ||
			observation.EntityRevision < 1 ||
			!platform.ValidIdentifier(observation.EntityID) ||
			!validTime(observation.ObservedAt) ||
			!platform.ValidIdentifier(origin.InvocationID) ||
			!platform.ValidIdentifier(origin.OperationID) ||
			!platform.ValidIdentifier(origin.RootOperationID) ||
			!platform.ValidIdentifier(origin.OwnerService) ||
			origin.SiteID != siteID ||
			!platform.ValidIdentifier(origin.SourceDataEpoch) ||
			!validCostClass(origin.CostClass) {
			return platform.NewError(platform.CodeIntegrityError, false, "费用观察原来源或独立版本矛盾")
		}
		switch observation.Kind {
		case "invocation":
			fact := observation.Invocation
			if fact == nil ||
				observation.Attempt != nil ||
				observation.Reservation != nil ||
				observation.EntityID != origin.InvocationID ||
				observation.Deleted ||
				!validTime(fact.CreatedAt) ||
				fact.AttemptCount < 0 ||
				fact.AttemptCount > 100000 ||
				!validInvocationState(fact.State) {
				return platform.NewError(platform.CodeIntegrityError, false, "费用原调用观察关联矛盾")
			}
		case "attempt":
			fact := observation.Attempt
			if fact == nil ||
				observation.Invocation != nil ||
				observation.Reservation != nil ||
				observation.EntityID != fact.AttemptID ||
				observation.Deleted ||
				!platform.ValidIdentifier(fact.AttemptID) ||
				!validAttemptState(fact.State) ||
				!platform.ValidIdentifier(fact.ProviderID) ||
				fact.ProviderVersion < 1 ||
				!platform.ValidIdentifier(fact.ModelID) ||
				fact.ModelVersion < 1 ||
				!platform.ValidHash(fact.ModelSnapshotHash) ||
				!platform.ValidIdentifier(fact.PriceSnapshotID) {
				return platform.NewError(platform.CodeIntegrityError, false, "费用独立尝试观察关联矛盾")
			}
		case "reservation":
			fact := observation.Reservation
			if fact == nil ||
				observation.Invocation != nil ||
				observation.Attempt != nil ||
				observation.EntityID != fact.ReservationID ||
				!platform.ValidIdentifier(fact.ReservationID) ||
				!platform.ValidIdentifier(fact.AttemptID) ||
				(fact.State != "held" && fact.State != "settled" && fact.State != "released") ||
				fact.Version < 0 ||
				fact.UnknownUsageCount < 0 ||
				fact.UnknownUsageCount > 9007199254740991 ||
				fact.AmountKnown != (fact.MaximumCost != nil) ||
				fact.AmountKnown != (fact.HeldCost != nil) ||
				(fact.AmountKnown && (!validMoney(fact.MaximumCost) || !validMoney(fact.HeldCost) || fact.HeldCost.Micros > fact.MaximumCost.Micros)) ||
				(observation.Deleted && fact.State == "held") {
				return platform.NewError(platform.CodeIntegrityError, false, "费用原预留观察关联、删除或金额状态矛盾")
			}
		default:
			return platform.NewError(platform.CodeUnsupportedSchema, false, "费用观察种类未登记")
		}
		revisions = append(revisions, observation.ChangeRevision)
		last = observation.ChangeRevision
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i] < revisions[j] })
	last = through
	for _, revision := range revisions {
		if revision <= last || revision > page.ThroughRevision {
			return platform.NewError(platform.CodeIntegrityError, false, "费用两类事实重复或越过分页水位")
		}
		last = revision
	}
	if page.NextCursor == "" {
		if page.ThroughRevision != upper {
			return platform.NewError(platform.CodeIntegrityError, false, "费用末页尚未收齐固定上界")
		}
	} else if len(revisions) == 0 || last != page.ThroughRevision || page.ThroughRevision >= upper {
		return platform.NewError(platform.CodeIntegrityError, false, "费用分页游标与实际时间线进度不一致")
	}
	return nil
}

func validInvocationState(value string) bool {
	return value == "accepted" ||
		value == "reserved" ||
		value == "dispatching" ||
		value == "streaming" ||
		value == "cancel_requested" ||
		value == "cancelled" ||
		value == "failed" ||
		value == "succeeded" ||
		value == "charge_unknown" ||
		value == "reconciled"
}

func validAttemptState(value string) bool {
	return value == "reserved" ||
		value == "dispatching" ||
		value == "streaming" ||
		value == "failed" ||
		value == "succeeded" ||
		value == "charge_unknown" ||
		value == "reconciled"
}
