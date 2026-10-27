package identity

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal/gen"
)

// 站主后台：平台设置、注册邀请码、用户、全部站点、举报。

func platformSettingsView(ctx context.Context, q sqlQuerier, settings platformSettings) gen.PlatformSettings {
	view := gen.PlatformSettings{RegistrationOpen: settings.RegistrationOpen, EmailRegistration: settings.EmailRegistration, InviteRequired: settings.InviteRequired,
		DefaultMaxSites: settings.DefaultMaxSites, DefaultStorageBytes: int(settings.DefaultStorageBytes), DefaultVisitorQaDaily: settings.DefaultVisitorQaDaily,
		PlatformHost: settings.PlatformHost, SitesDomain: settings.SitesDomain, Providers: []gen.ProviderConfig{}}
	for _, name := range configurableProviders() {
		conf := settings.Providers[name]
		view.Providers = append(view.Providers, gen.ProviderConfig{Provider: gen.ProviderConfigProvider(name), Enabled: conf.Enabled, ClientId: conf.ClientID, ClientSecretSet: hasProviderSecret(ctx, q, name)})
	}
	return view
}

func ReadPlatformSettings(ctx context.Context, op OperationContext, _ struct{}) (gen.PlatformSettings, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.PlatformSettings{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.PlatformSettings{}, err
	}
	return platformSettingsView(ctx, store.db, settings), nil
}

func UpdatePlatformSettings(ctx context.Context, op OperationContext, input gen.UpdatePlatformSettingsInput) (gen.PlatformSettings, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.PlatformSettings{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.PlatformSettings{}, err
	}
	if input.RegistrationOpen != nil {
		settings.RegistrationOpen = *input.RegistrationOpen
	}
	if input.EmailRegistration != nil {
		settings.EmailRegistration = *input.EmailRegistration
	}
	if input.InviteRequired != nil {
		settings.InviteRequired = *input.InviteRequired
	}
	if input.DefaultMaxSites != nil {
		settings.DefaultMaxSites = *input.DefaultMaxSites
	}
	if input.DefaultStorageBytes != nil {
		settings.DefaultStorageBytes = int64(*input.DefaultStorageBytes)
	}
	if input.DefaultVisitorQaDaily != nil {
		settings.DefaultVisitorQaDaily = *input.DefaultVisitorQaDaily
	}
	if input.PlatformHost != nil {
		settings.PlatformHost = strings.ToLower(strings.TrimSpace(*input.PlatformHost))
	}
	if input.SitesDomain != nil {
		settings.SitesDomain = strings.ToLower(strings.TrimSpace(*input.SitesDomain))
	}
	if input.Providers != nil {
		for _, update := range *input.Providers {
			name := string(update.Provider)
			settings.Providers[name] = providerConf{Enabled: update.Enabled, ClientID: strings.TrimSpace(update.ClientId)}
			if update.ClientSecret != nil && *update.ClientSecret != "" {
				if err := saveProviderSecret(ctx, store.db, name, *update.ClientSecret); err != nil {
					return gen.PlatformSettings{}, err
				}
			}
		}
	}
	if err := saveSettings(ctx, store.db, settings); err != nil {
		return gen.PlatformSettings{}, err
	}
	return platformSettingsView(ctx, store.db, settings), nil
}

// ---- 注册邀请码 ----

func inviteView(ctx context.Context, q sqlQuerier, inviteID string) (gen.RegistrationInvite, error) {
	var v gen.RegistrationInvite
	var expires, revoked sql.NullString
	var created string
	err := q.QueryRowContext(ctx, `SELECT invite_id,note,max_uses,used,expires_at,revoked_at,created_at FROM registration_invites WHERE invite_id=?`, inviteID).
		Scan(&v.InviteId, &v.Note, &v.MaxUses, &v.Used, &expires, &revoked, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return v, platform.NewError(platform.CodeNotFound, false, "邀请码不存在")
	}
	if err != nil {
		return v, identityDBError()
	}
	if expires.Valid {
		e := parseTS(expires.String)
		v.ExpiresAt = &e
	}
	v.Revoked, v.CreatedAt = revoked.Valid, parseTS(created)
	return v, nil
}

