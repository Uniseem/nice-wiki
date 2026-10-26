package identity

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal/gen"
)

// 站点、成员、域名：用户直接拥有站点，可邀请协作者，每个站点有子域名并可绑定自己的域名。

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,61})[a-z0-9]$`)

// reservedSlugs 是不能当站点名的保留子域名。
var reservedSlugs = map[string]bool{
	"www": true, "app": true, "api": true, "admin": true, "mail": true, "smtp": true, "imap": true, "ftp": true, "cdn": true,
	"static": true, "assets": true, "status": true, "help": true, "docs": true, "blog": true, "support": true, "login": true,
	"signup": true, "register": true, "dashboard": true, "console": true, "root": true, "system": true, "platform": true,
	"localhost": true, "test": true, "dev": true, "staging": true, "nw": true,
}

var hostPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

func requireAdminActor(op OperationContext) error {
	if op.Actor.Kind != "admin" || op.Actor.SubjectID == "" {
		return platform.NewError(platform.CodeUnauthenticated, false, "需要登录")
	}
	return nil
}

func siteSummary(ctx context.Context, q sqlQuerier, siteID, role string) (gen.SiteSummary, error) {
	var s gen.SiteSummary
	var status, visibility, createdAt string
	var daily sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT site_id,slug,name,status,visibility,created_at,
        COALESCE((SELECT host FROM site_domains WHERE site_id=sites.site_id AND is_primary=1 AND status='verified' LIMIT 1),''),
        visitor_qa_daily,qa_paused_on
        FROM sites WHERE site_id=?`, siteID).Scan(&s.SiteId, &s.Slug, &s.Name, &status, &visibility, &createdAt, &s.PrimaryHost, &daily, &s.QaPausedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return s, platform.NewError(platform.CodeNotFound, false, "站点不存在")
	}
	if err != nil {
		return s, identityDBError()
	}
	if daily.Valid {
		s.VisitorQaDaily = int(daily.Int64)
	} else if settings, err := loadSettings(ctx, q); err == nil {
		s.VisitorQaDaily = settings.DefaultVisitorQaDaily
	}
	// 前端按它决定显示哪些页面；真正的权限仍由服务端按角色判定。
	s.Grants, _ = roleGrants(role)
	if s.Grants == nil {
		s.Grants = []string{}
	}
	s.Status, s.Visibility, s.Role, s.CreatedAt = gen.SiteSummaryStatus(status), gen.SiteSummaryVisibility(visibility), gen.SiteSummaryRole(role), parseTS(createdAt)
	return s, nil
}

