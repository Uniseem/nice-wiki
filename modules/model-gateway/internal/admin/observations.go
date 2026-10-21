package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"slices"
)

type UsageObservationOrigin struct {
	InvocationID    string `json:"invocationId"`
	OperationID     string `json:"operationId"`
	RootOperationID string `json:"rootOperationId"`
	OwnerService    string `json:"ownerService"`
	SiteID          string `json:"siteId"`
	SourceDataEpoch string `json:"sourceDataEpoch"`
	CostClass       string `json:"costClass"`
}
type UsageInvocationFact struct {
	State            string `json:"state"`
	CreatedAt        string `json:"createdAt"`
	Terminal         bool   `json:"terminal"`
	ActiveExecution  bool   `json:"activeExecution"`
	AttemptCount     int    `json:"attemptCount"`
	AllAttemptsFinal bool   `json:"allAttemptsFinal"`
}
type UsageAttemptFact struct {
	AttemptID         string `json:"attemptId"`
	State             string `json:"state"`
	ProviderID        string `json:"providerId"`
	ProviderVersion   int    `json:"providerVersion"`
	ModelID           string `json:"modelId"`
	ModelVersion      int    `json:"modelVersion"`
	ModelSnapshotHash string `json:"modelSnapshotHash"`
	PriceSnapshotID   string `json:"priceSnapshotId"`
	Dispatched        bool   `json:"dispatched"`
}
type UsageReservationFact struct {
	ReservationID     string     `json:"reservationId"`
	AttemptID         string     `json:"attemptId"`
	State             string     `json:"state"`
	Version           int64      `json:"version"`
	AmountKnown       bool       `json:"amountKnown"`
	MaximumCost       *gen.Money `json:"maximumCost,omitempty"`
	HeldCost          *gen.Money `json:"heldCost,omitempty"`
	UnknownUsageCount int64      `json:"unknownUsageCount"`
}
type UsageObservation struct {
	ChangeRevision int64                  `json:"changeRevision"`
	EntityRevision int64                  `json:"entityRevision"`
	Kind           string                 `json:"kind"`
	EntityID       string                 `json:"entityId"`
	Origin         UsageObservationOrigin `json:"origin"`
	Deleted        bool                   `json:"deleted"`
	ObservedAt     string                 `json:"observedAt"`
	Invocation     *UsageInvocationFact   `json:"invocation,omitempty"`
	Attempt        *UsageAttemptFact      `json:"attempt,omitempty"`
	Reservation    *UsageReservationFact  `json:"reservation,omitempty"`
}
type usageOriginEvidence struct {
	UsageObservationOrigin
	ExecutionHash string `json:"executionHash"`
}
type storedUsageObservation struct {
	Origin usageOriginEvidence `json:"origin"`
	Fact   json.RawMessage     `json:"fact"`
}

