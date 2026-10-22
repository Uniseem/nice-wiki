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
	if !ok || gateway == nil || gateway.Reader == nil || gateway.ValidatePage == nil || gateway.SiteID == "" || gateway.DataEpoch == "" {
		return UsageGatewaySnapshot{}, platform.NewError(platform.CodeDependencyUnavailable, false, "费用统计只读端口或V2响应校验未装配")
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
		page, err := readUsageGatewayPage(raw, gateway.ValidatePage)
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

// readUsageGatewayPage在HTTP响应边界核对生产方schema、完整canonical摘要和本地模型无损性。
func readUsageGatewayPage(raw json.RawMessage, validate func(json.RawMessage) error) (UsageGatewayPage, error) {
	if len(raw) > usageGatewayPageBytes {
		return UsageGatewayPage{}, platform.NewError(platform.CodeBudgetExceeded, false, "费用统计页超过2MiB")
	}
	if err := validate(raw); err != nil {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计响应不符合生产方V2 schema")
	}
	var fields map[string]json.RawMessage
	var pageHash string
	if json.Unmarshal(raw, &fields) != nil || json.Unmarshal(fields["pageHash"], &pageHash) != nil || !platform.ValidHash(pageHash) {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页摘要缺失或非法")
	}
	delete(fields, "pageHash")
	canonical, err := platform.CanonicalJSON(fields)
	if err != nil || platform.HashBytes(canonical) != pageHash {
		return UsageGatewayPage{}, platform.NewError(platform.CodeIntegrityError, false, "费用统计页完整摘要不一致")
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
	last := through
	for _, change := range page.Changes {
		if change.ChangeRevision <= last ||
			change.SiteID != siteID {
			return platform.NewError(platform.CodeIntegrityError, false, "费用账项原来源、独立版本或金额状态矛盾")
		}
		if err := ValidateUsageEvent(change); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "费用账项原来源、独立版本或金额状态矛盾")
		}
		revisions = append(revisions, change.ChangeRevision)
		last = change.ChangeRevision
	}
	last = through
	for _, observation := range page.Observations {
		origin := observation.Origin
		if observation.ChangeRevision <= last ||
			origin.SiteID != siteID {
			return platform.NewError(platform.CodeIntegrityError, false, "费用观察原来源或独立版本矛盾")
		}
		switch observation.Kind {
		case "invocation":
			fact := observation.Invocation
			// 未终止的调用可以所有尝试都已结束（上一次尝试结束、尚未重试或写终态的中间快照），不算矛盾。
			if fact == nil ||
				observation.Attempt != nil ||
				observation.Reservation != nil ||
				observation.EntityID != origin.InvocationID ||
				observation.Deleted ||
				(fact.Terminal && fact.ActiveExecution) {
				return platform.NewError(platform.CodeIntegrityError, false, "费用原调用观察关联矛盾")
			}
		case "attempt":
			fact := observation.Attempt
			if fact == nil ||
				observation.Invocation != nil ||
				observation.Reservation != nil ||
				observation.EntityID != fact.AttemptID ||
				observation.Deleted {
				return platform.NewError(platform.CodeIntegrityError, false, "费用独立尝试观察关联矛盾")
			}
		case "reservation":
			fact := observation.Reservation
			if fact == nil ||
				observation.Invocation != nil ||
				observation.Attempt != nil ||
				observation.EntityID != fact.ReservationID ||
				fact.AmountKnown != (fact.MaximumCost != nil) ||
				fact.AmountKnown != (fact.HeldCost != nil) ||
				(fact.AmountKnown && (fact.MaximumCost.Currency != "USD" || fact.HeldCost.Currency != "USD" || fact.MaximumCost.Micros < 0 || fact.HeldCost.Micros < 0 || fact.HeldCost.Micros > fact.MaximumCost.Micros)) ||
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