// ListMySites 列出当前用户所有的站点和额度。
func ListMySites(ctx context.Context, op OperationContext, _ struct{}) (gen.ListMySitesResult, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.ListMySitesResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListMySitesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.ListMySitesResult{}, err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT m.site_id,m.role FROM site_members m JOIN sites s ON s.site_id=m.site_id
        WHERE m.user_id=? AND s.status<>'deleted' ORDER BY s.created_at`, userID)
	if err != nil {
		return gen.ListMySitesResult{}, identityDBError()
	}
	type pair struct{ id, role string }
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.id, &p.role); err != nil {
			rows.Close()
			return gen.ListMySitesResult{}, identityDBError()
		}
		pairs = append(pairs, p)
	}
	rows.Close()
	result := gen.ListMySitesResult{Sites: []gen.SiteSummary{}}
	for _, p := range pairs {
		s, err := siteSummary(ctx, store.db, p.id, p.role)
		if err != nil {
			return gen.ListMySitesResult{}, err
		}
		result.Sites = append(result.Sites, s)
	}
	limits, err := userLimits(ctx, store.db, userID)
	if err != nil {
		return gen.ListMySitesResult{}, err
	}
	result.Limits = limits
	return result, nil
}

func userLimits(ctx context.Context, q sqlQuerier, userID string) (gen.UserLimits, error) {
	var l gen.UserLimits
	var storage int64
	err := q.QueryRowContext(ctx, `SELECT max_sites,storage_bytes,(SELECT COUNT(*) FROM sites WHERE owner_user_id=? AND status<>'deleted') FROM user_limits WHERE user_id=?`, userID, userID).
		Scan(&l.MaxSites, &storage, &l.SiteCount)
	if err != nil {
		return l, identityDBError()
	}
	l.StorageBytes = int(storage)
	return l, nil
}

// CreateSite 创建站点：占一个子域名，创建者成为所有者；受用户站点数上限约束。
func CreateSite(ctx context.Context, op OperationContext, input gen.CreateSiteInput) (gen.SiteResult, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.SiteResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	name := strings.TrimSpace(input.Name)
	if !slugPattern.MatchString(slug) || reservedSlugs[slug] || strings.Contains(slug, "--") || name == "" {
		return gen.SiteResult{}, platform.NewError(platform.CodeValidation, false, "站点名需要 3–63 位小写字母、数字或连字符，且不能是保留名")
	}
	baseLanguage := "zh-Hans"
	if input.BaseLanguage != nil && *input.BaseLanguage != "" {
		baseLanguage = *input.BaseLanguage
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.SiteResult{}, err
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
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE user_id=?`, userID).Scan(&status); err != nil || status != "active" {
		return gen.SiteResult{}, platform.NewError(platform.CodeForbidden, false, "账号不可用")
	}
	limits, err := userLimits(ctx, tx, userID)
	if err != nil {
		return gen.SiteResult{}, err
	}
	if limits.SiteCount >= limits.MaxSites {
		return gen.SiteResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "已达到站点数量上限")
	}
	siteID, now := platform.NewIdentifier(), platform.Now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO sites(site_id,slug,name,owner_user_id,status,visibility,base_language,revision,created_at,updated_at) VALUES(?,?,?,?,'active','public',?,1,?,?)`,
		siteID, slug, name, userID, baseLanguage, now, now); err != nil {
		if stringsContains(err.Error(), "UNIQUE constraint failed") {
			return gen.SiteResult{}, platform.NewError(platform.CodeConflict, false, "站点名已被占用")
		}
		return gen.SiteResult{}, identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO site_members(site_id,user_id,role,created_at) VALUES(?,?,'owner',?)`, siteID, userID, now); err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	domain := slug + "." + sitesDomainOf(settings)
	if _, err := tx.ExecContext(ctx, `INSERT INTO site_domains(host,site_id,kind,status,is_primary,verify_token,verified_at,created_at) VALUES(?,?,'subdomain','verified',1,'',?,?)`, domain, siteID, now, now); err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	if err := tx.Commit(); err != nil {
		return gen.SiteResult{}, identityDBError()
	}
	s, err := siteSummary(ctx, store.db, siteID, "owner")
	return gen.SiteResult{Site: s}, err
}

// sitesDomainOf 是站点子域名的后缀；没配置（开发环境）时用 localhost。
func sitesDomainOf(settings platformSettings) string {
	if settings.SitesDomain != "" {
		return settings.SitesDomain
	}
	return "localhost"
}

// memberRole 返回当前用户在站点里的角色；站主可以以 owner 身份进入任何站点（审核与支持）。
func memberRole(ctx context.Context, q sqlQuerier, op OperationContext, siteID string) (string, error) {
	userID, err := userOfAccount(ctx, q, op.Actor.SubjectID)
	if err != nil {
		return "", err
	}
	var role string
	err = q.QueryRowContext(ctx, `SELECT role FROM site_members WHERE site_id=? AND user_id=?`, siteID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", platform.NewError(platform.CodeForbidden, false, "不是该站点成员")
	}
	if err != nil {
		return "", identityDBError()
	}
	return role, nil
}

func requireSiteScope(op OperationContext, siteID string) error {
	if op.Actor.SiteID != siteID {
		return platform.NewError(platform.CodeForbidden, false, "操作站点与路径不一致")
	}
	return nil
}

// ReadSite 读取站点。
func ReadSite(ctx context.Context, op OperationContext, input SiteRef) (gen.SiteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteResult{}, err
	}
	role, err := memberRole(ctx, store.db, op, input.SiteID)
	if err != nil {
		return gen.SiteResult{}, err
	}
	s, err := siteSummary(ctx, store.db, input.SiteID, role)
	return gen.SiteResult{Site: s}, err
}

