// Package admin 只读查询已有费用事实；不生成调用、预留、账单或伪造归属。
package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type GetLedgerInput struct {
	Cursor    *string   `json:"cursor,omitempty"`
	Limit     int       `json:"limit"`
	CostClass *string   `json:"costClass,omitempty"`
	From      time.Time `json:"from"`
	Until     time.Time `json:"until"`
}
type GetLedgerResult = gen.GetLedgerResult
type QueryUsageInput struct {
	RootOperationId string    `json:"rootOperationId"`
	From            time.Time `json:"from"`
	Until           time.Time `json:"until"`
}
type QueryUsageResult = gen.QueryUsageResult

type ledgerReadRuntime struct {
	Key           platform.SigningKey
	ValidatePrice func(json.RawMessage) error
}
type ledgerRuntimeKey struct{}
type ledgerReadCursor struct {
	SchemaVersion int    `json:"schemaVersion"`
	Revision      int64  `json:"revision"`
	AfterTime     string `json:"afterTime"`
	AfterId       string `json:"afterId"`
}
type ledgerReadMetadata struct {
	Id, Owner, Site, Epoch, Root, CostClass, State string
	CreatedAt                                      time.Time
	Price                                          gen.PriceSnapshot
}

const ledgerTimeLayout = "2006-01-02T15:04:05.000000000Z"
const ledgerTimeKey = `(substr(l.created_at,1,19)||'.'||substr((CASE WHEN substr(l.created_at,20,1)='.' THEN substr(l.created_at,21,length(l.created_at)-21) ELSE '' END)||'000000000',1,9)||'Z')`

func ledgerRuntime(ctx context.Context, op OperationContext) (ledgerReadRuntime, error) {
	runtime, ok := ctx.Value(ledgerRuntimeKey{}).(ledgerReadRuntime)
	if !ok || runtime.ValidatePrice == nil {
		return runtime, platform.NewError(platform.CodeDependencyUnavailable, true, "账本读取校验依赖未装配")
	}
	return runtime, nil
}

// validateLedgerCoverage 不将旧行缺归属、孤立账单或空价目过滤成无费用。
func validateLedgerCoverage(ctx context.Context, op OperationContext, tx *sql.Tx) error {
	var missing bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_entries l LEFT JOIN invocations i ON i.invocation_id=l.invocation_id WHERE i.invocation_id IS NULL) OR EXISTS(SELECT 1 FROM reservations r LEFT JOIN invocations i ON i.invocation_id=r.invocation_id WHERE i.invocation_id IS NULL)`).Scan(&missing); err != nil {
		return adminStorageError(err)
	}
	if missing {
		return platform.NewError(platform.CodeIntegrityError, false, "账本或预留引用的调用权威行缺失")
	}
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_entries l LEFT JOIN invocations i ON i.invocation_id=l.invocation_id WHERE i.invocation_id IS NULL OR i.owner_service IS NULL OR i.owner_service='' OR i.site_id IS NULL OR i.site_id='' OR i.created_at IS NULL OR i.created_at='' OR i.data_epoch='' OR l.created_at='' OR length(i.price_snapshot_json)=0)`).Scan(&missing)
	if err != nil {
		return adminStorageError(err)
	}
	if missing {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "历史账本缺必要调用归属、时间或固定价目，不能推断为空")
	}
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations r LEFT JOIN invocations i ON i.invocation_id=r.invocation_id WHERE i.invocation_id IS NULL OR i.owner_service IS NULL OR i.owner_service='' OR i.site_id IS NULL OR i.site_id='' OR i.created_at IS NULL OR i.created_at='' OR i.data_epoch='' OR length(i.price_snapshot_json)=0)`).Scan(&missing)
	if err != nil {
		return adminStorageError(err)
	}
	if missing {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "历史预留缺必要调用归属、时间或固定价目，不能忽略其占用")
	}
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_invalid_timestamps)`).Scan(&missing)
	if err != nil {
		return adminStorageError(err)
	}
	if missing {
		return platform.NewError(platform.CodeIntegrityError, false, "账本时刻不符合UTC持久格式，不能用于分页筛选")
	}
	return nil
}

