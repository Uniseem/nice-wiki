package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal/gen"
)

// AI 邀请码与额度：邀请码由站主生成，用户兑换后得到平台 AI 额度。
// 这里只放“能花多少、哪些任务可用平台渠道”；已经花了多少由 model-gateway 的账本记录。
// tasks 里写的是任务所属服务（qa、localization、memory、agent…），空表示全部。

func aiInviteView(ctx context.Context, q sqlQuerier, inviteID string) (gen.AiInvite, error) {
	var v gen.AiInvite
	var period, created string
	var tasks []byte
	var maxSites, storage sql.NullInt64
	var expires, revoked sql.NullString
	err := q.QueryRowContext(ctx, `SELECT invite_id,note,cap_micros,period,tasks_json,max_uses,used,max_sites,storage_bytes,expires_at,revoked_at,created_at FROM ai_invites WHERE invite_id=?`, inviteID).
		Scan(&v.InviteId, &v.Note, &v.CapMicros, &period, &tasks, &v.MaxUses, &v.Used, &maxSites, &storage, &expires, &revoked, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return v, platform.NewError(platform.CodeNotFound, false, "AI 邀请码不存在")
	}
	if err != nil {
		return v, identityDBError()
	}
	v.Period = gen.AiInvitePeriod(period)
	if json.Unmarshal(tasks, &v.Tasks) != nil || v.Tasks == nil {
		v.Tasks = []string{}
	}
	if maxSites.Valid {
		n := int(maxSites.Int64)
		v.MaxSites = &n
	}
	if storage.Valid {
		n := int(storage.Int64)
		v.StorageBytes = &n
	}
	if expires.Valid {
		e := parseTS(expires.String)
		v.ExpiresAt = &e
	}
	v.Revoked, v.CreatedAt = revoked.Valid, parseTS(created)
	return v, nil
}

