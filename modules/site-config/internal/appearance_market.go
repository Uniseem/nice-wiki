package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/theme"
	"nice-wiki/modules/site-config/internal/gen"
)

// 外观市场由多用户平台提供（docs/theme-standard.md 第 6 节）。单站点版只存市场地址，由本服务去拉列表、预览图和外观包，
// 外观包装进本站前照上传一样完整校验；浏览器不直接访问市场（后台的内容安全策略也不允许）。

var marketClient = &http.Client{Timeout: 20 * time.Second}

// ReadThemeMarket 返回市场地址和市场里已上架的外观；没配地址时列表为空，拉取失败时在 problem 里说明。
func ReadThemeMarket(ctx context.Context, op platform.OperationContext, _ struct{}) (gen.ReadThemeMarketResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.ReadThemeMarketResult{}, err
	}
	store, err := appearanceStore()
	if err != nil {
		return gen.ReadThemeMarketResult{}, err
	}
	market, err := store.marketURL(ctx)
	if err != nil {
		return gen.ReadThemeMarketResult{}, err
	}
	result := gen.ReadThemeMarketResult{MarketUrl: market, Themes: []gen.MarketTheme{}}
	if market == "" {
		return result, nil
	}
	// 市场拉不到时照常返回地址并说明原因，后台可以改地址。
	raw, _, err := fetchMarket(ctx, market+"/v1/theme-market", 2<<20)
	var failure *platform.Error
	if errors.As(err, &failure) {
		result.Problem = &failure.Message
		return result, nil
	}
	var list struct {
		Themes []gen.MarketTheme `json:"themes"`
	}
	if json.Unmarshal(raw, &list) != nil {
		problem := "外观市场返回的数据无法识别，请检查市场地址"
		result.Problem = &problem
		return result, nil
	}
	if list.Themes != nil {
		result.Themes = list.Themes
	}
	return result, nil
}

// UpdateThemeMarket 保存市场地址（多用户平台的地址，如 https://wiki.example.com）；空字符串表示不用市场。
func UpdateThemeMarket(ctx context.Context, op platform.OperationContext, input gen.UpdateThemeMarketInput) (gen.UpdateThemeMarketResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.UpdateThemeMarketResult{}, err
	}
	store, err := appearanceStore()
	if err != nil {
		return gen.UpdateThemeMarketResult{}, err
	}
	market := strings.TrimRight(strings.TrimSpace(input.MarketUrl), "/")
	if market != "" {
		parsed, err := url.Parse(market)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return gen.UpdateThemeMarketResult{}, platform.NewError(platform.CodeValidation, false, "市场地址要写成 https://域名 的形式")
		}
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.UpdateThemeMarketResult{}, err
	}
	pkg := theme.Default()
	if _, err := store.db.ExecContext(ctx, `INSERT INTO site_appearance(id, theme_id, theme_digest, color_scheme, market_url, updated_at) VALUES(1, ?, ?, 'system', ?, ?)
		ON CONFLICT(id) DO UPDATE SET market_url = excluded.market_url`, pkg.Manifest.ID, pkg.Digest, market, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return gen.UpdateThemeMarketResult{}, appearanceStorageError(err)
	}
	return gen.UpdateThemeMarketResult{MarketUrl: market}, nil
}

// ReadMarketThemePreview 代取市场里某个外观版本的预览图，返回图片字节与类型。
func ReadMarketThemePreview(ctx context.Context, op platform.OperationContext, themeID, version string) ([]byte, string, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return nil, "", err
	}
	market, err := configuredMarket(ctx)
	if err != nil {
		return nil, "", err
	}
	raw, header, err := fetchMarket(ctx, marketVersionURL(market, themeID, version)+"/preview", theme.MaxFileBytes)
	if err != nil {
		return nil, "", err
	}
	contentType := header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" {
		contentType = "application/octet-stream"
	}
	return raw, contentType, nil
}

// InstallMarketTheme 从市场下载外观包，核对 id、版本和摘要与市场列表一致后装到本站。
func InstallMarketTheme(ctx context.Context, op platform.OperationContext, input gen.InstallMarketThemeInput) (gen.InstallThemeResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.InstallThemeResult{}, err
	}
	market, err := configuredMarket(ctx)
	if err != nil {
		return gen.InstallThemeResult{}, err
	}
	raw, _, err := fetchMarket(ctx, marketVersionURL(market, input.ThemeId, input.Version)+"/package", theme.MaxPackageBytes)
	if err != nil {
		return gen.InstallThemeResult{}, err
	}
	pkg, err := theme.Parse(raw)
	if err != nil {
		return gen.InstallThemeResult{}, err
	}
	if pkg.Digest != input.Digest || pkg.Manifest.ID != input.ThemeId || pkg.Manifest.Version != input.Version {
		return gen.InstallThemeResult{}, platform.NewError(platform.CodeConflict, false, "市场里的外观包和列表对不上，请刷新后再装")
	}
	return installPackage(ctx, op, pkg)
}

func configuredMarket(ctx context.Context) (string, error) {
	store, err := appearanceStore()
	if err != nil {
		return "", err
	}
	market, err := store.marketURL(ctx)
	if err != nil {
		return "", err
	}
	if market == "" {
		return "", platform.NewError(platform.CodeValidation, false, "还没有填外观市场地址")
	}
	return market, nil
}

func marketVersionURL(market, themeID, version string) string {
	return market + "/v1/theme-market/" + url.PathEscape(themeID) + "/" + url.PathEscape(version)
}

// fetchMarket 向市场发 GET，读不超过 limit 字节。
func fetchMarket(ctx context.Context, target string, limit int64) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "市场地址不可用")
	}
	response, err := marketClient.Do(request)
	if err != nil {
		return nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "连不上外观市场，请检查市场地址或网络")
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return nil, nil, platform.NewError(platform.CodeNotFound, false, "外观市场里没有这个外观")
	case response.StatusCode != http.StatusOK:
		return nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "外观市场暂时不可用（HTTP %d）", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取外观市场失败")
	}
	if int64(len(raw)) > limit {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "外观市场返回的内容超出上限")
	}
	return raw, response.Header, nil
}

func (s *ActiveStore) marketURL(ctx context.Context) (string, error) {
	var market string
	err := s.db.QueryRowContext(ctx, `SELECT market_url FROM site_appearance WHERE id = 1`).Scan(&market)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", appearanceStorageError(err)
	}
	return market, nil
}