func readLedgerRevision(ctx context.Context, op OperationContext, tx *sql.Tx) (int64, error) {
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM ledger_read_state WHERE singleton=1`).Scan(&revision); errors.Is(err, sql.ErrNoRows) {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "账本分页水位行缺失")
	} else if err != nil {
		return 0, adminStorageError(err)
	}
	if revision < 1 {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "账本分页水位损坏")
	}
	return revision, nil
}

func readLedgerMetadata(ctx context.Context, op OperationContext, runtime ledgerReadRuntime, id, owner, site, epoch, root, costClass, state, created string, price []byte) (ledgerReadMetadata, error) {
	if owner == "" || site == "" || epoch == "" || created == "" || len(price) == 0 {
		return ledgerReadMetadata{}, platform.NewError(platform.CodeDependencyUnavailable, false, "调用缺必要费用读取归属或固定材料")
	}
	result := ledgerReadMetadata{Id: id, Owner: owner, Site: site, Epoch: epoch, Root: root, CostClass: costClass, State: state}
	var err error
	result.CreatedAt, err = platform.ParseTimestamp(created)
	if err != nil || !platform.ValidIdentifier(id) || !platform.ValidIdentifier(owner) || !platform.ValidIdentifier(site) || !platform.ValidIdentifier(epoch) || !platform.ValidIdentifier(root) || (costClass != platform.CostClassQA && costClass != platform.CostClassBackground && costClass != platform.CostClassAdminProbe) || !slices.Contains([]string{"accepted", "reserved", "dispatching", "streaming", "succeeded", "failed", "cancel_requested", "cancelled", "charge_unknown", "reconciled"}, state) || runtime.ValidatePrice(price) != nil || json.Unmarshal(price, &result.Price) != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "调用归属、时间、状态或固定价目损坏")
	}
	if result.Price.PricingKnown != (result.Price.InputPerMillionCost != nil && result.Price.OutputPerMillionCost != nil && result.Price.FixedCost != nil) || (!result.Price.PricingKnown && (result.Price.Warning == nil || *result.Price.Warning == "")) {
		return result, platform.NewError(platform.CodeIntegrityError, false, "调用固定价目完整率或未知提示矛盾")
	}
	return result, nil
}

// readLedgerEntries 所有SQL片段由本域构造，业务筛选值只通过参数传入。
func readLedgerEntries(ctx context.Context, op OperationContext, tx *sql.Tx, runtime ledgerReadRuntime, condition string, args []any, limit int) ([]gen.LedgerEntry, error) {
	query := `SELECT l.entry_id,l.invocation_id,l.provider_request_id,l.actual_micro_usd,l.input_tokens,l.output_tokens,l.status,l.created_at,l.attempt_id,l.pricing_known,i.owner_service,i.site_id,i.data_epoch,i.root_operation_id,i.cost_class,i.state,i.created_at,COALESCE(l.price_snapshot_json,i.price_snapshot_json) FROM ledger_entries l LEFT JOIN invocations i ON i.invocation_id=l.invocation_id WHERE ` + condition + ` ORDER BY ` + ledgerTimeKey + `,l.entry_id LIMIT ?`
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, adminStorageError(err)
	}
	defer rows.Close()
	result := make([]gen.LedgerEntry, 0)
	for rows.Next() {
		var entry gen.LedgerEntry
		var amount sql.NullInt64
		var inputTokens, outputTokens, known int64
		var created string
		var owner, site, epoch, root, costClass, state, invocationCreated sql.NullString
		var price []byte
		if err := rows.Scan(&entry.EntryId, &entry.InvocationId, &entry.ProviderRequestId, &amount, &inputTokens, &outputTokens, &entry.Status, &created, &entry.AttemptId, &known, &owner, &site, &epoch, &root, &costClass, &state, &invocationCreated, &price); err != nil {
			return nil, adminStorageError(err)
		}
		metadata, err := readLedgerMetadata(ctx, op, runtime, entry.InvocationId, owner.String, site.String, epoch.String, root.String, costClass.String, state.String, invocationCreated.String, price)
		if err != nil {
			return nil, err
		}
		if metadata.Site != op.Actor.SiteID {
			return nil, platform.NewError(platform.CodeForbidden, false, "账本条目不属于当前站点")
		}
		projected,e:=gatewayruntime.IsRestoredEpoch(ctx,tx,metadata.Epoch,op.DataEpoch);if e!=nil{return nil,e}
		if !projected {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "账本条目属于其他数据代次")
		}
		when, err := platform.ParseTimestamp(created)
		if err != nil || (known != 0 && known != 1) || amount.Valid != (known == 1) || (amount.Valid && (amount.Int64 < 0 || amount.Int64 > 9007199254740991)) || inputTokens < 0 || inputTokens > 9007199254740991 || outputTokens < 0 || outputTokens > 9007199254740991 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "账本金额、用量或时间材料矛盾")
		}
		entry.CreatedAt = gen.Timestamp(when)
		entry.InputTokens = int(inputTokens)
		entry.OutputTokens = int(outputTokens)
		entry.PricingKnown = known == 1
		if amount.Valid {
			entry.ActualCost = &gen.Money{Currency: "USD", Micros: int(amount.Int64)}
		}
		entry.PriceSnapshot = metadata.Price
		if !platform.ValidIdentifier(entry.EntryId) || !platform.ValidIdentifier(entry.AttemptId) || !utf8.ValidString(entry.ProviderRequestId) || utf8.RuneCountInString(entry.ProviderRequestId) < 1 || utf8.RuneCountInString(entry.ProviderRequestId) > 65536 || !slices.Contains([]string{"settled", "reconciled", "price_unknown"}, string(entry.Status)) || (string(entry.Status) == "price_unknown" && entry.PricingKnown) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "账本条目身份或价目状态矛盾")
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, adminStorageError(err)
	}
	return result, nil
}

func readLedgerPage(ctx context.Context, op OperationContext, input GetLedgerInput) (GetLedgerResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return GetLedgerResult{}, platform.NewError(platform.CodeForbidden, false, "账本列表仅允许当前后台管理账户")
	}
	if !input.From.Before(input.Until) || input.Until.Sub(input.From) > 90*24*time.Hour {
		return GetLedgerResult{}, platform.NewError(platform.CodeValidation, false, "账本时间窗必须大于零且不超过90天")
	}
	runtime, err := ledgerRuntime(ctx, op)
	if err != nil {
		return GetLedgerResult{}, err
	}
	binding, _ := platform.CanonicalJSON(struct {
		Caller, Actor, Site, Epoch, Session, From, Until string
		SessionRevision, PermissionRevision              int64
		Limit                                            int
		CostClass                                        *string
	}{op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.Actor.SessionID, input.From.UTC().Format(ledgerTimeLayout), input.Until.UTC().Format(ledgerTimeLayout), op.Actor.SessionRevision, op.Actor.PermissionRevision, input.Limit, input.CostClass})
	scope := "model-gateway.ledger.v1:" + platform.HashBytes(binding)
	position := ledgerReadCursor{SchemaVersion: 1}
	if input.Cursor != nil {
		raw, err := serviceauth.VerifyCursor(runtime.Key, scope, *input.Cursor)
		if err != nil {
			return GetLedgerResult{}, err
		}
		if json.Unmarshal(raw, &position) != nil {
			return GetLedgerResult{}, platform.NewError(platform.CodeValidation, false, "账本游标不可读")
		}
		canonical, _ := platform.CanonicalJSON(position)
		when, err := time.Parse(ledgerTimeLayout, position.AfterTime)
		if err != nil || when.UTC().Format(ledgerTimeLayout) != position.AfterTime || string(canonical) != string(raw) || position.SchemaVersion != 1 || position.Revision < 1 || !platform.ValidIdentifier(position.AfterId) {
			return GetLedgerResult{}, platform.NewError(platform.CodeValidation, false, "账本游标材料不合法")
		}
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return GetLedgerResult{}, err
	}
	defer tx.Rollback()
	if err := validateLedgerCoverage(ctx, op, tx); err != nil {
		return GetLedgerResult{}, err
	}
	revision, err := readLedgerRevision(ctx, op, tx)
	if err != nil {
		return GetLedgerResult{}, err
	}
	if input.Cursor != nil && revision != position.Revision {
		return GetLedgerResult{}, platform.NewError(platform.CodeVersionMismatch, true, "账本水位已变化，请从第一页重读")
	}
	condition := `i.site_id=? AND ` + ledgerTimeKey + `>=? AND ` + ledgerTimeKey + `<?`
	args := []any{op.Actor.SiteID, input.From.UTC().Format(ledgerTimeLayout), input.Until.UTC().Format(ledgerTimeLayout)}
	if input.CostClass != nil {
		condition += ` AND i.cost_class=?`
		args = append(args, *input.CostClass)
	}
	if input.Cursor != nil {
		condition += ` AND (` + ledgerTimeKey + `,l.entry_id)>(?,?)`
		args = append(args, position.AfterTime, position.AfterId)
	}
	entries, err := readLedgerEntries(ctx, op, tx, runtime, condition, args, input.Limit+1)
	if err != nil {
		return GetLedgerResult{}, err
	}
	if err := tx.Rollback(); err != nil {
		return GetLedgerResult{}, adminStorageError(err)
	}
	result := GetLedgerResult{Entries: entries}
	if len(entries) > input.Limit {
		result.Entries = entries[:input.Limit]
		last := result.Entries[len(result.Entries)-1]
		raw, _ := platform.CanonicalJSON(ledgerReadCursor{1, revision, time.Time(last.CreatedAt).UTC().Format(ledgerTimeLayout), last.EntryId})
		cursor, err := serviceauth.SignCursor(runtime.Key, scope, raw)
		if err != nil {
			return GetLedgerResult{}, err
		}
		result.NextCursor = &cursor
	}
	return result, nil
}

func readRootMetadata(ctx context.Context, op OperationContext, tx *sql.Tx, runtime ledgerReadRuntime, root string) ([]ledgerReadMetadata, error) {
	rows, err := tx.QueryContext(ctx, `SELECT invocation_id,owner_service,site_id,data_epoch,root_operation_id,cost_class,state,created_at,price_snapshot_json FROM invocations WHERE root_operation_id=? ORDER BY invocation_id LIMIT 257`, root)
	if err != nil {
		return nil, adminStorageError(err)
	}
	defer rows.Close()
	result := make([]ledgerReadMetadata, 0)
	for rows.Next() {
		var id, epoch, recordRoot, costClass, state string
		var owner, site, created sql.NullString
		var price []byte
		if err := rows.Scan(&id, &owner, &site, &epoch, &recordRoot, &costClass, &state, &created, &price); err != nil {
			return nil, adminStorageError(err)
		}
		metadata, err := readLedgerMetadata(ctx, op, runtime, id, owner.String, site.String, epoch, recordRoot, costClass, state, created.String, price)
		if err != nil {
			return nil, err
		}
		if metadata.Site != op.Actor.SiteID {
			return nil, platform.NewError(platform.CodeForbidden, false, "费用根不属于当前站点")
		}
		projected,e:=gatewayruntime.IsRestoredEpoch(ctx,tx,metadata.Epoch,op.DataEpoch);if e!=nil{return nil,e}
		if !projected {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "费用根含其他数据代次")
		}
		if op.Actor.Kind == "service" && metadata.Owner != op.CallerService && !(op.CallerService == "analytics" && op.Actor.SubjectID == "analytics") {
			return nil, platform.NewError(platform.CodeForbidden, false, "费用根不完全属于调用服务")
		}
		result = append(result, metadata)
		if len(result) > 256 {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "费用根完整调用集合超过响应预算")
		}
	}
	if err := rows.Err(); err != nil {
		return nil, adminStorageError(err)
	}
	if len(result) == 0 {
		return nil, platform.NewError(platform.CodeNotFound, false, "费用根尚无可确认的持久归属")
	}
	return result, nil
}

func sumLedgerMoney(ctx context.Context, op OperationContext, total *big.Int) (*gen.Money, error) {
	if total.Sign() < 0 || total.Cmp(big.NewInt(9007199254740991)) > 0 {
		return nil, platform.NewError(platform.CodeBudgetExceeded, false, "完整费用总额超出可表示整数范围")
	}
	return &gen.Money{Currency: "USD", Micros: int(total.Int64())}, nil
}

// QueryUsage 当前held与窗口内已结算金额分组；任一未知时省略完整相应总额。
func QueryUsage(ctx context.Context, op OperationContext, input QueryUsageInput) (QueryUsageResult, error) {
	admin := op.CallerService == "portal" && op.Actor.Kind == "admin"
	service := op.Actor.Kind == "service" && op.Actor.SubjectID == op.CallerService
	owner := false
	for _, task := range gen.AITaskCatalog {
		if task.Owner == op.CallerService {
			owner = true
		}
	}
	statistics := service && op.CallerService == "analytics"
	if !admin && !(service && owner) && !statistics {
		return QueryUsageResult{}, platform.NewError(platform.CodeForbidden, false, "费用状态只允许所属任务服务或当前后台管理账户")
	}
	if !input.From.Before(input.Until) {
		return QueryUsageResult{}, platform.NewError(platform.CodeValidation, false, "费用查询时间窗必须大于零")
	}
	runtime, err := ledgerRuntime(ctx, op)
	if err != nil {
		return QueryUsageResult{}, err
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return QueryUsageResult{}, err
	}
	defer tx.Rollback()
	if err := validateLedgerCoverage(ctx, op, tx); err != nil {
		return QueryUsageResult{}, err
	}
	metadata, err := readRootMetadata(ctx, op, tx, runtime, input.RootOperationId)
	if err != nil {
		return QueryUsageResult{}, err
	}
	unknown := map[string]bool{}
	unknownState := map[string]bool{}
	evidence := map[string]bool{}
	inWindow := map[string]bool{}
	safeToRetry := true
 windowCoversRoot,allInvocationsFinal:=true,true
	for _, record := range metadata {
  if record.CreatedAt.Before(input.From)||!record.CreatedAt.Before(input.Until){windowCoversRoot=false}
		var safeRaw []byte
		var safeHash, initialized sql.NullString
		var dispatched, attemptUnknown bool
		if err := tx.QueryRowContext(ctx, `SELECT invocation_json,invocation_hash,execution_hash,EXISTS(SELECT 1 FROM invocation_attempts a WHERE a.invocation_id=i.invocation_id AND a.dispatched_ns>0),EXISTS(SELECT 1 FROM invocation_attempts a WHERE a.invocation_id=i.invocation_id AND json_extract(a.attempt_json,'$.state')='charge_unknown') FROM invocations i WHERE invocation_id=?`, record.Id).Scan(&safeRaw, &safeHash, &initialized, &dispatched, &attemptUnknown); err != nil {
			return QueryUsageResult{}, adminStorageError(err)
		}
		if len(safeRaw) == 0 || !safeHash.Valid || !initialized.Valid || initialized.String == "" {
			safeToRetry = false
   allInvocationsFinal=false
		} else {
			var snapshot gen.Invocation
			if platform.HashBytes(safeRaw) != safeHash.String || json.Unmarshal(safeRaw, &snapshot) != nil || snapshot.InvocationId != record.Id || snapshot.RootOperationId != input.RootOperationId || string(snapshot.State) != record.State {
				return QueryUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "原root调用安全状态快照损坏")
			}
			proofOp := op
			proofOp.RootOperationID = input.RootOperationId
			proofOp.CostClass = record.CostClass
			execution := &gatewayruntime.ExecutionTransaction{Tx: tx, Context: ctx}
			var proof gatewayruntime.InvocationRecord
			var err error
			if statistics {
				proof, err = gatewayruntime.ReadStatisticsInvocationRecord(execution, op, record.Id)
			} else {
				proof, err = gatewayruntime.ReadInvocationRecord(execution, proofOp, record.Id)
			}
			if err != nil {
				return QueryUsageResult{}, err
			}
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM invocation_attempts WHERE invocation_id=?`, record.Id).Scan(&count); err != nil {
				return QueryUsageResult{}, adminStorageError(err)
			}
			if count != len(proof.Invocation.Attempts) {
				return QueryUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "原root尝试列表与持久证据不完整")
			}
   if proof.TerminalNS<=0||proof.LeaseID!=""||proof.LeaseUntilNS!=0||!slices.Contains([]string{"succeeded","failed","cancelled","reconciled"},string(proof.Invocation.State)){allInvocationsFinal=false}
   seenAttempts:=map[string]bool{}
			for _, item := range proof.Invocation.Attempts {
				stored, err := gatewayruntime.ReadAttemptRecord(execution, item.AttemptId)
				if err != nil {
					return QueryUsageResult{}, err
				}
    actual,_:=platform.CanonicalJSON(stored.Attempt);listed,_:=platform.CanonicalJSON(item)
    if seenAttempts[item.AttemptId]||stored.Attempt.InvocationId!=record.Id||platform.HashBytes(actual)!=platform.HashBytes(listed){return QueryUsageResult{},platform.NewError(platform.CodeIntegrityError,false,"原root尝试完整身份列表与持久事实矛盾")};seenAttempts[item.AttemptId]=true
    if !slices.Contains([]string{"succeeded","failed","reconciled"},string(stored.Attempt.State)){allInvocationsFinal=false}
				if stored.DispatchedNS > 0 {
					safeToRetry = false
				}
			}
		}
		if dispatched || attemptUnknown || (record.State != "accepted" && record.State != "reserved") {
			safeToRetry = false
		}
		if attemptUnknown && !record.CreatedAt.Before(input.From) && record.CreatedAt.Before(input.Until) {
			unknown[record.Id] = true
			unknownState[record.Id] = true
		}
	}
	for _, record := range metadata {
		if !record.CreatedAt.Before(input.From) && record.CreatedAt.Before(input.Until) {
			inWindow[record.Id] = true
			if record.State == "charge_unknown" {
				unknown[record.Id] = true
				unknownState[record.Id] = true
			}
		}
	}
 var outside bool
 if err:=tx.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM ledger_entries l JOIN invocations i ON i.invocation_id=l.invocation_id WHERE i.root_operation_id=? AND (`+ledgerTimeKey+`<? OR `+ledgerTimeKey+`>=?))`,input.RootOperationId,input.From.UTC().Format(ledgerTimeLayout),input.Until.UTC().Format(ledgerTimeLayout)).Scan(&outside);err!=nil{return QueryUsageResult{},adminStorageError(err)};if outside{windowCoversRoot=false}
	entries, err := readLedgerEntries(ctx, op, tx, runtime, `i.root_operation_id=? AND NOT EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id) AND NOT EXISTS(SELECT 1 FROM ledger_voids v WHERE v.entry_id=l.entry_id) AND `+ledgerTimeKey+`>=? AND `+ledgerTimeKey+`<?`, []any{input.RootOperationId, input.From.UTC().Format(ledgerTimeLayout), input.Until.UTC().Format(ledgerTimeLayout)}, 257)
	if err != nil {
		return QueryUsageResult{}, err
	}
	if len(entries) > 256 {
		return QueryUsageResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "费用根完整账本超过响应预算")
	}
	settled := big.NewInt(0)
	settledKnown := true
	for _, entry := range entries {
		if entry.PricingKnown {
			settled.Add(settled, big.NewInt(int64(entry.ActualCost.Micros)))
		} else {
			settledKnown = false
			unknown[entry.InvocationId] = true
			evidence[entry.InvocationId] = true
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.invocation_id,r.state,r.maximum_micro_usd,r.held_micro_usd,r.pricing_known,r.version,r.unknown_usage_count FROM reservations r JOIN invocations i ON i.invocation_id=r.invocation_id WHERE i.root_operation_id=?`, input.RootOperationId)
	if err != nil {
		return QueryUsageResult{}, adminStorageError(err)
	}
	held := big.NewInt(0)
	heldKnown := true
	for rows.Next() {
		var id, state string
		var maximum, amount sql.NullInt64
		var known, version, count int64
		if err := rows.Scan(&id, &state, &maximum, &amount, &known, &version, &count); err != nil {
			rows.Close()
			return QueryUsageResult{}, adminStorageError(err)
		}
		if !slices.Contains([]string{"held", "settled", "released"}, state) {
			rows.Close()
			return QueryUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "费用根预留状态不在合同三态")
		}
		if state != "held" {
			continue
		}
		if (known != 0 && known != 1) || maximum.Valid != (known == 1) || amount.Valid != (known == 1) || (amount.Valid && (amount.Int64 < 0 || amount.Int64 > 9007199254740991 || maximum.Int64 < amount.Int64 || maximum.Int64 > 9007199254740991)) || version < 0 || version > 9007199254740991 || count < 0 || count > 9007199254740991 {
			rows.Close()
			return QueryUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "预留金额、价目状态或版本矛盾")
		}
		if !inWindow[id] {
			continue
		}
		evidence[id] = true
		if known == 1 {
			held.Add(held, big.NewInt(amount.Int64))
		} else {
			heldKnown = false
			unknown[id] = true
		}
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return QueryUsageResult{}, adminStorageError(rowErr)
	}
	for id := range unknownState {
		if !evidence[id] {
			return QueryUsageResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "收费未知调用缺必要held或未知账单证据，不能确认为无占用")
		}
	}
	if len(unknown) > 256 {
		return QueryUsageResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "完整未知调用集合超过响应预算")
	}
	result := QueryUsageResult{Entries: entries, UnknownInvocationIds: make([]string, 0, len(unknown)), SafeToRetry: &safeToRetry,WindowCoversRoot:&windowCoversRoot,AllInvocationsFinal:&allInvocationsFinal}
	for id := range unknown {
		result.UnknownInvocationIds = append(result.UnknownInvocationIds, id)
	}
	slices.Sort(result.UnknownInvocationIds)
	if heldKnown {
		result.HeldCost, err = sumLedgerMoney(ctx, op, held)
		if err != nil {
			return QueryUsageResult{}, err
		}
	}
	if settledKnown {
		result.SettledCost, err = sumLedgerMoney(ctx, op, settled)
		if err != nil {
			return QueryUsageResult{}, err
		}
	}
	return result, nil
}

