// Package migrations 提供唯一版本的本域数据结构，不复制 SQL 定义。
package migrations

import _ "embed"

// Schema 是 schemaVersion=1 的建表语句。
//
//go:embed 0001_domain.sql
var Schema string

// OriginsAndPeriods是连续V2迁移，保留V1原链与旧事实。
//
//go:embed 0002_origins_and_periods.sql
var OriginsAndPeriods string

// AggregateJobs保存不可变业务输入/提交意图与真实租约镜像，不承担领取调度。
//
//go:embed 0003_aggregate_jobs.sql
var AggregateJobs string

// QaProjection只保存问答事件的计数分类和不透明轮次引用。
//
//go:embed 0004_qa_projection.sql
var QaProjection string

// ExportJournal补齐私有CSV的固定类型/代次/原结果及安装journal。
//
//go:embed 0005_export_journal.sql
var ExportJournal string

// UsageStatisticsV2保存完整安全账项及未入账观察，V1历史不改写。
//
//go:embed 0006_usage_statistics_v2.sql
var UsageStatisticsV2 string

// Visitors 保存站内唯一访客的有限期活动。
//
//go:embed 0007_visitors.sql
var Visitors string

// SiteKeys 把页面计数和爬虫计数的主键并入 site_id。
//
//go:embed 0008_site_keys.sql
var SiteKeys string
