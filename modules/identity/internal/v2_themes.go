package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/theme"
	"nice-wiki/modules/identity/internal/gen"
)

// 外观市场（docs/theme-standard.md）：登录用户提交外观包，平台管理员审核通过后上架，任何站点（包括单站点版）都能浏览和下载。
// 市场只管目录与审核；站点安装时把包复制到自己的 site-config，之后市场下架不影响已装的站点。

// maxPendingThemes 是每个用户同时待审核的提交数上限。
const maxPendingThemes = 5

// ThemeVersionRef 指向市场里的一个外观版本。
type ThemeVersionRef struct{ ThemeID, Version string }

type ReviewThemeCmd struct {
	ThemeID, Version string
	Body             gen.ReviewThemeInput
}

func themeStore(ctx context.Context) (*Store, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	return store, nil
}

// ListThemeMarket 列出上架的外观：每个 id 取版本号最大的已通过版本，按上架时间倒序。
func ListThemeMarket(ctx context.Context, op OperationContext, _ struct{}) (gen.ListThemeMarketResult, error) {
	store, err := themeStore(ctx)
	if err != nil {
		return gen.ListThemeMarketResult{}, err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT v.theme_id, v.version, v.digest, v.manifest, v.byte_size, COALESCE(v.reviewed_at, v.submitted_at), u.display_name
		FROM market_theme_versions v JOIN market_themes t ON t.theme_id = v.theme_id JOIN users u ON u.user_id = t.owner_user_id
		WHERE v.status = 'approved' AND u.status = 'active'`)
	if err != nil {
		return gen.ListThemeMarketResult{}, identityDBError()
	}
	defer rows.Close()
	latest := map[string]gen.MarketTheme{}
	for rows.Next() {
		var item gen.MarketTheme
		var manifest, published string
		if err := rows.Scan(&item.ThemeId, &item.Version, &item.Digest, &manifest, &item.ByteSize, &published, &item.Publisher); err != nil {
			return gen.ListThemeMarketResult{}, identityDBError()
		}
		_ = json.Unmarshal([]byte(manifest), &item.Manifest)
		item.PublishedAt, _ = time.Parse(time.RFC3339Nano, published)
		if current, ok := latest[item.ThemeId]; !ok || versionLess(current.Version, item.Version) {
			latest[item.ThemeId] = item
		}
	}
	if err := rows.Err(); err != nil {
		return gen.ListThemeMarketResult{}, identityDBError()
	}
	result := gen.ListThemeMarketResult{Themes: make([]gen.MarketTheme, 0, len(latest))}
	for _, item := range latest {
		result.Themes = append(result.Themes, item)
	}
	sort.Slice(result.Themes, func(i, j int) bool { return result.Themes[i].PublishedAt.After(result.Themes[j].PublishedAt) })
	return result, nil
}

// versionLess 按 主.次.修订 的数值比较（版本号格式已在提交时校验）。
func versionLess(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := range left {
		x, _ := strconv.Atoi(left[i])
		y, _ := strconv.Atoi(right[i])
		if x != y {
			return x < y
		}
	}
	return false
}

// ReadMarketThemePackage 下载已上架的外观包。
func ReadMarketThemePackage(ctx context.Context, ref ThemeVersionRef) ([]byte, error) {
	store, err := themeStore(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = store.db.QueryRowContext(ctx, `SELECT package FROM market_theme_versions WHERE theme_id = ? AND version = ? AND status = 'approved'`, ref.ThemeID, ref.Version).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, platform.NewError(platform.CodeNotFound, false, "市场里没有这个外观版本")
	}
	if err != nil {
		return nil, identityDBError()
	}
	return raw, nil
}

// ReadThemeSubmissionPackage 给平台管理员下载任意状态的提交，审核前检查样式表与资源。
func ReadThemeSubmissionPackage(ctx context.Context, ref ThemeVersionRef) ([]byte, error) {
	store, err := themeStore(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = store.db.QueryRowContext(ctx, `SELECT package FROM market_theme_versions WHERE theme_id = ? AND version = ?`, ref.ThemeID, ref.Version).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, platform.NewError(platform.CodeNotFound, false, "没有这个外观版本")
	}
	if err != nil {
		return nil, identityDBError()
	}
	return raw, nil
}

// ReadMarketThemePreview 返回已上架外观的预览图与媒体类型；外观没有预览图时 NOT_FOUND。
func ReadMarketThemePreview(ctx context.Context, ref ThemeVersionRef) ([]byte, string, error) {
	raw, err := ReadMarketThemePackage(ctx, ref)
	if err != nil {
		return nil, "", err
	}
	pkg, err := theme.Parse(raw)
	if err != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "市场里的外观包已损坏")
	}
	name := pkg.Manifest.Preview
	if name == "" {
		return nil, "", platform.NewError(platform.CodeNotFound, false, "这个外观没有预览图")
	}
	return pkg.Files[name], theme.ContentType(name), nil
}

// SubmitTheme 校验外观包并提交审核。外观 id 第一次出现时归提交者所有；同一版本只能提交一次。
func SubmitTheme(ctx context.Context, op OperationContext, raw []byte) (gen.ThemeSubmission, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.ThemeSubmission{}, err
	}
	store, err := themeStore(ctx)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	pkg, err := theme.Parse(raw)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	id, version := pkg.Manifest.ID, pkg.Manifest.Version
	if theme.Reserved(id) {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeValidation, false, "外观 id「%s」是内置外观，请在 theme.json 里换一个 id", id)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.ExecContext(ctx, `INSERT INTO market_themes(theme_id, owner_user_id, created_at) VALUES(?, ?, ?) ON CONFLICT(theme_id) DO NOTHING`, id, userID, now); err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	var owner string
	if err := store.db.QueryRowContext(ctx, `SELECT owner_user_id FROM market_themes WHERE theme_id = ?`, id).Scan(&owner); err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	if owner != userID {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeConflict, false, "外观 id「%s」已被别人使用，请换一个 id", id)
	}
	var pending int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM market_theme_versions v JOIN market_themes t ON t.theme_id = v.theme_id WHERE t.owner_user_id = ? AND v.status = 'pending'`, userID).Scan(&pending); err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	if pending >= maxPendingThemes {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeConflict, false, "已有 %d 个外观在等待审核，审核完再提交", maxPendingThemes)
	}
	result, err := store.db.ExecContext(ctx, `INSERT INTO market_theme_versions(theme_id, version, digest, manifest, package, byte_size, status, submitted_at)
		VALUES(?, ?, ?, ?, ?, ?, 'pending', ?) ON CONFLICT(theme_id, version) DO NOTHING`, id, version, pkg.Digest, string(pkg.Files["theme.json"]), pkg.Zip(), pkg.Size(), now)
	if err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeConflict, false, "%s %s 已经提交过，请在 theme.json 里升版本号", id, version)
	}
	return themeSubmission(ctx, store.db, ThemeVersionRef{id, version})
}

