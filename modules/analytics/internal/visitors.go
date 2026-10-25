package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

type ListVisitorsInput struct {
	From, To, Cursor, VisitorID string
	Limit                       int
}
type VisitorSummary struct {
	VisitorID  string `json:"visitorId"`
	IP         string `json:"ip"`
	UserAgent  string `json:"userAgent"`
	Device     string `json:"device"`
	Browser    string `json:"browser"`
	OS         string `json:"os"`
	Country    string `json:"country"`
	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
	PageViews  int64  `json:"pageViews"`
	Activities int64  `json:"activities"`
}
type VisitorActivity struct {
	ID             string `json:"id"`
	DocumentID     string `json:"documentId"`
	Locale         string `json:"locale"`
	EventType      string `json:"eventType"`
	IP             string `json:"ip"`
	UserAgent      string `json:"userAgent"`
	OccurredAt     string `json:"occurredAt"`
	ReferrerDomain string `json:"referrerDomain"`
}
type VisitorPage struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
}
type ListVisitorsResult struct {
	Visitors      []VisitorSummary  `json:"visitors"`
	Activities    []VisitorActivity `json:"activities"`
	TotalVisitors int64             `json:"totalVisitors"`
	PageViews     int64             `json:"pageViews"`
	PageInfo      VisitorPage       `json:"pageInfo"`
}

