// Package billing 提供免费价目读取与整数报价，不执行模型或修改费用状态。
package billing

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type OperationContext = platform.OperationContext
type PriceSnapshot = gen.PriceSnapshot
type QuoteCostInput = gen.QuoteCostInput
type QuoteCostResult = gen.QuoteCostResult
type ManualPrice struct {
	InputPerMillionCost  *gen.Money `json:"inputPerMillionCost,omitempty"`
	OutputPerMillionCost *gen.Money `json:"outputPerMillionCost,omitempty"`
	FixedCost            *gen.Money `json:"fixedCost,omitempty"`
}

const priceWarning = "该模型价格不完整，调用花费无法完整计入金额上限。"

// MakePriceSnapshot 缺任一费率为unknown，显式零与省略有不同含义。
func MakePriceSnapshot(ctx context.Context, op OperationContext, modelID string, modelVersion int, source string, effectiveAt time.Time, price ManualPrice) (PriceSnapshot, error) {
	snapshot := PriceSnapshot{ModelId: modelID, Currency: gen.USD, EffectiveAt: gen.Timestamp(effectiveAt.UTC()), InputPerMillionCost: price.InputPerMillionCost, OutputPerMillionCost: price.OutputPerMillionCost, FixedCost: price.FixedCost, PricingKnown: price.InputPerMillionCost != nil && price.OutputPerMillionCost != nil && price.FixedCost != nil}
	for _, money := range []*gen.Money{price.InputPerMillionCost, price.OutputPerMillionCost, price.FixedCost} {
		if money != nil && (money.Currency != "USD" || money.Micros < 0 || money.Micros > 9007199254740991) {
			return PriceSnapshot{}, platform.NewError(platform.CodeValidation, false, "价目必须为非负USD整数微额")
		}
	}
	if !snapshot.PricingKnown {
		warning := priceWarning
		snapshot.Warning = &warning
	}
	raw, err := platform.CanonicalJSON(struct {
		Format   string        `json:"format"`
		Epoch    string        `json:"dataEpoch"`
		Source   string        `json:"source"`
		Version  int           `json:"modelVersion"`
		Snapshot PriceSnapshot `json:"snapshot"`
	}{"price-snapshot-v1", op.DataEpoch, source, modelVersion, snapshot})
	if err != nil {
		return PriceSnapshot{}, platform.NewError(platform.CodeInternal, false, "价目身份编码失败")
	}
	snapshot.PriceSnapshotId = platform.HashBytes(raw)
	return snapshot, nil
}

// SaveManualPrice 仅从真实模型CAS事务追加人工价目，不改历史行。
func SaveManualPrice(ctx context.Context, op OperationContext, tx *sql.Tx, modelID string, modelVersion int, price ManualPrice) error {
	effectiveAt := time.Now().UTC()
	snapshot, err := MakePriceSnapshot(ctx, op, modelID, modelVersion, "manual", effectiveAt, price)
	if err != nil {
		return err
	}
	raw, err := platform.CanonicalJSON(snapshot)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "人工价目编码失败")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO price_book_entries(price_snapshot_id,model_id,model_version,data_epoch,source,effective_seconds,effective_nanos,snapshot_json,snapshot_hash) VALUES(?,?,?,?,?,?,?,?,?)`, snapshot.PriceSnapshotId, modelID, modelVersion, op.DataEpoch, "manual", effectiveAt.Unix(), effectiveAt.Nanosecond(), string(raw), platform.HashBytes(raw))
	return platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "保存人工价目失败")
}

func beginPriceSnapshot(ctx context.Context, op OperationContext) (*sql.Tx, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "价目私有存储未装配")
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	var epoch string
	if err = tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton=1`).Scan(&epoch); err != nil {
		tx.Rollback()
		return nil, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "价目权威代次不可读")
	}
	if epoch == "" {
		tx.Rollback()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "价目权威代次未知")
	}
	if epoch != op.DataEpoch {
		tx.Rollback()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "价目权威代次已变化")
	}
	return tx, nil
}

