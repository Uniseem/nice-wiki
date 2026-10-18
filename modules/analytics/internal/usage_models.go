package analytics

// UsageLedgerChange对应model-gateway V2安全统计页；金额未知时Amount必须为空。
// 此模型只接已核验权威页，不使用invocation事件修订推导账项修订。
type UsageLedgerChange struct {
	LedgerEntryID     string `json:"ledgerEntryId"`
	InvocationID      string `json:"invocationId"`
	AttemptID         string `json:"attemptId"`
	OperationID       string `json:"operationId"`
	RootOperationID   string `json:"rootOperationId"`
	OwnerService      string `json:"ownerService"`
	SiteID            string `json:"siteId"`
	SourceDataEpoch   string `json:"sourceDataEpoch"`
	ProviderID        string `json:"providerId"`
	ModelID           string `json:"modelId"`
	ChangeRevision    int64  `json:"changeRevision"`
	LedgerRevision    int64  `json:"ledgerRevision"`
	ProviderVersion   int64  `json:"providerVersion"`
	ModelVersion      int64  `json:"modelVersion"`
	ModelSnapshotHash string `json:"modelSnapshotHash"`
	CostClass         string `json:"costClass"`
	AmountKnown       bool   `json:"amountKnown"`
	Amount            *Money `json:"amount,omitempty"`
	State             string `json:"state"`
	LedgerStatus      string `json:"ledgerStatus"`
	OccurredAt        string `json:"occurredAt"`
	ObservedAt        string `json:"observedAt"`
	CaptureKind       string `json:"captureKind"`
}

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
	AttemptCount     int64  `json:"attemptCount"`
	AllAttemptsFinal bool   `json:"allAttemptsFinal"`
}

type UsageAttemptFact struct {
	AttemptID         string `json:"attemptId"`
	State             string `json:"state"`
	ProviderID        string `json:"providerId"`
	ProviderVersion   int64  `json:"providerVersion"`
	ModelID           string `json:"modelId"`
	ModelVersion      int64  `json:"modelVersion"`
	ModelSnapshotHash string `json:"modelSnapshotHash"`
	PriceSnapshotID   string `json:"priceSnapshotId"`
	Dispatched        bool   `json:"dispatched"`
}

type UsageReservationFact struct {
	ReservationID     string `json:"reservationId"`
	AttemptID         string `json:"attemptId"`
	State             string `json:"state"`
	Version           int64  `json:"version"`
	AmountKnown       bool   `json:"amountKnown"`
	MaximumCost       *Money `json:"maximumCost,omitempty"`
	HeldCost          *Money `json:"heldCost,omitempty"`
	UnknownUsageCount int64  `json:"unknownUsageCount"`
}

// UsageObservation保留无账项的原调用、独立尝试与预留事实；不能折成零金额账项。
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

// UsageProjectionInput是参考级ProjectUsage的完整事实输入。
// DataEpoch是本次已核验页/操作的当前代次；SourceDataEpoch保留原财务事实代次，
// 已认证恢复后的历史事实可属于旧来源代次，不能改写为当前代次。
// 权威页的schema、pageHash、固定范围由读取边界核验；此输入不宣称范围已收齐。
type UsageProjectionInput struct {
	DataEpoch    string              `json:"dataEpoch"`
	Changes      []UsageLedgerChange `json:"changes"`
	Observations []UsageObservation  `json:"observations"`
}