func CreateRegistrationInvite(ctx context.Context, op OperationContext, input gen.CreateRegistrationInviteInput) (gen.CreateRegistrationInviteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.CreateRegistrationInviteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	id, code, note := platform.NewIdentifier(), "nwr-"+randomCode(12), ""
	if input.Note != nil {
		note = *input.Note
	}
	var expires any
	if input.ExpiresAt != nil {
		expires = *input.ExpiresAt
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO registration_invites(invite_id,code_hash,note,max_uses,expires_at,created_by,created_at) VALUES(?,?,?,?,?,?,?)`,
		id, hashCode(code), note, input.MaxUses, expires, op.Actor.SubjectID, platform.Now()); err != nil {
		return gen.CreateRegistrationInviteResult{}, identityDBError()
	}
	v, err := inviteView(ctx, store.db, id)
	return gen.CreateRegistrationInviteResult{Invite: v, Code: code}, err
}

func ListRegistrationInvites(ctx context.Context, op OperationContext, _ struct{}) (gen.ListRegistrationInvitesResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListRegistrationInvitesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	rows, err := store.db.QueryContext(ctx, `SELECT invite_id FROM registration_invites ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return gen.ListRegistrationInvitesResult{}, identityDBError()
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	result := gen.ListRegistrationInvitesResult{Invites: []gen.RegistrationInvite{}}
	for _, id := range ids {
		v, err := inviteView(ctx, store.db, id)
		if err != nil {
			return result, err
		}
		result.Invites = append(result.Invites, v)
	}
	return result, nil
}

func RevokeRegistrationInvite(ctx context.Context, op OperationContext, input InviteRef) (gen.RegistrationInviteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.RegistrationInviteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE registration_invites SET revoked_at=COALESCE(revoked_at,?) WHERE invite_id=?`, platform.Now(), input.InviteID); err != nil {
		return gen.RegistrationInviteResult{}, identityDBError()
	}
	v, err := inviteView(ctx, store.db, input.InviteID)
	return gen.RegistrationInviteResult{Invite: v}, err
}

// ---- 用户与站点管理 ----

func userSummary(ctx context.Context, q sqlQuerier, userID string) (gen.UserSummary, error) {
	var u gen.UserSummary
	var status, created string
	err := q.QueryRowContext(ctx, `SELECT user_id,account_id,email,display_name,status,created_at,
        (SELECT COUNT(*) FROM site_members m JOIN sites s ON s.site_id=m.site_id WHERE m.user_id=users.user_id AND s.status<>'deleted') FROM users WHERE user_id=?`, userID).
		Scan(&u.UserId, &u.AccountId, &u.Email, &u.DisplayName, &status, &created, &u.SiteCount)
	if errors.Is(err, sql.ErrNoRows) {
		return u, platform.NewError(platform.CodeNotFound, false, "用户不存在")
	}
	if err != nil {
		return u, identityDBError()
	}
	u.Status, u.CreatedAt = gen.UserSummaryStatus(status), parseTS(created)
	return u, nil
}

func ListUsers(ctx context.Context, op OperationContext, input PageInput) (gen.ListUsersResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListUsersResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := store.db.QueryContext(ctx, `SELECT user_id FROM users WHERE user_id>? ORDER BY user_id LIMIT ?`, input.Cursor, limit+1)
	if err != nil {
		return gen.ListUsersResult{}, identityDBError()
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	result := gen.ListUsersResult{Users: []gen.UserSummary{}}
	if len(ids) > limit {
		next := ids[limit-1]
		result.NextCursor = &next
		ids = ids[:limit]
	}
	for _, id := range ids {
		u, err := userSummary(ctx, store.db, id)
		if err != nil {
			return result, err
		}
		result.Users = append(result.Users, u)
	}
	return result, nil
}

// SetUserStatus 停用或恢复用户：同时改家账户状态，停用后所有会话立即失效。
func SetUserStatus(ctx context.Context, op OperationContext, input SetUserStatusCmd) (gen.UserResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.UserResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	status := string(input.Body.Status)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.UserResult{}, identityDBError()
	}
	defer tx.Rollback()
	var accountID string
	var superadmin bool
	if err := tx.QueryRowContext(ctx, `SELECT u.account_id,a.is_superadmin FROM users u JOIN accounts a ON a.account_id=u.account_id WHERE u.user_id=?`, input.UserID).Scan(&accountID, &superadmin); err != nil {
		return gen.UserResult{}, platform.NewError(platform.CodeNotFound, false, "用户不存在")
	}
	if superadmin {
		return gen.UserResult{}, platform.NewError(platform.CodeForbidden, false, "不能停用站主")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET status=?, updated_at=? WHERE user_id=?`, status, platform.Now(), input.UserID); err != nil {
		return gen.UserResult{}, identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET status=?, revision=revision+1, permission_revision=permission_revision+1, updated_at=? WHERE account_id=?`, status, platform.Now(), accountID); err != nil {
		return gen.UserResult{}, identityDBError()
	}
	if err := tx.Commit(); err != nil {
		return gen.UserResult{}, identityDBError()
	}
	u, err := userSummary(ctx, store.db, input.UserID)
	return gen.UserResult{User: u}, err
}

func adminSite(ctx context.Context, q sqlQuerier, siteID string) (gen.AdminSite, error) {
	var s gen.AdminSite
	var status, created string
	err := q.QueryRowContext(ctx, `SELECT s.site_id,s.slug,s.name,s.owner_user_id,u.email,s.status,s.created_at FROM sites s JOIN users u ON u.user_id=s.owner_user_id WHERE s.site_id=?`, siteID).
		Scan(&s.SiteId, &s.Slug, &s.Name, &s.OwnerUserId, &s.OwnerEmail, &status, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return s, platform.NewError(platform.CodeNotFound, false, "站点不存在")
	}
	if err != nil {
		return s, identityDBError()
	}
	s.Status, s.CreatedAt = gen.AdminSiteStatus(status), parseTS(created)
	return s, nil
}

func ListAllSites(ctx context.Context, op OperationContext, input PageInput) (gen.ListAllSitesResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListAllSitesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := store.db.QueryContext(ctx, `SELECT site_id FROM sites WHERE site_id>? ORDER BY site_id LIMIT ?`, input.Cursor, limit+1)
	if err != nil {
		return gen.ListAllSitesResult{}, identityDBError()
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	result := gen.ListAllSitesResult{Sites: []gen.AdminSite{}}
	if len(ids) > limit {
		next := ids[limit-1]
		result.NextCursor = &next
		ids = ids[:limit]
	}
	for _, id := range ids {
		s, err := adminSite(ctx, store.db, id)
		if err != nil {
			return result, err
		}
		result.Sites = append(result.Sites, s)
	}
	return result, nil
}

func SetSiteStatus(ctx context.Context, op OperationContext, input SetSiteStatusCmd) (gen.AdminSiteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.AdminSiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE sites SET status=?, revision=revision+1, updated_at=? WHERE site_id=? AND status<>'deleted'`, string(input.Body.Status), platform.Now(), input.SiteID); err != nil {
		return gen.AdminSiteResult{}, identityDBError()
	}
	s, err := adminSite(ctx, store.db, input.SiteID)
	return gen.AdminSiteResult{Site: s}, err
}