// ReadPriceSnapshot 每次共用一个SQL快照；未知不当作零价格或已登记历史。
func ReadPriceSnapshot(ctx context.Context, op OperationContext, modelID string, effectiveAt time.Time, allowUnknown bool) (PriceSnapshot, error) {
	tx, err := beginPriceSnapshot(ctx, op)
	if err != nil {
		return PriceSnapshot{}, err
	}
	defer tx.Rollback()
	var epoch, updated string
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT data_epoch,version,updated_at FROM models WHERE model_id=?`, modelID).Scan(&epoch, &version, &updated); errors.Is(err, sql.ErrNoRows) {
		return PriceSnapshot{}, platform.NewError(platform.CodeNotFound, false, "报价模型不存在")
	} else if err != nil {
		return PriceSnapshot{}, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "读取报价模型失败")
	}
	if epoch != op.DataEpoch {
		return PriceSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "报价模型代次已变化")
	}
	var otherEpoch bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM price_book_entries WHERE model_id=? AND data_epoch<>?)`, modelID, op.DataEpoch).Scan(&otherEpoch); err != nil {
		return PriceSnapshot{}, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "核对价目代次失败")
	}
	if otherEpoch {
		return PriceSnapshot{}, platform.NewError(platform.CodeVersionMismatch, false, "价目历史含其他数据代次")
	}
	var raw, digest, id, source string
	var priceVersion int
	var seconds int64
	var nanos int
	err = tx.QueryRowContext(ctx, `SELECT price_snapshot_id,model_version,source,effective_seconds,effective_nanos,snapshot_json,snapshot_hash FROM price_book_entries WHERE model_id=? AND data_epoch=? AND (effective_seconds<? OR (effective_seconds=? AND effective_nanos<=?)) ORDER BY CASE source WHEN 'manual' THEN 0 WHEN 'remote' THEN 1 ELSE 2 END,effective_seconds DESC,effective_nanos DESC,model_version DESC LIMIT 1`, modelID, op.DataEpoch, effectiveAt.Unix(), effectiveAt.Unix(), effectiveAt.Nanosecond()).Scan(&id, &priceVersion, &source, &seconds, &nanos, &raw, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		if !allowUnknown {
			return PriceSnapshot{}, platform.NewError(platform.CodeNotFound, false, "指定时刻尚无已登记价目")
		}
		changed, parseErr := platform.ParseTimestamp(updated)
		if parseErr != nil || version < 1 || version > 9007199254740991 {
			return PriceSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "未知价目缺有效模型版本或持久时刻")
		}
		return MakePriceSnapshot(ctx, op, modelID, version, "unconfigured", changed, ManualPrice{})
	}
	if err != nil {
		return PriceSnapshot{}, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "读取不可变价目失败")
	}
	var stored PriceSnapshot
	if len(raw) > 1<<20 || platform.HashBytes([]byte(raw)) != digest || json.Unmarshal([]byte(raw), &stored) != nil || priceVersion < 1 || priceVersion > version {
		return PriceSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "不可变价目材料损坏")
	}
	expected, err := MakePriceSnapshot(ctx, op, modelID, priceVersion, source, time.Unix(seconds, int64(nanos)), ManualPrice{stored.InputPerMillionCost, stored.OutputPerMillionCost, stored.FixedCost})
	encoded, encodeErr := platform.CanonicalJSON(expected)
	if err != nil || encodeErr != nil || id != expected.PriceSnapshotId || !bytes.Equal([]byte(raw), encoded) {
		return PriceSnapshot{}, platform.NewError(platform.CodeIntegrityError, false, "不可变价目身份、时刻或完整率矛盾")
	}
	return expected, nil
}

func RequirePriceCaller(ctx context.Context, op OperationContext) error {
	if op.Actor.Kind == "service" && op.Actor.SubjectID == op.CallerService {
		for _, task := range gen.AITaskCatalog {
			if task.Owner == op.CallerService {
				return nil
			}
		}
	}
	return platform.NewError(platform.CodeForbidden, false, "免费报价只允许AI任务所属服务")
}

// QuoteCost 不写报价缓存、额度或账本，缺价不伪造零。
func QuoteCost(ctx context.Context, op OperationContext, input QuoteCostInput) (QuoteCostResult, error) {
	if err := RequirePriceCaller(ctx, op); err != nil {
		return QuoteCostResult{}, err
	}
	snapshot, err := ReadPriceSnapshot(ctx, op, input.ModelId, time.Now().UTC(), true)
	if err != nil {
		return QuoteCostResult{}, err
	}
	result := QuoteCostResult{PriceSnapshot: snapshot, PricingKnown: snapshot.PricingKnown, Warning: snapshot.Warning}
	if !snapshot.PricingKnown {
		return result, nil
	}
	total := big.NewInt(int64(snapshot.FixedCost.Micros))
	for _, component := range []struct{ tokens, rate int }{{input.InputTokens, snapshot.InputPerMillionCost.Micros}, {input.MaximumOutputTokens, snapshot.OutputPerMillionCost.Micros}} {
		amount := new(big.Int).Mul(big.NewInt(int64(component.tokens)), big.NewInt(int64(component.rate)))
		amount.Add(amount, big.NewInt(999999))
		amount.Quo(amount, big.NewInt(1000000))
		total.Add(total, amount)
	}
	if total.Cmp(big.NewInt(9007199254740991)) > 0 {
		return QuoteCostResult{}, platform.NewError(platform.CodeValidation, false, "保守报价超出可表示整数范围")
	}
	result.MaximumCost = &gen.Money{Currency: "USD", Micros: int(total.Int64())}
	return result, nil
}

func NewPriceHandlers() map[string]http.Handler {
	return map[string]http.Handler{"quoteCost": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		op, _ := platform.OperationContextFrom(r.Context())
		var input QuoteCostInput
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "报价输入不可读"), op.RequestID))
			return
		}
		result, err := QuoteCost(r.Context(), op, input)
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(result)
	})}
}
