package jobs

import (
	"github.com/mikestefanello/backlite"
	"time"
)

type Kind string

const (
	IndexDocument     Kind = "index-document"
	TranslateDocument Kind = "translate-document"
	DescribeAsset     Kind = "describe-asset"
	FetchVideo        Kind = "fetch-video"
	UpdateSummary     Kind = "update-summary"
	UpdateCatalog     Kind = "update-catalog"
	UpdateGeoIP       Kind = "update-geoip"
	Backup            Kind = "backup"
	Cleanup           Kind = "cleanup"
	Rollup            Kind = "rollup"
	RebuildIndex      Kind = "rebuild-index"
	TranslateSite     Kind = "translate-site"
)

func Config(kind Kind) backlite.QueueConfig {
	return backlite.QueueConfig{Name: string(kind), MaxAttempts: 3, Timeout: 5 * time.Minute, Backoff: 30 * time.Second, Retention: &backlite.Retention{Duration: 7 * 24 * time.Hour, OnlyFailed: true}}
}

type Task struct {
	Kind       Kind   `json:"kind"`
	SubjectID  string `json:"subjectId"`
	Generation int64  `json:"generation"`
}

func (t Task) Config() backlite.QueueConfig { return Config(t.Kind) }
