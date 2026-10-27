// Package app 装配 portal 的入口路由、运行状态客户端和 HTTP 容器。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform/apivalidate"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/geoip"
	"nice-wiki/libs/platform/lifecycle"
	portal "nice-wiki/modules/portal/internal"
	"nice-wiki/modules/portal/internal/gen"
	"nice-wiki/modules/portal/internal/transport"
)

const ModuleSlug = "portal"

// BuildApplication 完成 portal 无状态入口的装配。入口没有业务DB，
// active/control/restore 目录仅为部署布局保留。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	publicBaseURL, err := lifecycle.NormalizePublicURL(config.PublicBaseURL, config.AllowLocalHTTP)
	if err != nil {
		return nil, err
	}
	config.PublicBaseURL = publicBaseURL
	if err := ensureDataLayout(config.DataRoot); err != nil {
		return nil, err
	}
	anchors, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	delegationKey, err := lifecycle.LoadPortalDelegation(config, anchors)
	if err != nil {
		return nil, err
	}
	if anchors.DelegationKeyID(delegationKey.KeyID) == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "入口委托密钥未登记在专用信任锚")
	}
	runtimeClient, err := NewRuntimeClient(config)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeClient)
	if err != nil {
		return nil, err
	}
	routes, err := loadRouteRegistry()
	if err != nil {
		return nil, err
	}
	var csrfVerifier portal.IdentityVerifier
	var authResolver portal.AuthResolver
	var identityClient *gen.IdentityClient
	if base := config.Dependencies["identity"]; base != "" {
		client := gen.NewIdentityClient(base, ModuleSlug, key)
		identityClient = client
		csrfVerifier = portal.CSRFVerifier{Client: client}
		authResolver = portal.IdentityAuthResolver{Client: client}
	}
	siteID, err := readSiteIdentity(ctx, config, key, view.DataEpoch)
	var failure *platform.Error
	if err != nil && !(errors.As(err, &failure) && failure.Code == platform.CodeNotFound) {
		return nil, err
	}
	application, err := transport.NewApplication(config, anchors, key, delegationKey, runtimeClient, csrfVerifier, authResolver, routes, view.DataEpoch, siteID)
	if err != nil {
		return nil, err
	}
	application.SetSiteLoader(func(work context.Context) (platform.Identifier, error) {
		return readSiteIdentity(work, config, key, view.DataEpoch)
	})
	if identityClient != nil {
		application.SetHostResolver(hostResolver(identityClient, view.DataEpoch))
		application.SetTLSAllow(tlsAllower(identityClient, view.DataEpoch))
		application.SetPrivateRedeemer(privateRedeemer(identityClient, view.DataEpoch))
	}
	// 部署时 portal 前面有 edge（页面缓存）：发布等写入成功后通知它清这个站点的缓存。
	application.SetEdgePurge(os.Getenv("NW_EDGE_PURGE_URL"), os.Getenv("NW_EDGE_PURGE_SECRET_FILE"))
	budgetKey, err := lifecycle.LoadBudgetKey(config)
	if err != nil {
		return nil, err
	}
	application.SetBudgetKey(budgetKey)
	if config.GeoIPCountryDBRef != "" {
		countries, err := geoip.Open(config.GeoIPCountryDBRef)
		if err != nil {
			return nil, err
		}
		application.SetCountries(countries)
	}
	application.MountWeb(webDirectory())
	return application, nil
}

