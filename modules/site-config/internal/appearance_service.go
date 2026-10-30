package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/theme"
	"nice-wiki/modules/site-config/internal/gen"
)

// 站点外观（docs/theme-standard.md）：内置外观来自 libs/platform/theme；站点上传或从市场装的外观包存在 theme_packages，
// 当前选择存在 site_appearance（没有记录时用默认外观、访客明暗跟随系统）。公开站按摘要取包渲染样式。

// maxInstalledThemes 是每个站点最多装的外观包数（内置外观不算）。
const maxInstalledThemes = 20

func appearanceStore() (*ActiveStore, error) {
	store := currentActiveStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	return store, nil
}

func appearanceStorageError(err error) error {
	return platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "外观数据不可用")
}

// ReadAppearance 返回当前选择和全部可选外观（内置在前，本站装的按安装时间倒序）。
func ReadAppearance(ctx context.Context, op platform.OperationContext, _ struct{}) (gen.ReadAppearanceResult, error) {
	store, err := appearanceStore()
	if err != nil {
		return gen.ReadAppearanceResult{}, err
	}
	current, err := store.readAppearance(ctx)
	if err != nil {
		return gen.ReadAppearanceResult{}, err
	}
	themes := make([]gen.ThemeEntry, 0, len(theme.Builtins()))
	for _, pkg := range theme.Builtins() {
		themes = append(themes, gen.ThemeEntry{Digest: pkg.Digest, Builtin: true, ByteSize: pkg.Size(), Manifest: pkg.ManifestObject()})
	}
	rows, err := store.db.QueryContext(ctx, `SELECT digest, manifest, byte_size, installed_at FROM theme_packages ORDER BY installed_at DESC`)
	if err != nil {
		return gen.ReadAppearanceResult{}, appearanceStorageError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var digest, manifest, installed string
		var size int
		if err := rows.Scan(&digest, &manifest, &size, &installed); err != nil {
			return gen.ReadAppearanceResult{}, appearanceStorageError(err)
		}
		entry := gen.ThemeEntry{Digest: digest, ByteSize: size, Manifest: map[string]any{}}
		_ = json.Unmarshal([]byte(manifest), &entry.Manifest)
		if at, err := time.Parse(time.RFC3339Nano, installed); err == nil {
			entry.InstalledAt = &at
		}
		themes = append(themes, entry)
	}
	if err := rows.Err(); err != nil {
		return gen.ReadAppearanceResult{}, appearanceStorageError(err)
	}
	return gen.ReadAppearanceResult{Appearance: current, Themes: themes}, nil
}

// UpdateAppearance 切换外观或访客默认明暗；选的外观必须是内置的或本站已装的。
func UpdateAppearance(ctx context.Context, op platform.OperationContext, input gen.UpdateAppearanceInput) (gen.Appearance, error) {
	store, err := appearanceStore()
	if err != nil {
		return gen.Appearance{}, err
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.Appearance{}, err
	}
	id := ""
	if pkg := theme.Builtin(input.ThemeDigest); pkg != nil {
		id = pkg.Manifest.ID
	} else if err := store.db.QueryRowContext(ctx, `SELECT theme_id FROM theme_packages WHERE digest = ?`, input.ThemeDigest).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return gen.Appearance{}, platform.NewError(platform.CodeNotFound, false, "这个外观没有装在本站")
	} else if err != nil {
		return gen.Appearance{}, appearanceStorageError(err)
	}
	now := time.Now().UTC()
	if _, err := store.db.ExecContext(ctx, `INSERT INTO site_appearance(site_id, theme_id, theme_digest, color_scheme, updated_at) VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(site_id) DO UPDATE SET theme_id = excluded.theme_id, theme_digest = excluded.theme_digest, color_scheme = excluded.color_scheme, updated_at = excluded.updated_at`,
		string(op.Actor.SiteID), id, input.ThemeDigest, string(input.ColorScheme), now.Format(time.RFC3339Nano)); err != nil {
		return gen.Appearance{}, appearanceStorageError(err)
	}
	return gen.Appearance{ThemeDigest: input.ThemeDigest, ColorScheme: gen.AppearanceColorScheme(input.ColorScheme), UpdatedAt: &now}, nil
}