func CreateAiInvite(ctx context.Context, op OperationContext, input gen.CreateAiInviteInput) (gen.CreateAiInviteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.CreateAiInviteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	id, code, note := platform.NewIdentifier(), "nwa-"+randomCode(12), ""
	if input.Note != nil {
		note = *input.Note
	}
	tasks := []string{}
	if input.Tasks != nil {
		tasks = *input.Tasks
	}
	tasksJSON, _ := json.Marshal(tasks)
	var expires, maxSites, storage any
	if input.ExpiresAt != nil {
		expires = *input.ExpiresAt
	}
	if input.MaxSites != nil {
		maxSites = *input.MaxSites
	}
	if input.StorageBytes != nil {
		storage = *input.StorageBytes
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO ai_invites(invite_id,code_hash,note,cap_micros,period,tasks_json,max_uses,max_sites,storage_bytes,expires_at,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, hashCode(code), note, input.CapMicros, string(input.Period), string(tasksJSON), input.MaxUses, maxSites, storage, expires, op.Actor.SubjectID, platform.Now()); err != nil {
		return gen.CreateAiInviteResult{}, identityDBError()
	}
	v, err := aiInviteView(ctx, store.db, id)
	return gen.CreateAiInviteResult{Invite: v, Code: code}, err
}

func ListAiInvites(ctx context.Context, op OperationContext, _ struct{}) (gen.ListAiInvitesResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListAiInvitesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	rows, err := store.db.QueryContext(ctx, `SELECT invite_id FROM ai_invites ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return gen.ListAiInvitesResult{}, identityDBError()
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	result := gen.ListAiInvitesResult{Invites: []gen.AiInvite{}}
	for _, id := range ids {
		v, err := aiInviteView(ctx, store.db, id)
		if err != nil {
			return result, err
		}
		result.Invites = append(result.Invites, v)
	}
	return result, nil
}

func RevokeAiInvite(ctx context.Context, op OperationContext, input InviteRef) (gen.AiInviteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.AiInviteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE ai_invites SET revoked_at=COALESCE(revoked_at,?) WHERE invite_id=?`, platform.Now(), input.InviteID); err != nil {
		return gen.AiInviteResult{}, identityDBError()
	}
	v, err := aiInviteView(ctx, store.db, input.InviteID)
	return gen.AiInviteResult{Invite: v}, err
}

// quotaPeriodStart 是额度当前周期的起点：一次性额度从兑换时算起；按月额度以兑换时刻为基准逐月滚动。
func quotaPeriodStart(period string, anchor, now time.Time) time.Time {
	if period != "monthly" {
		return anchor
	}
	months := (now.Year()-anchor.Year())*12 + int(now.Month()-anchor.Month())
	start := anchor.AddDate(0, months, 0)
	if start.After(now) {
		start = anchor.AddDate(0, months-1, 0)
	}
	return start
}

type userAI struct {
	Cap      int64
	Period   string
	Tasks    []string
	Redeemed time.Time
	Anchor   time.Time
}

func loadUserAI(ctx context.Context, q sqlQuerier, userID string) (*userAI, error) {
	var entitlement userAI
	var tasks []byte
	var redeemed, anchor string
	err := q.QueryRowContext(ctx, `SELECT cap_micros,period,tasks_json,redeemed_at,period_anchor FROM user_ai WHERE user_id=?`, userID).Scan(&entitlement.Cap, &entitlement.Period, &tasks, &redeemed, &anchor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, identityDBError()
	}
	if json.Unmarshal(tasks, &entitlement.Tasks) != nil || entitlement.Tasks == nil {
		entitlement.Tasks = []string{}
	}
	entitlement.Redeemed, entitlement.Anchor = parseTS(redeemed), parseTS(anchor)
	return &entitlement, nil
}

func myAiView(entitlement *userAI) gen.MyAi {
	if entitlement == nil {
		return gen.MyAi{Tasks: []string{}}
	}
	period := gen.MyAiPeriod(entitlement.Period)
	redeemed := entitlement.Redeemed
	start := quotaPeriodStart(entitlement.Period, entitlement.Anchor, time.Now().UTC())
	return gen.MyAi{Entitled: true, CapMicros: int(entitlement.Cap), Period: &period, Tasks: entitlement.Tasks, RedeemedAt: &redeemed, PeriodStart: &start}
}

// RedeemAiCode 兑换 AI 邀请码：新码替换旧码，旧码的剩余额度作废（周期起点重算，账本只数新周期）。
func RedeemAiCode(ctx context.Context, op OperationContext, input gen.RedeemAiCodeInput) (gen.MyAiResult, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.MyAiResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.MyAiResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.MyAiResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.MyAiResult{}, identityDBError()
	}
	defer tx.Rollback()
	var inviteID, period string
	var capMicros int64
	var tasks []byte
	var maxUses, used int
	var maxSites, storage sql.NullInt64
	var expires, revoked sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT invite_id,cap_micros,period,tasks_json,max_uses,used,max_sites,storage_bytes,expires_at,revoked_at FROM ai_invites WHERE code_hash=?`, hashCode(input.Code)).
		Scan(&inviteID, &capMicros, &period, &tasks, &maxUses, &used, &maxSites, &storage, &expires, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.MyAiResult{}, platform.NewError(platform.CodeForbidden, false, "AI 邀请码无效")
	}
	if err != nil {
		return gen.MyAiResult{}, identityDBError()
	}
	now := time.Now().UTC()
	if revoked.Valid || used >= maxUses || (expires.Valid && !parseTS(expires.String).After(now)) {
		return gen.MyAiResult{}, platform.NewError(platform.CodeForbidden, false, "AI 邀请码无效")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ai_invites SET used=used+1 WHERE invite_id=?`, inviteID); err != nil {
		return gen.MyAiResult{}, identityDBError()
	}
	stamp := platform.Now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_ai(user_id,invite_id,cap_micros,period,tasks_json,redeemed_at,period_anchor) VALUES(?,?,?,?,?,?,?)
        ON CONFLICT(user_id) DO UPDATE SET invite_id=excluded.invite_id, cap_micros=excluded.cap_micros, period=excluded.period, tasks_json=excluded.tasks_json, redeemed_at=excluded.redeemed_at, period_anchor=excluded.period_anchor`,
		userID, inviteID, capMicros, period, string(tasks), stamp, stamp); err != nil {
		return gen.MyAiResult{}, identityDBError()
	}
	if maxSites.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE user_limits SET max_sites=?, updated_at=? WHERE user_id=?`, maxSites.Int64, stamp, userID); err != nil {
			return gen.MyAiResult{}, identityDBError()
		}
	}
	if storage.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE user_limits SET storage_bytes=?, updated_at=? WHERE user_id=?`, storage.Int64, stamp, userID); err != nil {
			return gen.MyAiResult{}, identityDBError()
		}
	}
	entitlement, err := loadUserAI(ctx, tx, userID)
	if err != nil {
		return gen.MyAiResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return gen.MyAiResult{}, identityDBError()
	}
	return gen.MyAiResult{Ai: myAiView(entitlement)}, nil
}

func ReadMyAi(ctx context.Context, op OperationContext, _ struct{}) (gen.MyAiResult, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.MyAiResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.MyAiResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.MyAiResult{}, err
	}
	entitlement, err := loadUserAI(ctx, store.db, userID)
	return gen.MyAiResult{Ai: myAiView(entitlement)}, err
}

// ReadSiteAiPolicy 告诉 model-gateway 与 qa：这个站点的 AI 费用算在谁头上、额度多少、哪些任务可用平台渠道。
// 平台站主自己的站点不限额；其他用户没兑换过 AI 邀请码就不能用平台渠道（只能用自带 key）。
func ReadSiteAiPolicy(ctx context.Context, op OperationContext, input SiteRef) (gen.SiteAiPolicy, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteAiPolicy{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	policy := gen.SiteAiPolicy{SiteId: input.SiteID, Tasks: []string{}, SiteIds: []string{}}
	var daily sql.NullInt64
	var superadmin bool
	err := store.db.QueryRowContext(ctx, `SELECT s.owner_user_id,s.visitor_qa_daily,a.is_superadmin FROM sites s JOIN users u ON u.user_id=s.owner_user_id JOIN accounts a ON a.account_id=u.account_id WHERE s.site_id=? AND s.status<>'deleted'`, input.SiteID).
		Scan(&policy.OwnerUserId, &daily, &superadmin)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, platform.NewError(platform.CodeNotFound, false, "站点不存在")
	}
	if err != nil {
		return policy, identityDBError()
	}
	if daily.Valid {
		policy.VisitorQaDaily = int(daily.Int64)
	} else {
		settings, err := loadSettings(ctx, store.db)
		if err != nil {
			return policy, err
		}
		policy.VisitorQaDaily = settings.DefaultVisitorQaDaily
	}
	rows, err := store.db.QueryContext(ctx, `SELECT site_id FROM sites WHERE owner_user_id=? AND status<>'deleted'`, policy.OwnerUserId)
	if err != nil {
		return policy, identityDBError()
	}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			policy.SiteIds = append(policy.SiteIds, id)
		}
	}
	rows.Close()
	if superadmin {
		policy.PlatformOwner, policy.PlatformAllowed = true, true
		return policy, nil
	}
	entitlement, err := loadUserAI(ctx, store.db, policy.OwnerUserId)
	if err != nil || entitlement == nil {
		return policy, err
	}
	policy.PlatformAllowed, policy.CapMicros, policy.Tasks = entitlement.Cap > 0, int(entitlement.Cap), entitlement.Tasks
	policy.PeriodStartNs = int(quotaPeriodStart(entitlement.Period, entitlement.Anchor, time.Now().UTC()).UnixNano())
	return policy, nil
}

// ReadSiteStoragePolicy 告诉 media：这个站点的所有者存储上限多少、名下有哪些站点，media 按这些站点合计用量。
func ReadSiteStoragePolicy(ctx context.Context, op OperationContext, input SiteRef) (gen.SiteStoragePolicy, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteStoragePolicy{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	policy := gen.SiteStoragePolicy{SiteId: input.SiteID, SiteIds: []string{}}
	var storage int64
	err := store.db.QueryRowContext(ctx, `SELECT s.owner_user_id,l.storage_bytes FROM sites s JOIN user_limits l ON l.user_id=s.owner_user_id WHERE s.site_id=? AND s.status<>'deleted'`, input.SiteID).
		Scan(&policy.OwnerUserId, &storage)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, platform.NewError(platform.CodeNotFound, false, "站点不存在")
	}
	if err != nil {
		return policy, identityDBError()
	}
	policy.StorageBytes = int(storage)
	rows, err := store.db.QueryContext(ctx, `SELECT site_id FROM sites WHERE owner_user_id=? AND status<>'deleted'`, policy.OwnerUserId)
	if err != nil {
		return policy, identityDBError()
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return policy, identityDBError()
		}
		policy.SiteIds = append(policy.SiteIds, id)
	}
	return policy, nil
}

// NoteVisitorQaCap 记录站点当天访客提问已达上限（站点所有者在后台能看到“今天已暂停”）。
func NoteVisitorQaCap(ctx context.Context, op OperationContext, input SiteRef) (gen.OkResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.OkResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE sites SET qa_paused_on=? WHERE site_id=?`, time.Now().UTC().Format("2006-01-02"), input.SiteID); err != nil {
		return gen.OkResult{}, identityDBError()
	}
	return gen.OkResult{Ok: true}, nil
}
