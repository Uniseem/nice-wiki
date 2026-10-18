package analytics

import (
	"context"
	"encoding/json"
	"net/url"
)

// UsageStatisticsPageReader由本域生成的ModelGatewayClient直接满足；只使用版本化HTTP端口。
type UsageStatisticsPageReader interface {
	ReadUsageStatisticsPage(context.Context, ...url.Values) (json.RawMessage, error)
}

// UsageGateway只由启动装配；读取边界使用本域显式V2结构与pageHash核验完整响应。
type UsageGateway struct {
	Reader    UsageStatisticsPageReader
	SiteID    string
	DataEpoch string
}

// UsageGatewayFetchInput总是从AfterRevision开始收齐固定上界，不接受半途游标作为完整结果。
type UsageGatewayFetchInput struct {
	AfterRevision int64  `json:"afterRevision"`
	Watermark     int64  `json:"watermark"`
	DataEpoch     string `json:"dataEpoch"`
}

// UsageGatewayPage是生产方V2响应的本地无损模型；pageHash只在计算自身摘要时省略。
type UsageGatewayPage struct {
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

// UsageGatewaySnapshot只在最后一页收齐后返回；Pages保留每页原hash、游标和全部事实供隔离投影。
// CompleteFeeCoverage只表示来源闭包完整，不表示金额均已知或没有in-flight负债。
type UsageGatewaySnapshot struct {
	SchemaVersion       int                `json:"schemaVersion"`
	AfterRevision       int64              `json:"afterRevision"`
	Watermark           int64              `json:"watermark"`
	ThroughRevision     int64              `json:"throughRevision"`
	Pages               []UsageGatewayPage `json:"pages"`
	FeeCoverage         string             `json:"feeCoverage"`
	CompleteFeeCoverage bool               `json:"completeFeeCoverage"`
}
