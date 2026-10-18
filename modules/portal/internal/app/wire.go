// Package app 装配 portal 的入口路由、运行状态客户端和 HTTP 容器。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"nice-wiki/libs/platform/apivalidate"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
		portal "nice-wiki/modules/portal/internal"
	"nice-wiki/modules/portal/internal/gen"
	"nice-wiki/modules/portal/internal/transport"
)

const ModuleSlug = "portal"

// BuildApplication 完成 portal 无状态入口的装配。入口没有业务DB，
// active/control/restore 目录仅为部署布局与后续恢复屏障保留。
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
 if err != nil { return nil, err }
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
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执尚未装配")
	}
	routes, err := loadRouteRegistry()
	if err != nil {
		return nil, err
	}
	var csrfVerifier portal.IdentityVerifier
	var authResolver portal.AuthResolver
	if base := config.Dependencies["identity"]; base != "" {
		client := gen.NewIdentityClient(base, ModuleSlug, key)
		csrfVerifier = portal.CSRFVerifier{Client: client}
		authResolver = portal.IdentityAuthResolver{Client: client}
	}
	siteID, err := readSiteIdentity(ctx, config, key, view.DataEpoch)
 if err != nil { return nil, err }
 return transport.NewApplication(config, anchors, key, delegationKey, runtimeClient, csrfVerifier, authResolver, routes, view.DataEpoch, siteID)
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
		"POST\x00/v1/barriers": true, "DELETE\x00/v1/barriers/{barrierId}": true,
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
	for _, name := range []string{"active", "control", "restore"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立portal数据布局失败")
		}
	}
	return nil
}

func readSiteIdentity(ctx context.Context, config platform.ServiceConfig, key platform.SigningKey, epoch platform.Identifier) (platform.Identifier, error) {
 validate, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "PortalSiteConfigResponse")
 if err != nil { return "", err }
 deadline := time.Now().UTC().Add(30*time.Second)
 work, cancel := context.WithDeadline(platform.WithPortalProof(ctx, ""), deadline)
 defer cancel()
 id := platform.NewIdentifier()
 op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: epoch, Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
 client := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
 raw, err := client.ReadSiteConfig(platform.WithOperationContext(work, op))
 if err != nil { return "", err }
 if err := validate(raw); err != nil { return "", err }
 var result struct { Config struct { SiteID platform.Identifier `json:"siteId"` } `json:"config"` }
 if err := json.Unmarshal(raw, &result); err != nil { return "", platform.NewError(platform.CodeIntegrityError, false, "站点身份响应不可读") }
 return result.Config.SiteID, nil
}