// NewLedgerHandlers 将真实只读HTTP与schema/签名依赖装配，鉴权只在公共入口一次。
func NewLedgerHandlers(key platform.SigningKey) (map[string]http.Handler, error) {
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "账本内嵌规格不可读")
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "账本内嵌规格不可编码")
	}
	runtime := ledgerReadRuntime{Key: key}
	runtime.ValidatePrice, err = apivalidate.NewPayloadValidator(raw, "PriceSnapshot")
	if err != nil {
		return nil, err
	}
	handlers := map[string]http.Handler{}
	for _, operation := range []string{"getLedger", "queryUsage", "readUsageStatisticsPage"} {
		handlers[operation] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			query := r.URL.Query()
   if operation=="readUsageStatisticsPage"{after,_:=strconv.ParseInt(query.Get("afterRevision"),10,64);watermark,_:=strconv.ParseInt(query.Get("watermark"),10,64);limit,_:=strconv.Atoi(query.Get("limit"));ctx:=context.WithValue(r.Context(),ledgerRuntimeKey{},runtime);result,err:=ReadUsageStatisticsPage(ctx,op,ReadUsageStatisticsPageInput{AfterRevision:after,Watermark:watermark,Cursor:query.Get("cursor"),Limit:limit});if err!=nil{_ = problem.WriteProblem(w,problem.FromError(err,op.RequestID));return};w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(result);return}
			from, fromErr := platform.ParseTimestamp(query.Get("from"))
			until, untilErr := platform.ParseTimestamp(query.Get("until"))
			if fromErr != nil || untilErr != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "费用查询时间不可读"), op.RequestID))
				return
			}
			ctx := context.WithValue(r.Context(), ledgerRuntimeKey{}, runtime)
			var result any
			var err error
			if operation == "getLedger" {
				input := GetLedgerInput{Limit: 20, From: from, Until: until}
				if query.Has("limit") {
					value, _ := strconv.ParseInt(query.Get("limit"), 0, 64)
					input.Limit = int(value)
				}
				if query.Has("cursor") {
					value := query.Get("cursor")
					input.Cursor = &value
				}
				if query.Has("costClass") {
					value := query.Get("costClass")
					input.CostClass = &value
				}
				result, err = GetLedger(ctx, op, input)
			} else {
				result, err = QueryUsage(ctx, op, QueryUsageInput{RootOperationId: query.Get("rootOperationId"), From: from, Until: until})
			}
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		})
	}
	return handlers, nil
}