// webDirectory 取后台静态目录：NW_WEB_DIR，否则当前目录下的 web/dist。
func webDirectory() string {
	if dir := os.Getenv("NW_WEB_DIR"); dir != "" {
		return dir
	}
	if executable, err := os.Executable(); err == nil {
		if candidate := filepath.Join(filepath.Dir(executable), "web"); fileExists(filepath.Join(candidate, "index.html")) {
			return candidate
		}
	}
	return "web/dist"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// NewRuntimeClient 构造 site-config 运行权威客户端，不在此函数中缓存
// 业务数据；所有运行状态请求由已登记的服务断言客户端发出。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	base := config.Dependencies["site-config"]
	if base == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	_, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

type catalogRoute struct {
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Owner       string  `json:"owner"`
	OperationID string  `json:"operationId"`
	Permission  string  `json:"permission"`
	Admission   *string `json:"admission"`
	TimeoutMS   *int    `json:"timeoutMs"`
	Streaming   bool    `json:"streaming"`
}

type catalogFile struct {
	Version int            `json:"version"`
	Routes  []catalogRoute `json:"routes"`
}

func loadRouteRegistry() (portal.RouteRegistry, error) {
	paths := []string{"config/routes.json"}
	if file := os.Getenv("NW_ROUTES_FILE"); file != "" {
		paths = []string{file}
	}
	if executable, err := os.Executable(); err == nil {
		base := filepath.Dir(executable)
		paths = append(paths, filepath.Join(base, "config", "routes.json"), filepath.Join(base, "..", "config", "routes.json"))
	}
	var raw []byte
	var readErr error
	for _, path := range paths {
		raw, readErr = os.ReadFile(path)
		if readErr == nil {
			break
		}
	}
	if readErr != nil {
		return portal.RouteRegistry{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口路由表不可读")
	}
	var file catalogFile
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil || file.Version != 1 || len(file.Routes) == 0 {
		return portal.RouteRegistry{}, platform.NewError(platform.CodeValidation, false, "入口路由表格式不合法")
	}
	local := map[string]bool{
		"GET\x00/healthz": true, "GET\x00/readyz": true,
		"GET\x00/v1/bootstrap": true, "GET\x00/v1/events": true,
		"GET\x00/v1/preferences": true, "PUT\x00/v1/preferences": true,
		"GET\x00/v1/readiness": true,
	}
	definitions := make([]portal.RouteDefinition, 0, len(file.Routes))
	for _, route := range file.Routes {
		method := strings.ToUpper(route.Method)
		if local[method+"\x00"+route.Path] || route.Owner == "portal" {
			continue
		}
		timeout := 5000
		if route.TimeoutMS != nil {
			timeout = *route.TimeoutMS
		}
		maxBody := int64(0)
		if method != "GET" && method != "HEAD" {
			maxBody = 16 << 20
		}
		admission := ""
		if route.Admission != nil {
			admission = *route.Admission
		}
		definitions = append(definitions, portal.RouteDefinition{
			RouteID: route.Owner + ":" + route.OperationID, Method: method, PathTemplate: route.Path,
			Owner: route.Owner, UpstreamOperationID: route.OperationID,
			Permission: portal.PermissionRequirement{Permission: route.Permission}, TimeoutMS: timeout,
			MaxBodyBytes: maxBody, Streaming: route.Streaming, AllowedHeaders: []string{}, Admission: admission,
		})
	}
	compiled, err := portal.CompileRouteRegistry(portal.CompileRouteRegistryInput{Definitions: definitions})
	if err != nil {
		return portal.RouteRegistry{}, err
	}
	return compiled.Registry, nil
}

func ensureDataLayout(dataRoot string) error {
	for _, name := range []string{"active", "control"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立portal数据布局失败")
		}
	}
	return nil
}

func readSiteIdentity(ctx context.Context, config platform.ServiceConfig, key platform.SigningKey, epoch platform.Identifier) (platform.Identifier, error) {
	validate, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "PortalSiteConfigResponse")
	if err != nil {
		return "", err
	}
	deadline := time.Now().UTC().Add(30 * time.Second)
	work, cancel := context.WithDeadline(platform.WithPortalProof(ctx, ""), deadline)
	defer cancel()
	id := platform.NewIdentifier()
	op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: epoch, Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
	client := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
	raw, err := client.ReadSiteConfig(platform.WithOperationContext(work, op))
	if err != nil {
		return "", err
	}
	if err := validate(raw); err != nil {
		return "", err
	}
	var result struct {
		Config struct {
			SiteID platform.Identifier `json:"siteId"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "站点身份响应不可读")
	}
	return result.Config.SiteID, nil
}

// hostResolver 经 identity:resolveHost 把访问域名解析成站点；域名未登记返回 (_, false, nil)。
func hostResolver(client *gen.IdentityClient, epoch platform.Identifier) func(context.Context, string) (transport.HostSite, bool, error) {
	return func(ctx context.Context, host string) (transport.HostSite, bool, error) {
		deadline := platform.Timestamp(time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano))
		id := platform.NewIdentifier()
		op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline, DataEpoch: epoch,
			Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: deadline, Grants: []string{}}}
		raw, err := client.ResolveHost(platform.WithPortalProof(platform.WithOperationContext(ctx, op), ""), host)
		if err != nil {
			var failure *platform.Error
			if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
				return transport.HostSite{}, false, nil
			}
			return transport.HostSite{}, false, err
		}
		var body struct {
			SiteID       string `json:"siteId"`
			Status       string `json:"status"`
			Visibility   string `json:"visibility"`
			PrimaryHost  string `json:"primaryHost"`
			RedirectHost string `json:"redirectHost"`
			PlatformHost string `json:"platformHost"`
		}
		if json.Unmarshal(raw, &body) != nil || body.SiteID == "" {
			return transport.HostSite{}, false, nil
		}
		return transport.HostSite{SiteID: body.SiteID, Status: body.Status, Visibility: body.Visibility, PrimaryHost: body.PrimaryHost, RedirectHost: body.RedirectHost, PlatformHost: body.PlatformHost}, true, nil
	}
}

// tlsAllower 问身份服务：这个域名能不能签证书（平台域名、已验证的站点域名）。
func tlsAllower(client *gen.IdentityClient, epoch platform.Identifier) func(context.Context, string) (bool, error) {
	return func(ctx context.Context, host string) (bool, error) {
		deadline := platform.Timestamp(time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano))
		id := platform.NewIdentifier()
		op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline, DataEpoch: epoch,
			Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: deadline, Grants: []string{}}}
		raw, err := client.AllowTlsHost(platform.WithPortalProof(platform.WithOperationContext(ctx, op), ""), host)
		if err != nil {
			return false, err
		}
		var body struct {
			Allowed bool `json:"allowed"`
		}
		if json.Unmarshal(raw, &body) != nil {
			return false, nil
		}
		return body.Allowed, nil
	}
}

// privateRedeemer 在站点域名上把协作者带来的一次性码交给身份服务核对。
func privateRedeemer(client *gen.IdentityClient, epoch platform.Identifier) func(context.Context, platform.Identifier, string) error {
	return func(ctx context.Context, siteID platform.Identifier, code string) error {
		deadline := platform.Timestamp(time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano))
		id := platform.NewIdentifier()
		op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline, DataEpoch: epoch,
			Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: siteID, ExpiresAt: deadline, Grants: []string{}}}
		_, err := client.RedeemPrivateAccess(platform.WithPortalProof(platform.WithOperationContext(ctx, op), ""), string(siteID), struct {
			Code string `json:"code"`
		}{code})
		return err
	}
}