// ListMyThemeSubmissions 列出我提交过的全部版本（新的在前）。
func ListMyThemeSubmissions(ctx context.Context, op OperationContext, _ struct{}) (gen.ListThemeSubmissionsResult, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.ListThemeSubmissionsResult{}, err
	}
	store, err := themeStore(ctx)
	if err != nil {
		return gen.ListThemeSubmissionsResult{}, err
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.ListThemeSubmissionsResult{}, err
	}
	return themeSubmissions(ctx, store.db, `WHERE t.owner_user_id = ? ORDER BY v.submitted_at DESC LIMIT 200`, userID)
}

// ListThemeReviews 给平台管理员：待审核的在前，其余按提交时间倒序。
func ListThemeReviews(ctx context.Context, op OperationContext, _ struct{}) (gen.ListThemeSubmissionsResult, error) {
	store, err := themeStore(ctx)
	if err != nil {
		return gen.ListThemeSubmissionsResult{}, err
	}
	return themeSubmissions(ctx, store.db, `ORDER BY (v.status = 'pending') DESC, v.submitted_at DESC LIMIT 200`)
}

// ReviewTheme 通过（上架）或驳回一个版本；驳回必须写原因，提交者在“我的外观”里能看到。
func ReviewTheme(ctx context.Context, op OperationContext, cmd ReviewThemeCmd) (gen.ThemeSubmission, error) {
	store, err := themeStore(ctx)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	note := ""
	if cmd.Body.Note != nil {
		note = strings.TrimSpace(*cmd.Body.Note)
	}
	status := "approved"
	if cmd.Body.Decision == gen.Reject {
		status = "rejected"
		if note == "" {
			return gen.ThemeSubmission{}, platform.NewError(platform.CodeValidation, false, "驳回时请写明原因")
		}
	}
	result, err := store.db.ExecContext(ctx, `UPDATE market_theme_versions SET status = ?, review_note = ?, reviewed_at = ? WHERE theme_id = ? AND version = ?`,
		status, note, time.Now().UTC().Format(time.RFC3339Nano), cmd.ThemeID, cmd.Version)
	if err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeNotFound, false, "没有这个外观版本")
	}
	return themeSubmission(ctx, store.db, ThemeVersionRef{cmd.ThemeID, cmd.Version})
}