// ListVisitors 在同一快照按随机访客ID去重，跨页面和跨天均不叠加独立访客数。
func ListVisitors(ctx context.Context, op OperationContext, in ListVisitorsInput) (ListVisitorsResult, error) {
	result := ListVisitorsResult{Visitors: []VisitorSummary{}, Activities: []VisitorActivity{}}
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return result, err
	}
	if rt.Authorize == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, false, "统计授权未装配")
	}
	if err := rt.Authorize(ctx, op, "stats.read", "analytics.visitors"); err != nil {
		return result, err
	}
	from, e1 := time.Parse(time.RFC3339Nano, in.From)
	to, e2 := time.Parse(time.RFC3339Nano, in.To)
	if e1 != nil || e2 != nil || !from.Before(to) || to.Sub(from) > 31*24*time.Hour || in.Limit < 1 || in.Limit > 100 {
		return result, platform.NewError(platform.CodeValidation, false, "访客查询范围不合法")
	}
	start, end := from.UTC().Format(analyticsTimeLayout), to.UTC().Format(analyticsTimeLayout)
	actor := op.Actor
	actor.ExpiresAt = ""
	binding, _ := platform.CanonicalJSON(struct {
		Actor platform.AuthContext
		Epoch string
		Input ListVisitorsInput
	}{actor, op.DataEpoch, ListVisitorsInput{From: start, To: end, VisitorID: in.VisitorID, Limit: in.Limit}})
	scope := "analytics.visitors.v1:" + platform.HashBytes(binding)
	var after struct{ At, ID string }
	if in.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(rt.Key, scope, in.Cursor)
		if err != nil {
			return result, err
		}
		if json.Unmarshal(raw, &after) != nil || after.At == "" || !platform.ValidIdentifier(after.ID) {
			return result, platform.NewError(platform.CodeValidation, false, "访客游标不合法")
		}
	}
	tx, err := rt.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, storageError(err)
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT visitor_id),COALESCE(SUM(event_type='page-view'),0) FROM visitor_activities WHERE data_epoch=? AND occurred_at>=? AND occurred_at<?`, op.DataEpoch, start, end).Scan(&result.TotalVisitors, &result.PageViews); err != nil {
		return result, storageError(err)
	}
	if in.VisitorID == "" {
		rows, err := tx.QueryContext(ctx, `WITH visitors AS (SELECT visitor_id,MIN(occurred_at) first_seen,MAX(occurred_at) last_seen,SUM(event_type='page-view') views,COUNT(*) activities FROM visitor_activities WHERE data_epoch=? AND occurred_at>=? AND occurred_at<? GROUP BY visitor_id) SELECT v.visitor_id,v.first_seen,v.last_seen,v.views,v.activities,a.ip,a.user_agent,a.country FROM visitors v JOIN visitor_activities a ON a.activity_id=(SELECT activity_id FROM visitor_activities WHERE data_epoch=? AND visitor_id=v.visitor_id AND occurred_at>=? AND occurred_at<? ORDER BY occurred_at DESC,activity_id DESC LIMIT 1) WHERE (?='' OR v.last_seen<? OR (v.last_seen=? AND v.visitor_id<?)) ORDER BY v.last_seen DESC,v.visitor_id DESC LIMIT ?`, op.DataEpoch, start, end, op.DataEpoch, start, end, after.At, after.At, after.At, after.ID, in.Limit+1)
		if err != nil {
			return result, storageError(err)
		}
		for rows.Next() {
			var v VisitorSummary
			if err = rows.Scan(&v.VisitorID, &v.FirstSeen, &v.LastSeen, &v.PageViews, &v.Activities, &v.IP, &v.UserAgent, &v.Country); err != nil {
				break
			}
			v.Device, v.Browser, v.OS = visitorDevice(v.UserAgent)
			result.Visitors = append(result.Visitors, v)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return result, storageError(err)
		}
		if len(result.Visitors) > in.Limit {
			result.PageInfo.HasMore = true
			result.Visitors = result.Visitors[:in.Limit]
			last := result.Visitors[in.Limit-1]
			after.At, after.ID = last.LastSeen, last.VisitorID
		}
	} else {
		rows, err := tx.QueryContext(ctx, `SELECT activity_id,document_id,locale,event_type,ip,user_agent,occurred_at,referrer_domain FROM visitor_activities WHERE data_epoch=? AND visitor_id=? AND occurred_at>=? AND occurred_at<? AND (?='' OR occurred_at<? OR (occurred_at=? AND activity_id<?)) ORDER BY occurred_at DESC,activity_id DESC LIMIT ?`, op.DataEpoch, in.VisitorID, start, end, after.At, after.At, after.At, after.ID, in.Limit+1)
		if err != nil {
			return result, storageError(err)
		}
		for rows.Next() {
			var a VisitorActivity
			if err = rows.Scan(&a.ID, &a.DocumentID, &a.Locale, &a.EventType, &a.IP, &a.UserAgent, &a.OccurredAt, &a.ReferrerDomain); err != nil {
				break
			}
			result.Activities = append(result.Activities, a)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return result, storageError(err)
		}
		if len(result.Activities) > in.Limit {
			result.PageInfo.HasMore = true
			result.Activities = result.Activities[:in.Limit]
			last := result.Activities[in.Limit-1]
			after.At, after.ID = last.OccurredAt, last.ID
		}
	}
	if result.PageInfo.HasMore {
		raw, _ := json.Marshal(after)
		result.PageInfo.NextCursor, err = serviceauth.SignCursor(rt.Key, scope, raw)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// 设备信息仅是User-Agent的可读摘要，不用于身份或授权。
func visitorDevice(agent string) (device, browser, os string) {
	a := strings.ToLower(agent)
	device = "desktop"
	if strings.Contains(a, "mobile") {
		device = "mobile"
	}
	if strings.Contains(a, "ipad") || strings.Contains(a, "tablet") {
		device = "tablet"
	}
	browser = "unknown"
	for _, b := range []struct{ key, name string }{{"edg/", "Edge"}, {"opr/", "Opera"}, {"firefox/", "Firefox"}, {"chrome/", "Chrome"}, {"safari/", "Safari"}} {
		if strings.Contains(a, b.key) {
			browser = b.name
			break
		}
	}
	os = "unknown"
	for _, v := range []struct{ key, name string }{{"windows", "Windows"}, {"android", "Android"}, {"iphone", "iOS"}, {"ipad", "iPadOS"}, {"macintosh", "macOS"}, {"linux", "Linux"}} {
		if strings.Contains(a, v.key) {
			os = v.name
			break
		}
	}
	return
}