// UpdateSite 修改名称或可见性。
func UpdateSite(ctx context.Context, op OperationContext, input UpdateSiteCmd) (gen.SiteResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteResult{}, err
	}
	role, err := memberRole(ctx, store.db, op, input.SiteID)
	if err != nil {
		return gen.SiteResult{}, err
	}
	if role == "viewer" {
		return gen.SiteResult{}, platform.NewError(platform.CodeForbidden, false, "只读成员不能修改站点")
	}
	if input.Body.Name != nil {
		name := strings.TrimSpace(*input.Body.Name)
		if name == "" {
			return gen.SiteResult{}, platform.NewError(platform.CodeValidation, false, "站点名称不能为空")
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE sites SET name=?, revision=revision+1, updated_at=? WHERE site_id=?`, name, platform.Now(), input.SiteID); err != nil {
			return gen.SiteResult{}, identityDBError()
		}
	}
	if input.Body.Visibility != nil {
		if role != "owner" {
			return gen.SiteResult{}, platform.NewError(platform.CodeForbidden, false, "只有所有者能改可见性")
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE sites SET visibility=?, revision=revision+1, updated_at=? WHERE site_id=?`, string(*input.Body.Visibility), platform.Now(), input.SiteID); err != nil {
			return gen.SiteResult{}, identityDBError()
		}
	}
	if input.Body.VisitorQaDaily != nil {
		if role != "owner" {
			return gen.SiteResult{}, platform.NewError(platform.CodeForbidden, false, "只有所有者能改访客提问上限")
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE sites SET visitor_qa_daily=?, revision=revision+1, updated_at=? WHERE site_id=?`, *input.Body.VisitorQaDaily, platform.Now(), input.SiteID); err != nil {
			return gen.SiteResult{}, identityDBError()
		}
	}
	s, err := siteSummary(ctx, store.db, input.SiteID, role)
	return gen.SiteResult{Site: s}, err
}

// DeleteSite 软删除站点（名称保留，域名失效）；只有所有者能做。
func DeleteSite(ctx context.Context, op OperationContext, input SiteRef) (gen.RemovedResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.RemovedResult{}, err
	}
	role, err := memberRole(ctx, store.db, op, input.SiteID)
	if err != nil {
		return gen.RemovedResult{}, err
	}
	if role != "owner" {
		return gen.RemovedResult{}, platform.NewError(platform.CodeForbidden, false, "只有所有者能删除站点")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE sites SET status='deleted', revision=revision+1, updated_at=? WHERE site_id=?`, platform.Now(), input.SiteID); err != nil {
		return gen.RemovedResult{}, identityDBError()
	}
	return gen.RemovedResult{Removed: true}, nil
}

// ---- 成员 ----

func memberView(ctx context.Context, q sqlQuerier, siteID, userID string) (gen.SiteMember, error) {
	var m gen.SiteMember
	var role, created string
	err := q.QueryRowContext(ctx, `SELECT u.user_id,u.display_name,u.email,m.role,m.created_at FROM site_members m JOIN users u ON u.user_id=m.user_id WHERE m.site_id=? AND m.user_id=?`, siteID, userID).
		Scan(&m.UserId, &m.DisplayName, &m.Email, &role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return m, platform.NewError(platform.CodeNotFound, false, "成员不存在")
	}
	if err != nil {
		return m, identityDBError()
	}
	m.Role, m.CreatedAt = gen.SiteMemberRole(role), parseTS(created)
	return m, nil
}