// ReadUsageObservation只给真实来源安全事实；无ledger调用、独立attempt与held金额均不补0。
func ReadUsageObservation(ctx context.Context, op OperationContext, tx *sql.Tx, kind, entity string, raw []byte, revision, version int64, deleted bool, observed string) (UsageObservation, error) {
	var stored storedUsageObservation
	if len(raw) > 16384 || json.Unmarshal(raw, &stored) != nil {
		return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "统计安全观察JSON损坏或超过16KiB")
	}
	origin := stored.Origin
	if origin.InvocationID == "" || origin.OperationID == "" || origin.RootOperationID == "" || origin.OwnerService == "" || origin.SiteID == "" || origin.SourceDataEpoch == "" || origin.CostClass == "" || origin.ExecutionHash == "" {
		return UsageObservation{}, platform.NewError(platform.CodeDependencyUnavailable, false, "原无账项观察缺完整归属/原执行证据，不能当0")
	}
	for _, id := range []string{origin.InvocationID, origin.OperationID, origin.RootOperationID, origin.OwnerService, origin.SiteID, origin.SourceDataEpoch} {
		if !platform.ValidIdentifier(id) {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "统计观察原来源身份非法")
		}
	}
	if !slices.Contains([]string{platform.CostClassQA, platform.CostClassBackground, platform.CostClassAdminProbe}, origin.CostClass) {
		return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "统计观察费用类别非法")
	}
	normalized, _ := platform.CanonicalJSON(stored)
	original, e := platform.CanonicalJSON(json.RawMessage(raw))
	if e != nil || platform.HashBytes(normalized) != platform.HashBytes(original) {
		return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "统计观察包含不能损失的信息")
	}
	record, err := gatewayruntime.ReadStatisticsInvocationRecord(&gatewayruntime.ExecutionTransaction{Tx: tx, Context: ctx}, op, origin.InvocationID)
	if err != nil {
		return UsageObservation{}, err
	}
	var hash string
	if err = tx.QueryRowContext(ctx, `SELECT execution_hash FROM invocations WHERE invocation_id=?`, origin.InvocationID).Scan(&hash); err != nil {
		return UsageObservation{}, adminStorageError(err)
	}
	if hash != origin.ExecutionHash || origin.OperationID != record.Operation.OperationID || origin.RootOperationID != record.Operation.RootOperationID || origin.OwnerService != record.Operation.CallerService || origin.SiteID != record.Operation.Actor.SiteID || origin.SourceDataEpoch != record.Operation.DataEpoch || origin.CostClass != record.Operation.CostClass || revision < 1 || version < 1 || !platform.ValidIdentifier(entity) {
		return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "安全观察原完整来源与冻结调用不同")
	}
	at, e := platform.ParseTimestamp(observed)
	if e != nil {
		return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "安全观察真实时刻损坏")
	}
	result := UsageObservation{ChangeRevision: revision, EntityRevision: version, Kind: kind, EntityID: entity, Origin: origin.UsageObservationOrigin, Deleted: deleted, ObservedAt: at.UTC().Format(ledgerTimeLayout)}
	canonicalFact := func(value any) bool {
		a, e := platform.CanonicalJSON(value)
		b, e2 := platform.CanonicalJSON(stored.Fact)
		return e == nil && e2 == nil && platform.HashBytes(a) == platform.HashBytes(b)
	}
	switch kind {
	case "invocation":
		var fact struct {
			State            string `json:"state"`
			CreatedAt        string `json:"createdAt"`
			TerminalNS       int64  `json:"terminalNS"`
			LeaseID          string `json:"leaseID"`
			LeaseUntilNS     int64  `json:"leaseUntilNS"`
			AttemptCount     *int   `json:"attemptCount"`
			AllAttemptsFinal int    `json:"allAttemptsFinal"`
		}
		if json.Unmarshal(stored.Fact, &fact) != nil || !canonicalFact(fact) || fact.AttemptCount == nil || *fact.AttemptCount < 0 || *fact.AttemptCount > 100000 || fact.AllAttemptsFinal < 0 || fact.AllAttemptsFinal > 1 || fact.TerminalNS < 0 || fact.LeaseUntilNS < 0 || ((fact.LeaseID == "") != (fact.LeaseUntilNS == 0)) || entity != origin.InvocationID || !slices.Contains([]string{"accepted", "reserved", "dispatching", "streaming", "cancel_requested", "cancelled", "failed", "succeeded", "charge_unknown", "reconciled"}, fact.State) {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "原调用观察状态/完整尝试数损坏")
		}
		created, e := platform.ParseTimestamp(fact.CreatedAt)
		if e != nil || created.UnixNano() != record.CreatedNS {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "原调用观察创建时间不同")
		}
		result.Invocation = &UsageInvocationFact{State: fact.State, CreatedAt: created.UTC().Format(ledgerTimeLayout), Terminal: fact.TerminalNS > 0, ActiveExecution: fact.LeaseID != "" || fact.LeaseUntilNS != 0, AttemptCount: *fact.AttemptCount, AllAttemptsFinal: fact.AllAttemptsFinal == 1 && (*fact.AttemptCount > 0 || fact.TerminalNS > 0)} // 还没有任何尝试的未终止调用不能算“全部尝试已终结”（空真值）
	case "attempt":
		var fact struct {
			AttemptID         string  `json:"attemptId"`
			State             string  `json:"state"`
			ProviderID        string  `json:"providerId"`
			ProviderVersion   int     `json:"providerVersion"`
			ModelID           string  `json:"modelId"`
			ModelVersion      int     `json:"modelVersion"`
			ModelSnapshotHash *string `json:"modelSnapshotHash"`
			PriceSnapshotID   string  `json:"priceSnapshotId"`
			DispatchedNS      int64   `json:"dispatchedNS"`
		}
		if json.Unmarshal(stored.Fact, &fact) != nil || !canonicalFact(fact) || entity != fact.AttemptID || !slices.Contains([]string{"reserved", "dispatching", "streaming", "failed", "succeeded", "charge_unknown", "reconciled"}, fact.State) {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "原独立尝试观察损坏")
		}
		lineage, e := readStatisticsLineage(ctx, op, tx, storedUsageChange{InvocationID: origin.InvocationID, AttemptID: fact.AttemptID, SourceDataEpoch: origin.SourceDataEpoch})
		if e != nil {
			return UsageObservation{}, e
		}
		attempt := lineage.Attempt
		if fact.ProviderID != attempt.Provider.ProviderId || fact.ProviderVersion != attempt.Provider.Version || fact.ModelID != attempt.Model.ModelId || fact.ModelVersion != attempt.Model.Version || fact.PriceSnapshotID != attempt.Price.PriceSnapshotId || (fact.ModelSnapshotHash != nil && *fact.ModelSnapshotHash != lineage.ModelHash) || fact.DispatchedNS < 0 {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "独立尝试观察不等于原冻结模型/渠道/价目")
		}
		result.Attempt = &UsageAttemptFact{AttemptID: fact.AttemptID, State: fact.State, ProviderID: fact.ProviderID, ProviderVersion: fact.ProviderVersion, ModelID: fact.ModelID, ModelVersion: fact.ModelVersion, ModelSnapshotHash: lineage.ModelHash, PriceSnapshotID: fact.PriceSnapshotID, Dispatched: fact.DispatchedNS > 0}
	case "reservation":
		var fact struct {
			ReservationID     string `json:"reservationId"`
			AttemptID         string `json:"attemptId"`
			State             string `json:"state"`
			Version           int64  `json:"version"`
			PricingKnown      int    `json:"pricingKnown"`
			MaximumMicros     *int64 `json:"maximumMicros"`
			HeldMicros        *int64 `json:"heldMicros"`
			UnknownUsageCount int64  `json:"unknownUsageCount"`
		}
		if json.Unmarshal(stored.Fact, &fact) != nil || !canonicalFact(fact) || entity != fact.ReservationID || !slices.Contains([]string{"held", "settled", "released"}, fact.State) || fact.Version < 0 || fact.Version > 9007199254740991 || fact.UnknownUsageCount < 0 || fact.UnknownUsageCount > 9007199254740991 || (fact.PricingKnown != 0 && fact.PricingKnown != 1) {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "原费用预留观察损坏")
		}
		attempt, e := gatewayruntime.ReadAttemptRecord(&gatewayruntime.ExecutionTransaction{Tx: tx, Context: ctx}, fact.AttemptID)
		if e != nil {
			return UsageObservation{}, e
		}
		if attempt.Attempt.InvocationId != origin.InvocationID || attempt.Attempt.ReservationId != fact.ReservationID {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "真实预留不属于原独立尝试")
		}
		known := fact.PricingKnown == 1
		if known != (fact.MaximumMicros != nil) || known != (fact.HeldMicros != nil) || (known && (*fact.MaximumMicros < 0 || *fact.MaximumMicros > 9007199254740991 || *fact.HeldMicros < 0 || *fact.HeldMicros > *fact.MaximumMicros)) {
			return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "真实预留金额已知/未知与原值矛盾")
		}
		value := &UsageReservationFact{ReservationID: fact.ReservationID, AttemptID: fact.AttemptID, State: fact.State, Version: fact.Version, AmountKnown: known, UnknownUsageCount: fact.UnknownUsageCount}
		if known {
			value.MaximumCost = &gen.Money{Currency: "USD", Micros: int(*fact.MaximumMicros)}
			value.HeldCost = &gen.Money{Currency: "USD", Micros: int(*fact.HeldMicros)}
		}
		result.Reservation = value
	default:
		return UsageObservation{}, platform.NewError(platform.CodeIntegrityError, false, "未知安全观察种类")
	}
	return result, nil
}
