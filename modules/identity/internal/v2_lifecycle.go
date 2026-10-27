package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/modules/identity/internal/gen"
)

// 站点生命周期：下架、恢复、删除时在同一事务里发 identity.site.changed，各服务据此停掉或恢复该站点的后台工作。
// 删除的站点保留 siteRetention，期内所有者可以恢复；到期后发 identity.site.purged，各服务清掉该站点的数据，
// 本域删掉站点、成员、域名和私有访问码，站点名随之放出。

const (
	siteChangedEvent = "identity.site.changed"
	sitePurgedEvent  = "identity.site.purged"
	siteRetention    = 30 * 24 * time.Hour
	sitePurgeEvery   = time.Minute
)

// DeletedSiteRef 是恢复接口的路径参数：不叫 siteId，入口就不会按已删除站点去授权（那会被拒）。
type DeletedSiteRef struct{ SiteID string }

func publishSiteEvent(tx *sql.Tx, eventType string, dataEpoch platform.Identifier, siteID, status string, revision int64) error {
	payload := map[string]any{"siteId": siteID, "revision": revision}
	if status != "" {
		payload["status"] = status
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "站点事件载荷不可编码")
	}
	if err := queue.EnqueueEvent(tx, platform.EventEnvelope{
		EventID: platform.NewIdentifier(), EventType: eventType, EventVersion: 1, Producer: "identity", SubjectID: siteID,
		Revision: revision, OccurredAt: platform.Now(), DataEpoch: dataEpoch, SiteID: siteID, Payload: raw,
	}); err != nil {
		return mapIdentityOutboxError(err)
	}
	return nil
}

// announceSite 按站点当前的状态和修订号发 identity.site.changed，与状态变化同一事务。
func announceSite(ctx context.Context, tx *sql.Tx, dataEpoch platform.Identifier, siteID string) error {
	var status string
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT status, revision FROM sites WHERE site_id=?`, siteID).Scan(&status, &revision); err != nil {
		return identityDBError()
	}
	return publishSiteEvent(tx, siteChangedEvent, dataEpoch, siteID, status, revision)
}

// ownedSites 列出某用户名下处于某状态的站点。
func ownedSites(ctx context.Context, q sqlQuerier, ownerID, status string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT site_id FROM sites WHERE owner_user_id=? AND status=?`, ownerID, status)
	if err != nil {
		return nil, identityDBError()
	}
	defer rows.Close()
	var sites []string
	for rows.Next() {
		var site string
		if err := rows.Scan(&site); err != nil {
			return nil, identityDBError()
		}
		sites = append(sites, site)
	}
	return sites, nil
}

// purgeAt 是已删除站点的清除时间。
func purgeAt(deletedAt string) time.Time { return parseTS(deletedAt).Add(siteRetention) }

// RestoreSite 所有者在保留期内恢复已删除的站点，回到删除前的状态（删除前已下架的仍是下架）；仍受站点数上限约束。
func RestoreSite(ctx context.Context, op OperationContext, input DeletedSiteRef) (gen.SiteResult, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.SiteResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.SiteResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	defer tx.Rollback()
	var owner, status string
	var restore, deletedAt sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id,status,restore_status,deleted_at FROM sites WHERE site_id=?`, input.SiteID).Scan(&owner, &status, &restore, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) || err == nil && owner != userID {
		return gen.SiteResult{}, platform.NewError(platform.CodeNotFound, false, "站点不存在")
	}
	if err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	if status != "deleted" {
		return gen.SiteResult{}, platform.NewError(platform.CodeConflict, false, "站点没有被删除")
	}
	if !deletedAt.Valid || !purgeAt(deletedAt.String).After(time.Now()) {
		return gen.SiteResult{}, platform.NewError(platform.CodeConflict, false, "已超过保留期，站点不能再恢复")
	}
	limits, err := userLimits(ctx, tx, userID)
	if err != nil {
		return gen.SiteResult{}, err
	}
	if limits.SiteCount >= limits.MaxSites {
		return gen.SiteResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "已达到站点数量上限，先删掉别的站点再恢复")
	}
	next := "active"
	if restore.Valid && restore.String != "" {
		next = restore.String
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sites SET status=?, restore_status=NULL, deleted_at=NULL, revision=revision+1, updated_at=? WHERE site_id=?`, next, platform.Now(), input.SiteID); err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	if err := announceSite(ctx, tx, op.DataEpoch, input.SiteID); err != nil {
		return gen.SiteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	s, err := siteSummary(ctx, store.db, input.SiteID, "owner")
	return gen.SiteResult{Site: s}, err
}

// runSitePurge 每分钟清除一批保留期已满的站点。
func runSitePurge(ctx context.Context, store *Store, siteConfig platform.SiteConfigClient, dataEpoch platform.Identifier) error {
	return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{
		AdmitBatch:  lifecycle.SimpleAdmission(siteConfig.ReadRuntimeState, dataEpoch),
		PollMillis:  int(sitePurgeEvery / time.Millisecond),
		Runtime:     siteConfig.ReadRuntimeState,
		WriteWindow: lifecycle.SimpleWriteWindow(siteConfig.ReadRuntimeState),
		Recover: func(work context.Context) error {
			return purgeExpiredSites(work, store.db, dataEpoch)
		},
	})
}

// purgeExpiredSites 每个站点一个事务：发 identity.site.purged，再删掉私有访问码、域名、成员和站点本身。
func purgeExpiredSites(ctx context.Context, db *sql.DB, dataEpoch platform.Identifier) error {
	rows, err := db.QueryContext(ctx, `SELECT site_id, deleted_at FROM sites WHERE status='deleted' AND deleted_at IS NOT NULL`)
	if err != nil {
		return identityDBError()
	}
	var expired []string
	for rows.Next() {
		var site, deletedAt string
		if err := rows.Scan(&site, &deletedAt); err != nil {
			rows.Close()
			return identityDBError()
		}
		if !purgeAt(deletedAt).After(time.Now()) {
			expired = append(expired, site)
		}
	}
	rows.Close()
	for _, site := range expired {
		if err := purgeSite(ctx, db, dataEpoch, site); err != nil {
			return err
		}
	}
	return nil
}

func purgeSite(ctx context.Context, db *sql.DB, dataEpoch platform.Identifier, site string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return identityDBError()
	}
	defer tx.Rollback()
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM sites WHERE site_id=? AND status='deleted'`, site).Scan(&revision); err != nil {
		return identityDBError()
	}
	if err := publishSiteEvent(tx, sitePurgedEvent, dataEpoch, site, "", revision+1); err != nil {
		return err
	}
	for _, statement := range []string{
		`DELETE FROM private_access_codes WHERE site_id=?`,
		`DELETE FROM site_domains WHERE site_id=?`,
		`DELETE FROM site_members WHERE site_id=?`,
		`DELETE FROM sites WHERE site_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, site); err != nil {
			return identityDBError()
		}
	}
	if err := tx.Commit(); err != nil {
		return identityDBError()
	}
	return nil
}