func ListSiteMembers(ctx context.Context, op OperationContext, input SiteRef) (gen.ListSiteMembersResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListSiteMembersResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.ListSiteMembersResult{}, err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT user_id FROM site_members WHERE site_id=? ORDER BY created_at`, input.SiteID)
	if err != nil {
		return gen.ListSiteMembersResult{}, identityDBError()
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	result := gen.ListSiteMembersResult{Members: []gen.SiteMember{}}
	for _, id := range ids {
		m, err := memberView(ctx, store.db, input.SiteID, id)
		if err != nil {
			return result, err
		}
		result.Members = append(result.Members, m)
	}
	return result, nil
}

func AddSiteMember(ctx context.Context, op OperationContext, input AddSiteMemberCmd) (gen.SiteMemberResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteMemberResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteMemberResult{}, err
	}
	email, err := normalizeEmail(input.Body.Email)
	if err != nil {
		return gen.SiteMemberResult{}, err
	}
	var userID string
	if err := store.db.QueryRowContext(ctx, `SELECT user_id FROM users WHERE email=? AND status='active'`, email).Scan(&userID); err != nil {
		return gen.SiteMemberResult{}, platform.NewError(platform.CodeNotFound, false, "该邮箱还没有注册")
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO site_members(site_id,user_id,role,created_at) VALUES(?,?,?,?)`, input.SiteID, userID, string(input.Body.Role), platform.Now()); err != nil {
		if stringsContains(err.Error(), "UNIQUE constraint failed") {
			return gen.SiteMemberResult{}, platform.NewError(platform.CodeConflict, false, "已经是成员")
		}
		return gen.SiteMemberResult{}, identityDBError()
	}
	m, err := memberView(ctx, store.db, input.SiteID, userID)
	return gen.SiteMemberResult{Member: m}, err
}

func UpdateSiteMember(ctx context.Context, op OperationContext, input UpdateSiteMemberCmd) (gen.SiteMemberResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteMemberResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteMemberResult{}, err
	}
	var current string
	if err := store.db.QueryRowContext(ctx, `SELECT role FROM site_members WHERE site_id=? AND user_id=?`, input.SiteID, input.UserID).Scan(&current); err != nil {
		return gen.SiteMemberResult{}, platform.NewError(platform.CodeNotFound, false, "成员不存在")
	}
	if current == "owner" {
		return gen.SiteMemberResult{}, platform.NewError(platform.CodeForbidden, false, "不能改所有者的角色")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE site_members SET role=? WHERE site_id=? AND user_id=?`, string(input.Body.Role), input.SiteID, input.UserID); err != nil {
		return gen.SiteMemberResult{}, identityDBError()
	}
	if err := bumpPermissionRevision(ctx, store.db, input.UserID); err != nil {
		return gen.SiteMemberResult{}, err
	}
	m, err := memberView(ctx, store.db, input.SiteID, input.UserID)
	return gen.SiteMemberResult{Member: m}, err
}

func RemoveSiteMember(ctx context.Context, op OperationContext, input SiteUserRef) (gen.RemovedResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.RemovedResult{}, err
	}
	var current string
	if err := store.db.QueryRowContext(ctx, `SELECT role FROM site_members WHERE site_id=? AND user_id=?`, input.SiteID, input.UserID).Scan(&current); err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeNotFound, false, "成员不存在")
	}
	if current == "owner" {
		return gen.RemovedResult{}, platform.NewError(platform.CodeForbidden, false, "不能移除所有者")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM site_members WHERE site_id=? AND user_id=?`, input.SiteID, input.UserID); err != nil {
		return gen.RemovedResult{}, identityDBError()
	}
	if err := bumpPermissionRevision(ctx, store.db, input.UserID); err != nil {
		return gen.RemovedResult{}, err
	}
	return gen.RemovedResult{Removed: true}, nil
}

// ---- 域名 ----

func domainView(row struct {
	Host, Kind, Status, VerifyToken, CreatedAt string
	Primary                                    bool
	VerifiedAt                                 sql.NullString
}) gen.SiteDomain {
	d := gen.SiteDomain{Host: row.Host, Kind: gen.SiteDomainKind(row.Kind), Status: gen.SiteDomainStatus(row.Status), Primary: row.Primary,
		VerifyName: "_nw-verify." + row.Host, VerifyToken: "nw-verify=" + row.VerifyToken, CreatedAt: parseTS(row.CreatedAt)}
	if row.VerifiedAt.Valid {
		v := parseTS(row.VerifiedAt.String)
		d.VerifiedAt = &v
	}
	return d
}

