package analytics

import (
	"nice-wiki/libs/platform"
	"nice-wiki/modules/analytics/internal/gen"
)

// 领域输入输出与 types.json 一致；HTTP 结构仍由生成代码校验。
type StatisticRow struct {
	PeriodStart platform.Timestamp  `json:"periodStart"`
	DocumentID  platform.Identifier `json:"documentId"`
	Locale      string              `json:"locale"`
	Views       int64               `json:"views"`
	Visits      int64               `json:"visits"`
}

type UsageEntry struct {
	LedgerEntryID  platform.Identifier `json:"ledgerEntryId"`
	LedgerRevision platform.Revision   `json:"ledgerRevision"`
	OperationID    platform.Identifier `json:"operationId"`
	CostClass      string              `json:"costClass"`
	Amount         Money               `json:"amount"`
	State          string              `json:"state"`
	OccurredAt     platform.Timestamp  `json:"occurredAt"`
}

type AuditRecord struct {
	SchemaVersion     int                 `json:"schemaVersion,omitempty"`
	SiteID            string              `json:"siteId,omitempty"`
	Producer          string              `json:"producer,omitempty"`
	ActorKind         string              `json:"actorKind,omitempty"`
	SourceOperationID string              `json:"sourceOperationId,omitempty"`
	OriginEpoch       string              `json:"originEpoch,omitempty"`
	Metadata          string              `json:"metadata,omitempty"`
	AuditID           platform.Identifier `json:"auditId"`
	ActorID           platform.Identifier `json:"actorId"`
	Action            string              `json:"action"`
	ResourceID        platform.Identifier `json:"resourceId"`
	ResultCode        string              `json:"resultCode"`
	OccurredAt        platform.Timestamp  `json:"occurredAt"`
	Hash              platform.Hash       `json:"hash"`
	PreviousHash      platform.Hash       `json:"previousHash"`
}

type ValidateBeaconInput struct {
	BeaconID       string `json:"beaconId"`
	DocumentID     string `json:"documentId"`
	Locale         string `json:"locale"`
	EventType      string `json:"eventType"`
	Status         int64  `json:"status"`
	ElapsedMs      int64  `json:"elapsedMs"`
	ReferrerDomain string `json:"referrerDomain"`
}

type ValidateBeaconResult struct {
	Valid bool `json:"valid"`
}

type HashDayTokenInput struct {
	PeerAddress string `json:"peerAddress"`
	UserAgent   string `json:"userAgent"`
	Day         string `json:"day"`
	SiteID      string `json:"siteId"`
}

type HashDayTokenResult struct {
	DayHash string `json:"dayHash"`
}

type IngestBeaconInput struct {
	VisitorID      string              `json:"visitorId"`
	BeaconID       string              `json:"beaconId"`
	DocumentID     string              `json:"documentId"`
	Locale         string              `json:"locale"`
	EventType      string              `json:"eventType"`
	Status         int64               `json:"status"`
	ElapsedMs      int64               `json:"elapsedMs"`
	ReferrerDomain string              `json:"referrerDomain"`
	PeerAddress    string              `json:"peerAddress"`
	UserAgent      string              `json:"userAgent"`
	DataEpoch      platform.Identifier `json:"dataEpoch"`
}

type IngestBeaconResult struct {
	Accepted bool `json:"accepted"`
}

type FlushBeaconsInput struct {
	BatchSize int64               `json:"batchSize"`
	DataEpoch platform.Identifier `json:"dataEpoch"`
}

type FlushBeaconsResult struct {
	Persisted int64 `json:"persisted"`
	Dropped   int64 `json:"dropped"`
}

type AggregateHourlyInput struct {
	Execution       *platform.JobExecutionInput `json:"-"`
	SourceWatermark int64                       `json:"-"`
	Hour            platform.Timestamp          `json:"hour"`
	DataEpoch       platform.Identifier         `json:"dataEpoch"`
}

type AggregateHourlyResult struct {
	Revision platform.Revision `json:"revision"`
	Rows     []StatisticRow    `json:"rows"`
}

type AggregateDailyInput struct {
	Execution       *platform.JobExecutionInput `json:"-"`
	SourceWatermark int64                       `json:"-"`
	Day             string                      `json:"day"`
	Timezone        string                      `json:"timezone"`
	DataEpoch       platform.Identifier         `json:"dataEpoch"`
}

type AggregateDailyResult struct {
	Revision platform.Revision `json:"revision"`
}

type ValidateDateRangeInput struct {
	From    platform.Timestamp `json:"from"`
	To      platform.Timestamp `json:"to"`
	MaxDays int64              `json:"maxDays"`
}

