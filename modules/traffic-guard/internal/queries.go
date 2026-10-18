package trafficguard

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"time"
)

type GuardPageRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}
type GuardPageInfo struct {
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
}
type ListRiskEventsInput struct {
	Page  GuardPageRequest
	Since string
}
type RiskEvent struct {
	EventID    string `json:"eventId"`
	Action     string `json:"action"`
	Decision   string `json:"decision"`
	NetworkKey string `json:"networkKey"`
	OccurredAt string `json:"occurredAt"`
}
type ListRiskEventsResult struct {
	Items    []RiskEvent   `json:"items"`
	PageInfo GuardPageInfo `json:"pageInfo"`
}

func ListRiskEvents(ctx context.Context, op OperationContext, input ListRiskEventsInput) (ListRiskEventsResult, error) {
	if err := authorizeGuardManagement(ctx, op, "listRiskEvents"); err != nil {
		return ListRiskEventsResult{}, err
	}
	t, err := beginGuardTransaction(ctx, op, false)
	if err != nil {
		return ListRiskEventsResult{}, err
	}
	defer t.close()
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return ListRiskEventsResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_LIMIT")
	}
	sinceNS := int64(-9223372036854775808)
	if input.Since != "" {
		since, err := platform.ParseTimestamp(input.Since)
		if err != nil {
			return ListRiskEventsResult{}, err
		}
		sinceNS = since.UnixNano()
	}
	scopeJSON, _ := platform.CanonicalJSON(map[string]any{"site": t.site, "epoch": op.DataEpoch, "actor": op.Actor.SubjectID, "since": input.Since})
	scope := "guard.risk:" + platform.HashBytes(scopeJSON)
	position := struct {
		At int64  `json:"at"`
		ID string `json:"id"`
	}{At: 9223372036854775807, ID: ""}
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(t.runtime.SigningKey, scope, input.Page.Cursor)
		if err != nil {
			return ListRiskEventsResult{}, err
		}
		if json.Unmarshal(raw, &position) != nil {
			return ListRiskEventsResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_CURSOR")
		}
	}
	rows, err := t.tx.QueryContext(t.ctx, `SELECT event_id,action,decision,network_key,occurred_at_ns FROM guard_risk_events
  WHERE site_id=? AND data_epoch=? AND occurred_at_ns>=? AND (occurred_at_ns<? OR (occurred_at_ns=? AND event_id<?))
  ORDER BY occurred_at_ns DESC,event_id DESC LIMIT ?`, t.site, op.DataEpoch, sinceNS, position.At, position.At, position.ID, limit+1)
	if err != nil {
		return ListRiskEventsResult{}, loginDBError()
	}
	defer rows.Close()
	items := make([]RiskEvent, 0, limit+1)
	timestamps := make([]int64, 0, limit+1)
	for rows.Next() {
		var item RiskEvent
		var at int64
		if rows.Scan(&item.EventID, &item.Action, &item.Decision, &item.NetworkKey, &at) != nil {
			return ListRiskEventsResult{}, loginDBError()
		}
		item.OccurredAt = time.Unix(0, at).UTC().Format(time.RFC3339Nano)
		items = append(items, item)
		timestamps = append(timestamps, at)
	}
	if rows.Err() != nil {
		return ListRiskEventsResult{}, loginDBError()
	}
	result := ListRiskEventsResult{Items: items, PageInfo: GuardPageInfo{HasMore: len(items) > limit}}
	if result.PageInfo.HasMore {
		result.Items = items[:limit]
		position.At = timestamps[limit-1]
		position.ID = items[limit-1].EventID
		raw, _ := json.Marshal(position)
		result.PageInfo.NextCursor, err = serviceauth.SignCursor(t.runtime.SigningKey, scope, raw)
		if err != nil {
			return ListRiskEventsResult{}, err
		}
	}
	return result, nil
}