func readDomain(ctx context.Context, q sqlQuerier, siteID, host string) (gen.SiteDomain, error) {
	var row struct {
		Host, Kind, Status, VerifyToken, CreatedAt string
		Primary                                    bool
		VerifiedAt                                 sql.NullString
	}
	err := q.QueryRowContext(ctx, `SELECT host,kind,status,verify_token,created_at,is_primary,verified_at FROM site_domains WHERE site_id=? AND host=?`, siteID, host).
		Scan(&row.Host, &row.Kind, &row.Status, &row.VerifyToken, &row.CreatedAt, &row.Primary, &row.VerifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.SiteDomain{}, platform.NewError(platform.CodeNotFound, false, "域名不存在")
	}
	if err != nil {
		return gen.SiteDomain{}, identityDBError()
	}
	return domainView(row), nil
}

func ListSiteDomains(ctx context.Context, op OperationContext, input SiteRef) (gen.ListSiteDomainsResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ListSiteDomainsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.ListSiteDomainsResult{}, err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT host FROM site_domains WHERE site_id=? ORDER BY is_primary DESC, created_at`, input.SiteID)
	if err != nil {
		return gen.ListSiteDomainsResult{}, identityDBError()
	}
	var hosts []string
	for rows.Next() {
		var h string
		if rows.Scan(&h) == nil {
			hosts = append(hosts, h)
		}
	}
	rows.Close()
	result := gen.ListSiteDomainsResult{Domains: []gen.SiteDomain{}}
	for _, h := range hosts {
		d, err := readDomain(ctx, store.db, input.SiteID, h)
		if err != nil {
			return result, err
		}
		result.Domains = append(result.Domains, d)
	}
	return result, nil
}

// AddSiteDomain 登记自定义域名，返回需要添加的 TXT 记录；验证通过后才会对外生效。
func AddSiteDomain(ctx context.Context, op OperationContext, input AddSiteDomainCmd) (gen.SiteDomainResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteDomainResult{}, err
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(input.Body.Host)), ".")
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.SiteDomainResult{}, err
	}
	if !hostPattern.MatchString(host) || net.ParseIP(host) != nil || len(host) > 253 {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeValidation, false, "域名格式不合法")
	}
	for _, own := range []string{settings.SitesDomain, settings.PlatformHost} {
		if own != "" && (host == own || strings.HasSuffix(host, "."+own)) {
			return gen.SiteDomainResult{}, platform.NewError(platform.CodeValidation, false, "不能绑定平台自己的域名")
		}
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO site_domains(host,site_id,kind,status,is_primary,verify_token,created_at) VALUES(?,?,'custom','pending',0,?,?)`,
		host, input.SiteID, randomCode(16), platform.Now()); err != nil {
		if stringsContains(err.Error(), "UNIQUE constraint failed") {
			return gen.SiteDomainResult{}, platform.NewError(platform.CodeConflict, false, "域名已被绑定")
		}
		return gen.SiteDomainResult{}, identityDBError()
	}
	d, err := readDomain(ctx, store.db, input.SiteID, host)
	return gen.SiteDomainResult{Domain: d}, err
}

// lookupTXT 可替换，便于离线环境；默认走系统解析器。
var lookupTXT = func(ctx context.Context, name string) ([]string, error) {
	return (&net.Resolver{}).LookupTXT(ctx, name)
}

// VerifySiteDomain 查 _nw-verify.<域名> 的 TXT 记录，匹配则标记已验证。
func VerifySiteDomain(ctx context.Context, op OperationContext, input SiteHostRef) (gen.SiteDomainResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteDomainResult{}, err
	}
	var token, kind string
	if err := store.db.QueryRowContext(ctx, `SELECT verify_token,kind FROM site_domains WHERE site_id=? AND host=?`, input.SiteID, input.Host).Scan(&token, &kind); err != nil {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeNotFound, false, "域名不存在")
	}
	if kind != "custom" {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeValidation, false, "子域名无需验证")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	records, _ := lookupTXT(lookupCtx, "_nw-verify."+input.Host)
	matched := false
	for _, record := range records {
		if strings.TrimSpace(record) == "nw-verify="+token {
			matched = true
		}
	}
	status := "failed"
	if matched {
		status = "verified"
		if _, err := store.db.ExecContext(ctx, `UPDATE site_domains SET status='verified', verified_at=? WHERE site_id=? AND host=?`, platform.Now(), input.SiteID, input.Host); err != nil {
			return gen.SiteDomainResult{}, identityDBError()
		}
	} else if _, err := store.db.ExecContext(ctx, `UPDATE site_domains SET status='failed' WHERE site_id=? AND host=?`, input.SiteID, input.Host); err != nil {
		return gen.SiteDomainResult{}, identityDBError()
	}
	_ = status
	d, err := readDomain(ctx, store.db, input.SiteID, input.Host)
	return gen.SiteDomainResult{Domain: d}, err
}