// InstallTheme 校验上传的外观包并装到本站；同一内容重复上传得到同一条记录。
func InstallTheme(ctx context.Context, op platform.OperationContext, raw []byte) (gen.InstallThemeResult, error) {
	store, err := appearanceStore()
	if err != nil {
		return gen.InstallThemeResult{}, err
	}
	pkg, err := theme.Parse(raw)
	if err != nil {
		return gen.InstallThemeResult{}, err
	}
	if builtin := theme.Builtin(pkg.Digest); builtin != nil {
		return gen.InstallThemeResult{Theme: gen.ThemeEntry{Digest: builtin.Digest, Builtin: true, ByteSize: builtin.Size(), Manifest: builtin.ManifestObject()}}, nil
	}
	if theme.Reserved(pkg.Manifest.ID) {
		return gen.InstallThemeResult{}, platform.NewError(platform.CodeValidation, false, "外观 id「%s」和内置外观重复，请在 theme.json 里换一个 id", pkg.Manifest.ID)
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.InstallThemeResult{}, err
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM theme_packages WHERE digest <> ?`, pkg.Digest).Scan(&count); err != nil {
		return gen.InstallThemeResult{}, appearanceStorageError(err)
	}
	if count >= maxInstalledThemes {
		return gen.InstallThemeResult{}, platform.NewError(platform.CodeConflict, false, "每个站点最多装 %d 个外观包，先删掉不用的", maxInstalledThemes)
	}
	now := time.Now().UTC()
	if _, err := store.db.ExecContext(ctx, `INSERT INTO theme_packages(site_id, digest, theme_id, version, manifest, package, byte_size, installed_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id, digest) DO NOTHING`, string(op.Actor.SiteID), pkg.Digest, pkg.Manifest.ID, pkg.Manifest.Version, string(pkg.Files["theme.json"]), pkg.Zip(), pkg.Size(), now.Format(time.RFC3339Nano)); err != nil {
		return gen.InstallThemeResult{}, appearanceStorageError(err)
	}
	return gen.InstallThemeResult{Theme: gen.ThemeEntry{Digest: pkg.Digest, ByteSize: pkg.Size(), InstalledAt: &now, Manifest: pkg.ManifestObject()}}, nil
}

// DeleteTheme 删掉本站装的外观包；内置外观和正在用的外观不能删。
func DeleteTheme(ctx context.Context, op platform.OperationContext, digest string) (gen.DeleteThemeResult, error) {
	store, err := appearanceStore()
	if err != nil {
		return gen.DeleteThemeResult{}, err
	}
	if theme.Builtin(digest) != nil {
		return gen.DeleteThemeResult{}, platform.NewError(platform.CodeValidation, false, "内置外观不能删除")
	}
	current, err := store.readAppearance(ctx)
	if err != nil {
		return gen.DeleteThemeResult{}, err
	}
	if current.ThemeDigest == digest {
		return gen.DeleteThemeResult{}, platform.NewError(platform.CodeConflict, false, "正在使用的外观不能删除，先换成别的外观")
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.DeleteThemeResult{}, err
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM theme_packages WHERE digest = ?`, digest); err != nil {
		return gen.DeleteThemeResult{}, appearanceStorageError(err)
	}
	return gen.DeleteThemeResult{Deleted: digest}, nil
}

// ReadPublicAppearance 给公开站：当前外观的摘要（一定能取到包）与访客默认明暗。
func ReadPublicAppearance(ctx context.Context, op platform.OperationContext, _ struct{}) (gen.PublicAppearance, error) {
	store, err := appearanceStore()
	if err != nil {
		return gen.PublicAppearance{}, err
	}
	current, err := store.readAppearance(ctx)
	if err != nil {
		return gen.PublicAppearance{}, err
	}
	return gen.PublicAppearance{ThemeDigest: current.ThemeDigest, ColorScheme: gen.PublicAppearanceColorScheme(current.ColorScheme)}, nil
}

// ReadThemePackage 按摘要返回外观包 zip（内置外观现打包）。
func ReadThemePackage(ctx context.Context, op platform.OperationContext, digest string) ([]byte, error) {
	if pkg := theme.Builtin(digest); pkg != nil {
		return pkg.Zip(), nil
	}
	store, err := appearanceStore()
	if err != nil {
		return nil, err
	}
	var raw []byte
	if err := store.db.QueryRowContext(ctx, `SELECT package FROM theme_packages WHERE digest = ?`, digest).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return nil, platform.NewError(platform.CodeNotFound, false, "外观包不存在")
	} else if err != nil {
		return nil, appearanceStorageError(err)
	}
	return raw, nil
}

// readAppearance 读当前选择，并换成现在可用的包：内置外观随程序升级换了内容时按 id 接上新版；
// 找不到（不该发生，删除时已挡住正在用的）就回到默认外观。
func (s *ActiveStore) readAppearance(ctx context.Context) (gen.Appearance, error) {
	var id, digest, scheme, updated string
	err := s.db.QueryRowContext(ctx, `SELECT theme_id, theme_digest, color_scheme, updated_at FROM site_appearance`).Scan(&id, &digest, &scheme, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Appearance{ThemeDigest: theme.Default().Digest, ColorScheme: gen.AppearanceColorSchemeSystem}, nil
	}
	if err != nil {
		return gen.Appearance{}, appearanceStorageError(err)
	}
	result := gen.Appearance{ThemeDigest: digest, ColorScheme: gen.AppearanceColorScheme(scheme)}
	if at, err := time.Parse(time.RFC3339Nano, updated); err == nil {
		result.UpdatedAt = &at
	}
	if theme.Builtin(digest) != nil {
		return result, nil
	}
	var found int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM theme_packages WHERE digest = ?`, digest).Scan(&found); err != nil {
		return gen.Appearance{}, appearanceStorageError(err)
	}
	if found > 0 {
		return result, nil
	}
	result.ThemeDigest = theme.Default().Digest
	for _, pkg := range theme.Builtins() {
		if pkg.Manifest.ID == id {
			result.ThemeDigest = pkg.Digest
		}
	}
	return result, nil
}