type ValidateDateRangeResult struct {
	FromUTC platform.Timestamp `json:"fromUTC"`
	ToUTC   platform.Timestamp `json:"toUTC"`
}

type QueryPageStatisticsInput struct {
	Granularity string               `json:"granularity,omitempty"`
	From        platform.Timestamp   `json:"from"`
	To          platform.Timestamp   `json:"to"`
	Locale      string               `json:"locale"`
	DocumentID  string               `json:"documentId"`
	Actor       platform.AuthContext `json:"actor"`
	DataEpoch   platform.Identifier  `json:"dataEpoch"`
}

type QueryPageStatisticsResult struct {
	Granularity string            `json:"granularity"`
	FromUTC     string            `json:"fromUTC"`
	ToUTC       string            `json:"toUTC"`
	Rows        []StatisticRow    `json:"rows"`
	Watermark   platform.Revision `json:"watermark"`
	DropCount   int64             `json:"dropCount"`
}

type ValidateAuditInput struct {
	VerifiedOrigin string             `json:"-"`
	Historical     bool               `json:"-"`
	AuditID        string             `json:"auditId"`
	ActorID        string             `json:"actorId"`
	Action         string             `json:"action"`
	ResourceID     string             `json:"resourceId"`
	ResultCode     string             `json:"resultCode"`
	OccurredAt     platform.Timestamp `json:"occurredAt"`
}

type ValidateAuditResult struct {
	Valid bool `json:"valid"`
}

type ComputeAuditHashInput struct {
	PreviousHash platform.Hash `json:"previousHash"`
	AuditJSON    AuditRecord   `json:"auditJSON"`
}

type ComputeAuditHashResult struct {
	Hash platform.Hash `json:"hash"`
}

type AppendAuditInput struct {
	ActorKind         *string             `json:"actorKind,omitempty"`
	OccurredAt        *string             `json:"occurredAt,omitempty"`
	SourceOperationID *string             `json:"sourceOperationId,omitempty"`
	AuditID           string              `json:"auditId"`
	ActorID           string              `json:"actorId"`
	Action            string              `json:"action"`
	ResourceID        string              `json:"resourceId"`
	ResultCode        string              `json:"resultCode"`
	Metadata          string              `json:"metadata"`
	DataEpoch         platform.Identifier `json:"dataEpoch"`
}

type AppendAuditResult struct {
	Seq  platform.Revision `json:"seq"`
	Hash platform.Hash     `json:"hash"`
}