// SetPrimaryDomain 把已验证的域名设为主域名；其余域名访问时跳转到主域名。
func SetPrimaryDomain(ctx context.Context, op OperationContext, input SiteHostRef) (gen.SiteDomainResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.SiteDomainResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.SiteDomainResult{}, identityDBError()
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM site_domains WHERE site_id=? AND host=?`, input.SiteID, input.Host).Scan(&status); err != nil || status != "verified" {
		return gen.SiteDomainResult{}, platform.NewError(platform.CodeValidation, false, "只有已验证的域名能设为主域名")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE site_domains SET is_primary=0 WHERE site_id=?`, input.SiteID); err != nil {
		return gen.SiteDomainResult{}, identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE site_domains SET is_primary=1 WHERE site_id=? AND host=?`, input.SiteID, input.Host); err != nil {
		return gen.SiteDomainResult{}, identityDBError()
	}
	if err := tx.Commit(); err != nil {
		return gen.SiteDomainResult{}, identityDBError()
	}
	d, err := readDomain(ctx, store.db, input.SiteID, input.Host)
	return gen.SiteDomainResult{Domain: d}, err
}

func RemoveSiteDomain(ctx context.Context, op OperationContext, input SiteHostRef) (gen.RemovedResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.RemovedResult{}, err
	}
	var kind string
	var primary bool
	if err := store.db.QueryRowContext(ctx, `SELECT kind,is_primary FROM site_domains WHERE site_id=? AND host=?`, input.SiteID, input.Host).Scan(&kind, &primary); err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeNotFound, false, "域名不存在")
	}
	if kind == "subdomain" || primary {
		return gen.RemovedResult{}, platform.NewError(platform.CodeValidation, false, "平台子域名和主域名不能解绑")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM site_domains WHERE site_id=? AND host=?`, input.SiteID, input.Host); err != nil {
		return gen.RemovedResult{}, identityDBError()
	}
	return gen.RemovedResult{Removed: true}, nil
}

// ResolveHost 把访问域名解析成站点：只认已验证的域名；非主域名带上跳转目标。
// 开发环境里按 IP 或 localhost 访问时，落到最早创建的站点，保持单站点用法可用。
func ResolveHost(ctx context.Context, op OperationContext, input HostRef) (gen.ResolveHostResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ResolveHostResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	host := strings.ToLower(strings.TrimSuffix(input.Host, "."))
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	var result gen.ResolveHostResult
	var status, visibility string
	var primary bool
	err := store.db.QueryRowContext(ctx, `SELECT s.site_id,s.slug,s.status,s.visibility,d.is_primary,
        COALESCE((SELECT host FROM site_domains WHERE site_id=s.site_id AND is_primary=1 AND status='verified' LIMIT 1),'')
        FROM site_domains d JOIN sites s ON s.site_id=d.site_id WHERE d.host=? AND d.status='verified'`, host).
		Scan(&result.SiteId, &result.Slug, &status, &visibility, &primary, &result.PrimaryHost)
	if errors.Is(err, sql.ErrNoRows) {
		if net.ParseIP(host) == nil && host != "localhost" && !strings.HasSuffix(host, ".localhost") {
			return result, platform.NewError(platform.CodeNotFound, false, "域名没有对应站点")
		}
		err = store.db.QueryRowContext(ctx, `SELECT site_id,slug,status,visibility,'' FROM sites WHERE status<>'deleted' ORDER BY created_at LIMIT 1`).
			Scan(&result.SiteId, &result.Slug, &status, &visibility, &result.PrimaryHost)
		primary = true
		if errors.Is(err, sql.ErrNoRows) {
			return result, platform.NewError(platform.CodeNotFound, false, "还没有站点")
		}
	}
	if err != nil {
		return result, identityDBError()
	}
	result.Status, result.Visibility = gen.ResolveHostResultStatus(status), gen.ResolveHostResultVisibility(visibility)
	if settings, err := loadSettings(ctx, store.db); err == nil {
		result.PlatformHost = settings.PlatformHost
	}
	if !primary && result.PrimaryHost != "" && result.PrimaryHost != host {
		redirect := result.PrimaryHost
		result.RedirectHost = &redirect
	}
	return result, nil
}

