package trafficguard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

type CreateBanInput struct {
	IP         string `json:"ip"`
	Scope      string `json:"-"`
	TTLSeconds int64  `json:"ttlSeconds"`
	ReasonCode string `json:"reasonCode"`
	DataEpoch  string `json:"-"`
}
type CreateBanResult struct {
	BanID    string `json:"banId"`
	Revision int64  `json:"revision"`
}
type RevokeBanInput struct {
	BanID            string
	ExpectedRevision int64
	DataEpoch        string
}
type RevokeBanResult struct {
	Revision int64  `json:"revision"`
	State    string `json:"state"`
}
type ListBansInput struct {
	Cursor    string
	Limit     int
	DataEpoch string
}
type BanRecord struct {
	IP         string `json:"ip"`
	BanID      string `json:"banId"`
	SubjectRef string `json:"subjectRef"`
	Scope      string `json:"scope"`
	ReasonCode string `json:"reasonCode"`
	StartsAt   string `json:"startsAt"`
	ExpiresAt  string `json:"expiresAt"`
	State      string `json:"state"`
	Revision   int64  `json:"revision"`
}
type ListBansResult struct {
	BanIDs     []string    `json:"banIds"`
	Items      []BanRecord `json:"items"`
	NextCursor string      `json:"nextCursor"`
}

// CreateBan 按单个IP和时长限制AI问答，不影响公开阅读和管理员登录。
func CreateBan(ctx context.Context, op OperationContext, in CreateBanInput) (CreateBanResult, error) {
	if err := authorizeGuardManagement(ctx, op, "createBan"); err != nil {
		return CreateBanResult{}, err
	}
	peer, err := netip.ParseAddr(in.IP)
	if err != nil || peer.Zone() != "" {
		return CreateBanResult{}, platform.NewError(platform.CodeValidation, false, "请输入单个IPv4或IPv6地址")
	}
	in.IP = peer.Unmap().String()
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return CreateBanResult{}, err
	}
	defer t.close()
	var result CreateBanResult
	if found, err := readGuardReceipt(t, "ban.create", in, &result); err != nil {
		return CreateBanResult{}, err
	} else if found {
		return result, nil
	}
	ttl := in.TTLSeconds
	if ttl == 0 {
		ttl = 600
	}
	if ttl < 1 || ttl > 2592000 || (in.ReasonCode != "manual" && in.ReasonCode != "abuse") {
		return CreateBanResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_BAN")
	}
	result, err = createIPBan(t, in.IP, ttl, in.ReasonCode)
	if err != nil {
		return CreateBanResult{}, err
	}
	if err = saveGuardReceipt(t, "ban.create", in, result); err != nil {
		return CreateBanResult{}, err
	}
	if err = t.commit(); err != nil {
		return CreateBanResult{}, err
	}
	return result, nil
}

func RevokeBan(ctx context.Context, op OperationContext, in RevokeBanInput) (RevokeBanResult, error) {
	if err := authorizeGuardManagement(ctx, op, "revokeBan"); err != nil {
		return RevokeBanResult{}, err
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return RevokeBanResult{}, err
	}
	defer t.close()
	var result RevokeBanResult
	if found, err := readGuardReceipt(t, "ban.revoke", in, &result); err != nil {
		return RevokeBanResult{}, err
	} else if found {
		return result, nil
	}
	var state string
	var revision int64
	err = t.tx.QueryRowContext(t.ctx, `SELECT state,revision FROM guard_bans WHERE ban_id=? AND site_id=? AND data_epoch=?`, in.BanID, t.site, op.DataEpoch).Scan(&state, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return RevokeBanResult{}, platform.NewError(platform.CodeNotFound, false, "禁令不存在")
	}
	if err != nil {
		return RevokeBanResult{}, loginDBError()
	}
	if revision != in.ExpectedRevision || state != "active" || revision == 9223372036854775807 {
		return RevokeBanResult{}, platform.NewError(platform.CodeVersionMismatch, false, "禁令状态或修订已变化")
	}
	result = RevokeBanResult{Revision: revision + 1, State: "revoked"}
	_, err = t.tx.ExecContext(t.ctx, `UPDATE guard_bans SET state='revoked',revision=? WHERE ban_id=? AND site_id=? AND data_epoch=? AND revision=?`, result.Revision, in.BanID, t.site, op.DataEpoch, revision)
	if err != nil {
		return RevokeBanResult{}, loginDBError()
	}
	if err = saveGuardReceipt(t, "ban.revoke", in, result); err != nil {
		return RevokeBanResult{}, err
	}
	if err = enqueueGuardEvent(t, "traffic-guard.ban-changed.v1", in.BanID, result.Revision, map[string]any{"banId": in.BanID, "banRevision": result.Revision, "state": result.State}); err != nil {
		return RevokeBanResult{}, err
	}
	if err = t.commit(); err != nil {
		return RevokeBanResult{}, err
	}
	return result, nil
}

func ListBans(ctx context.Context, op OperationContext, in ListBansInput) (ListBansResult, error) {
	if err := authorizeGuardManagement(ctx, op, "listBans"); err != nil {
		return ListBansResult{}, err
	}
	t, err := beginGuardTransaction(ctx, op, false)
	if err != nil {
		return ListBansResult{}, err
	}
	defer t.close()
	limit := in.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return ListBansResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_LIMIT")
	}
	scope := "guard.bans:" + t.site + ":" + op.DataEpoch + ":" + op.Actor.SubjectID
	after := ""
	if in.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(t.runtime.SigningKey, scope, in.Cursor)
		if err != nil {
			return ListBansResult{}, err
		}
		if json.Unmarshal(raw, &after) != nil {
			return ListBansResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_CURSOR")
		}
	}
	rows, err := t.tx.QueryContext(t.ctx, `SELECT ban_id,network_key,scope,reason_code,starts_at_ns,expires_at_ns,state,revision,ip FROM guard_bans
  WHERE site_id=? AND data_epoch=? AND ban_id>? ORDER BY ban_id LIMIT ?`, t.site, op.DataEpoch, after, limit+1)
	if err != nil {
		return ListBansResult{}, loginDBError()
	}
	defer rows.Close()
	items := make([]BanRecord, 0, limit+1)
	for rows.Next() {
		var item BanRecord
		var starts, expires int64
		if rows.Scan(&item.BanID, &item.SubjectRef, &item.Scope, &item.ReasonCode, &starts, &expires, &item.State, &item.Revision, &item.IP) != nil {
			return ListBansResult{}, loginDBError()
		}
		item.StartsAt = time.Unix(0, starts).UTC().Format(time.RFC3339Nano)
		item.ExpiresAt = time.Unix(0, expires).UTC().Format(time.RFC3339Nano)
		items = append(items, item)
	}
	if rows.Err() != nil {
		return ListBansResult{}, loginDBError()
	}
	result := ListBansResult{BanIDs: []string{}, Items: items, NextCursor: ""}
	if len(items) > limit {
		result.Items = items[:limit]
		raw, _ := json.Marshal(result.Items[limit-1].BanID)
		result.NextCursor, err = serviceauth.SignCursor(t.runtime.SigningKey, scope, raw)
		if err != nil {
			return ListBansResult{}, err
		}
	}
	for _, item := range result.Items {
		result.BanIDs = append(result.BanIDs, item.BanID)
	}
	return result, nil
}