// WithdrawTheme 删除市场里的一个版本：提交者本人撤回，或平台管理员下架。外观 id 仍归原提交者。
func WithdrawTheme(ctx context.Context, op OperationContext, ref ThemeVersionRef) (gen.ThemeSubmission, error) {
	if err := requireAdminActor(op); err != nil {
		return gen.ThemeSubmission{}, err
	}
	store, err := themeStore(ctx)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	submission, err := themeSubmission(ctx, store.db, ref)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	var owner string
	var superadmin bool
	if err := store.db.QueryRowContext(ctx, `SELECT t.owner_user_id, a.is_superadmin FROM market_themes t, accounts a WHERE t.theme_id = ? AND a.account_id = ?`, ref.ThemeID, op.Actor.SubjectID).Scan(&owner, &superadmin); err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	if owner != userID && !superadmin {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeForbidden, false, "只有提交者或平台管理员能撤下这个外观")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM market_theme_versions WHERE theme_id = ? AND version = ?`, ref.ThemeID, ref.Version); err != nil {
		return gen.ThemeSubmission{}, identityDBError()
	}
	return submission, nil
}

const submissionColumns = `SELECT v.theme_id, v.version, v.digest, v.manifest, v.byte_size, u.display_name, v.status, v.review_note, v.submitted_at, v.reviewed_at
	FROM market_theme_versions v JOIN market_themes t ON t.theme_id = v.theme_id JOIN users u ON u.user_id = t.owner_user_id `

func themeSubmission(ctx context.Context, q sqlQuerier, ref ThemeVersionRef) (gen.ThemeSubmission, error) {
	list, err := themeSubmissions(ctx, q, `WHERE v.theme_id = ? AND v.version = ?`, ref.ThemeID, ref.Version)
	if err != nil {
		return gen.ThemeSubmission{}, err
	}
	if len(list.Submissions) == 0 {
		return gen.ThemeSubmission{}, platform.NewError(platform.CodeNotFound, false, "没有这个外观版本")
	}
	return list.Submissions[0], nil
}

func themeSubmissions(ctx context.Context, q sqlQuerier, where string, args ...any) (gen.ListThemeSubmissionsResult, error) {
	rows, err := q.QueryContext(ctx, submissionColumns+where, args...)
	if err != nil {
		return gen.ListThemeSubmissionsResult{}, identityDBError()
	}
	defer rows.Close()
	result := gen.ListThemeSubmissionsResult{Submissions: []gen.ThemeSubmission{}}
	for rows.Next() {
		var item gen.ThemeSubmission
		var manifest, status, submitted string
		var reviewed sql.NullString
		if err := rows.Scan(&item.ThemeId, &item.Version, &item.Digest, &manifest, &item.ByteSize, &item.Publisher, &status, &item.ReviewNote, &submitted, &reviewed); err != nil {
			return gen.ListThemeSubmissionsResult{}, identityDBError()
		}
		_ = json.Unmarshal([]byte(manifest), &item.Manifest)
		item.Status = gen.ThemeSubmissionStatus(status)
		item.SubmittedAt, _ = time.Parse(time.RFC3339Nano, submitted)
		if reviewed.Valid {
			at, _ := time.Parse(time.RFC3339Nano, reviewed.String)
			item.ReviewedAt = &at
		}
		result.Submissions = append(result.Submissions, item)
	}
	if err := rows.Err(); err != nil {
		return gen.ListThemeSubmissionsResult{}, identityDBError()
	}
	return result, nil
}