// AllowTlsHost 回答入口 Caddy 的“按需证书”询问：只给平台域名和已验证的站点域名（含平台子域名）签证书，防止被人拿任意域名消耗签发额度。
func AllowTlsHost(ctx context.Context, op OperationContext, input HostRef) (gen.TlsHostResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.TlsHostResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	host := strings.ToLower(strings.TrimSuffix(input.Host, "."))
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.TlsHostResult{}, err
	}
	if settings.PlatformHost != "" && host == strings.ToLower(settings.PlatformHost) {
		return gen.TlsHostResult{Allowed: true}, nil
	}
	var n int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM site_domains d JOIN sites s ON s.site_id=d.site_id WHERE d.host=? AND d.status='verified' AND s.status<>'deleted'`, host).Scan(&n); err != nil {
		return gen.TlsHostResult{}, identityDBError()
	}
	return gen.TlsHostResult{Allowed: n > 0}, nil
}

// IssuePrivateAccess 给站点成员发一个 60 秒内有效的一次性码，由前端带去站点域名换取该站点专用的短期访问。
func IssuePrivateAccess(ctx context.Context, op OperationContext, input SiteRef) (gen.PrivateAccessResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.PrivateAccessResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	if err := requireSiteScope(op, input.SiteID); err != nil {
		return gen.PrivateAccessResult{}, err
	}
	if _, err := memberRole(ctx, store.db, op, input.SiteID); err != nil {
		return gen.PrivateAccessResult{}, err
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.PrivateAccessResult{}, err
	}
	code := "nwp-" + randomCode(16)
	expires := time.Now().UTC().Add(60 * time.Second).Format(time.RFC3339Nano)
	if _, err := store.db.ExecContext(ctx, `DELETE FROM private_access_codes WHERE expires_at<?`, platform.Now()); err != nil {
		return gen.PrivateAccessResult{}, identityDBError()
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO private_access_codes(code_hash,site_id,user_id,expires_at) VALUES(?,?,?,?)`, hashCode(code), input.SiteID, userID, expires); err != nil {
		return gen.PrivateAccessResult{}, identityDBError()
	}
	summary, err := siteSummary(ctx, store.db, input.SiteID, "")
	if err != nil {
		return gen.PrivateAccessResult{}, err
	}
	return gen.PrivateAccessResult{Code: code, Host: summary.PrimaryHost}, nil
}

// RedeemPrivateAccess 由入口在站点域名上调用：码有效、没过期、属于这个站点才换成短期访问，码只能用一次。
func RedeemPrivateAccess(ctx context.Context, op OperationContext, input RedeemPrivateAccessCmd) (gen.PrivateAccessGrant, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.PrivateAccessGrant{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.PrivateAccessGrant{}, identityDBError()
	}
	defer tx.Rollback()
	var siteID, expires string
	err = tx.QueryRowContext(ctx, `SELECT site_id,expires_at FROM private_access_codes WHERE code_hash=?`, hashCode(input.Body.Code)).Scan(&siteID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.PrivateAccessGrant{}, platform.NewError(platform.CodeForbidden, false, "进入私有站点的码无效或已过期")
	}
	if err != nil {
		return gen.PrivateAccessGrant{}, identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM private_access_codes WHERE code_hash=?`, hashCode(input.Body.Code)); err != nil {
		return gen.PrivateAccessGrant{}, identityDBError()
	}
	if err := tx.Commit(); err != nil {
		return gen.PrivateAccessGrant{}, identityDBError()
	}
	if siteID != input.SiteID || !parseTS(expires).After(time.Now().UTC()) {
		return gen.PrivateAccessGrant{}, platform.NewError(platform.CodeForbidden, false, "进入私有站点的码无效或已过期")
	}
	return gen.PrivateAccessGrant{TtlSeconds: 12 * 3600}, nil
}