type QueryAuditsInput struct {
	From      string               `json:"from,omitempty"`
	To        string               `json:"to,omitempty"`
	Cursor    string               `json:"cursor"`
	Limit     int64                `json:"limit"`
	Action    string               `json:"action"`
	Actor     platform.AuthContext `json:"actor"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type QueryAuditsResult struct {
	AuditIDs   []string `json:"auditIds"`
	NextCursor string   `json:"nextCursor"`
}

type ReadAuditInput struct {
	AuditID   string               `json:"auditId"`
	Actor     platform.AuthContext `json:"actor"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type ReadAuditResult struct {
	AuditJSON  AuditRecord `json:"auditJSON"`
	ChainValid bool        `json:"chainValid"`
}

type ValidateUsageEventInput struct {
	LedgerEntryID  string            `json:"ledgerEntryId"`
	LedgerRevision platform.Revision `json:"ledgerRevision"`
	OperationID    string            `json:"operationId"`
	CostClass      string            `json:"costClass"`
	AmountMicros   int64             `json:"amountMicros"`
	Currency       string            `json:"currency"`
}

type ValidateUsageEventResult struct {
	Valid bool `json:"valid"`
}

type ProjectUsageInput struct {
	LedgerEntryID  string              `json:"ledgerEntryId"`
	LedgerRevision platform.Revision   `json:"ledgerRevision"`
	OperationID    string              `json:"operationId"`
	CostClass      string              `json:"costClass"`
	AmountMicros   int64               `json:"amountMicros"`
	Currency       string              `json:"currency"`
	DataEpoch      platform.Identifier `json:"dataEpoch"`
}

type ProjectUsageResult struct {
	Applied bool `json:"applied"`
}

type AggregateUsageInput struct {
	Day       string              `json:"day"`
	DataEpoch platform.Identifier `json:"dataEpoch"`
}

type AggregateUsageResult struct {
	AmountMicros int64             `json:"amountMicros"`
	EntryCount   int64             `json:"entryCount"`
	Watermark    platform.Revision `json:"watermark"`
}

type QueryUsageStatisticsInput struct {
	From      platform.Timestamp   `json:"from"`
	To        platform.Timestamp   `json:"to"`
	CostClass string               `json:"costClass"`
	Actor     platform.AuthContext `json:"actor"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type QueryUsageStatisticsResult struct {
	AmountMicros     int64             `json:"amountMicros"`
	Entries          []UsageEntry      `json:"entries"`
	Watermark        platform.Revision `json:"watermark"`
	LastReconciledAt string            `json:"lastReconciledAt"`
}

type FetchUsagePageInput struct {
	Cursor    string              `json:"cursor"`
	Watermark platform.Revision   `json:"watermark"`
	DataEpoch platform.Identifier `json:"dataEpoch"`
}

type FetchUsagePageResult struct {
	Entries    []UsageEntry `json:"entries"`
	NextCursor string       `json:"nextCursor"`
}

type ReconcileUsageInput struct {
	From      platform.Timestamp   `json:"from"`
	To        platform.Timestamp   `json:"to"`
	Actor     platform.AuthContext `json:"actor"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type ReconcileUsageResult struct {
	Watermark     platform.Revision `json:"watermark"`
	MismatchCount int64             `json:"mismatchCount"`
}

type ConsumeAnalyticsEventInput struct {
	Event platform.EventEnvelope `json:"event"`
}

type ConsumeAnalyticsEventResult struct {
	Accepted bool `json:"accepted"`
}

type CreateExportInput struct {
	Kind      string               `json:"kind"`
	From      platform.Timestamp   `json:"from"`
	To        platform.Timestamp   `json:"to"`
	Actor     platform.AuthContext `json:"actor"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type CreateExportResult struct {
	Kind      string             `json:"kind"`
	Watermark int64              `json:"watermark"`
	ExportID  string             `json:"exportId"`
	ObjectRef platform.ObjectRef `json:"objectRef"`
	ExpiresAt platform.Timestamp `json:"expiresAt"`
}

type ValidateCSVCellInput struct {
	Value string `json:"value"`
}

type ValidateCSVCellResult struct {
	Escaped string `json:"escaped"`
}

type PurgeBeaconsInput struct {
	Now       platform.Timestamp `json:"now"`
	BatchSize int64              `json:"batchSize"`
}

type PurgeBeaconsResult struct {
	Deleted int64 `json:"deleted"`
}

type PurgeAuditHistoryInput struct {
	Now           platform.Timestamp `json:"now"`
	RetentionDays int64              `json:"retentionDays"`
}

type PurgeAuditHistoryResult struct {
	Deleted    int64  `json:"deleted"`
	AnchorHash string `json:"anchorHash"`
}

type PurgeExportsInput struct {
	Now platform.Timestamp `json:"now"`
}

type PurgeExportsResult struct {
	Deleted int64 `json:"deleted"`
}

type PublishAggregateEventsInput struct {
	BatchSize int64               `json:"batchSize"`
	DataEpoch platform.Identifier `json:"dataEpoch"`
}

type PublishAggregateEventsResult struct {
	Sent int64 `json:"sent"`
}

type ReleaseBarrierInput struct {
	BarrierID string              `json:"barrierId"`
	DataEpoch platform.Identifier `json:"dataEpoch"`
}

type ReleaseBarrierResult struct {
	Status gen.BarrierStatus `json:"status"`
}

type ClassifyCrawlerInput struct {
	UserAgent string `json:"userAgent"`
}

type ClassifyCrawlerResult struct {
	KnownCrawler  bool   `json:"knownCrawler"`
	CrawlerFamily string `json:"crawlerFamily"`
}

type LookupCountryInput struct {
	PeerAddress string `json:"peerAddress"`
}

type LookupCountryResult struct {
	CountryCode string `json:"countryCode"`
}

type RecordCrawlerAccessInput struct {
	DocumentID    string              `json:"documentId"`
	Locale        string              `json:"locale"`
	CrawlerFamily string              `json:"crawlerFamily"`
	Status        int64               `json:"status"`
	DataEpoch     platform.Identifier `json:"dataEpoch"`
}

type RecordCrawlerAccessResult struct {
	Accepted bool `json:"accepted"`
}

type QueryContentGapsInput struct {
	From      platform.Timestamp   `json:"from"`
	To        platform.Timestamp   `json:"to"`
	Actor     platform.AuthContext `json:"actor"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type QueryContentGapsResult struct {
	QaTotals            QaTotals       `json:"qaTotals"`
	PageWatermark       int64          `json:"pageWatermark"`
	QaWatermark         int64          `json:"qaWatermark"`
	UpdatedAt           string         `json:"updatedAt"`
	FromUTC             string         `json:"fromUTC"`
	ToUTC               string         `json:"toUTC"`
	HotDocuments        []StatisticRow `json:"hotDocuments"`
	UnansweredRecordIDs []string       `json:"unansweredRecordIds"`
}

type Money struct {
	Currency string `json:"currency"`
	Micros   int64  `json:"micros"`
}