// ---- 举报 ----

// SubmitReport 访客举报：按域名找到站点后记下，由站主处理。
func SubmitReport(ctx context.Context, op OperationContext, input gen.SubmitReportInput) (gen.SubmitReportResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SubmitReportResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	host := strings.ToLower(strings.TrimSpace(input.Host))
	var siteID string
	_ = store.db.QueryRowContext(ctx, `SELECT site_id FROM site_domains WHERE host=?`, host).Scan(&siteID)
	id := platform.NewIdentifier()
	var url, detail, contactHash string
	if input.Url != nil {
		url = *input.Url
	}
	if input.Detail != nil {
		detail = *input.Detail
	}
	if input.Contact != nil && *input.Contact != "" {
		contactHash = hashCode(strings.ToLower(*input.Contact))
	}
	var open int
	_ = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE status='open' AND host=?`, host).Scan(&open)
	if open >= 50 {
		return gen.SubmitReportResult{ReportId: id}, nil // 同一站点未处理举报过多时不再重复记录
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO reports(report_id,site_id,host,url,reason,detail,contact_hash,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, siteID, host, url, string(input.Reason), detail, contactHash, platform.Now()); err != nil {
		return gen.SubmitReportResult{}, identityDBError()
	}
	return gen.SubmitReportResult{ReportId: id}, nil
}

func reportView(ctx context.Context, q sqlQuerier, id string) (gen.Report, error) {
	var r gen.Report
	var reason, status, created string
	err := q.QueryRowContext(ctx, `SELECT report_id,site_id,host,url,reason,detail,status,resolution,created_at FROM reports WHERE report_id=?`, id).
		Scan(&r.ReportId, &r.SiteId, &r.Host, &r.Url, &reason, &r.Detail, &status, &r.Resolution, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return r, platform.NewError(platform.CodeNotFound, false, "举报不存在")
	}
	if err != nil {
		return r, identityDBError()
	}
	r.Reason, r.Status, r.CreatedAt = gen.ReportReason(reason), gen.ReportStatus(status), parseTS(created)
	return r, nil
}

func ListReports(ctx context.Context, op OperationContext, _ struct{}) (gen.ListReportsResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListReportsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	rows, err := store.db.QueryContext(ctx, `SELECT report_id FROM reports ORDER BY (status='open') DESC, created_at DESC LIMIT 200`)
	if err != nil {
		return gen.ListReportsResult{}, identityDBError()
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	result := gen.ListReportsResult{Reports: []gen.Report{}}
	for _, id := range ids {
		r, err := reportView(ctx, store.db, id)
		if err != nil {
			return result, err
		}
		result.Reports = append(result.Reports, r)
	}
	return result, nil
}

// ResolveReport 驳回、下架站点，或封禁所有者（停用其账号并下架其全部站点）。
func ResolveReport(ctx context.Context, op OperationContext, input ResolveReportCmd) (gen.ReportResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ReportResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	report, err := reportView(ctx, store.db, input.ReportID)
	if err != nil {
		return gen.ReportResult{}, err
	}
	note := ""
	if input.Body.Note != nil {
		note = *input.Body.Note
	}
	status := "dismissed"
	switch input.Body.Action {
	case "takedown", "ban_owner":
		if report.SiteId == "" {
			return gen.ReportResult{}, platform.NewError(platform.CodeValidation, false, "举报没有对应站点")
		}
		tx, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			return gen.ReportResult{}, identityDBError()
		}
		defer tx.Rollback()
		var ownerID string
		if err := tx.QueryRowContext(ctx, `SELECT owner_user_id FROM sites WHERE site_id=?`, report.SiteId).Scan(&ownerID); err != nil {
			return gen.ReportResult{}, platform.NewError(platform.CodeNotFound, false, "站点不存在")
		}
		status = "takedown"
		if input.Body.Action == "ban_owner" {
			status = "banned"
			if _, err := tx.ExecContext(ctx, `UPDATE sites SET status='suspended', revision=revision+1, updated_at=? WHERE owner_user_id=? AND status='active'`, platform.Now(), ownerID); err != nil {
				return gen.ReportResult{}, identityDBError()
			}
			if _, err := tx.ExecContext(ctx, `UPDATE users SET status='disabled', updated_at=? WHERE user_id=?`, platform.Now(), ownerID); err != nil {
				return gen.ReportResult{}, identityDBError()
			}
			if _, err := tx.ExecContext(ctx, `UPDATE accounts SET status='disabled', revision=revision+1, permission_revision=permission_revision+1, updated_at=?
                WHERE account_id=(SELECT account_id FROM users WHERE user_id=?) AND is_superadmin=0`, platform.Now(), ownerID); err != nil {
				return gen.ReportResult{}, identityDBError()
			}
		} else if _, err := tx.ExecContext(ctx, `UPDATE sites SET status='suspended', revision=revision+1, updated_at=? WHERE site_id=?`, platform.Now(), report.SiteId); err != nil {
			return gen.ReportResult{}, identityDBError()
		}
		if err := tx.Commit(); err != nil {
			return gen.ReportResult{}, identityDBError()
		}
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE reports SET status=?, resolution=? WHERE report_id=?`, status, note, input.ReportID); err != nil {
		return gen.ReportResult{}, identityDBError()
	}
	r, err := reportView(ctx, store.db, input.ReportID)
	return gen.ReportResult{Report: r}, err
}
