// Package lifecycle 实现服务配置加载、运行状态读取与进程模板。
//
// 各服务main只装配本域参数并调用RunService；启动、信号、关闭与
// 恢复重启的退出码语义统一在本包实现。
package lifecycle

import (
	"bytes"
	"encoding/json"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"golang.org/x/net/idna"

	"nice-wiki/libs/platform"
)

// 命令行与环境变量约定。
const (
	envPrefix       = "NW_"
	defaultListen   = "127.0.0.1:8080"
	maxDependencies = 32
)

// LoadServiceConfig 按CLI>env>文件>内置默认加载并严格校验服务配置：
// 只接受--config/--data-dir/--listen与NW_前缀环境变量；配置文件
// 拒绝未知字段；secret只保存引用路径，不读入日志。
func LoadServiceConfig(args []string, env []string) (platform.ServiceConfig, error) {
	flags, err := parseFlags(args)
	if err != nil {
		return platform.ServiceConfig{}, err
	}
	config := platform.ServiceConfig{
		Listen:       defaultListen,
		Dependencies: map[string]string{},
	}
	if flags.configPath != "" {
		if err := loadConfigFile(flags.configPath, &config); err != nil {
			return platform.ServiceConfig{}, err
		}
	}
	if config.Dependencies == nil {
		config.Dependencies = map[string]string{}
	}
	if err := applyEnv(env, &config); err != nil {
		return platform.ServiceConfig{}, err
	}
	applyFlags(flags, &config)
	if config.ReportingTimeZone != "" {
		if config.Module != "analytics" || config.ReportingTimeZone == "Local" {
			return platform.ServiceConfig{}, platform.NewError(platform.CodeValidation, false, "报表时区只供统计域可信部署配置且不能使用宿主本地时区")
		}
		if _, err := time.LoadLocation(config.ReportingTimeZone); err != nil {
			return platform.ServiceConfig{}, platform.NewError(platform.CodeValidation, false, "报表时区必须为有效IANA名称")
		}
	}
	if config.PublicBaseURL != "" {
		config.PublicBaseURL, err = NormalizePublicURL(config.PublicBaseURL, config.AllowLocalHTTP)
		if err != nil {
			return platform.ServiceConfig{}, err
		}
	}
	if err := validateConfig(config); err != nil {
		return platform.ServiceConfig{}, err
	}
	return config, nil
}

// NormalizePublicURL 规范受信部署地址：IDNA Lookup、HTTPS与显式本地HTTP。
// 保留部署basePath，拒绝带凭据、查询、片段、控制字符、连续斜杠或路径穿越的地址。
func NormalizePublicURL(raw string, allowLocalHTTP bool) (string, error) {
	invalid := func() (string, error) {
		return "", platform.NewError(platform.CodeValidation, false, "公开地址不合法")
	}
	if strings.ContainsAny(raw, "\x00\r\n\t\\") {
		return invalid()
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Opaque != "" || u.Hostname() == "" || u.User != nil || u.ForceQuery || u.RawQuery != "" || u.Fragment != "" || strings.Contains(raw, "#") {
		return invalid()
	}
	if strings.Contains(u.Path, "//") || strings.Contains(u.RawPath, "//") {
		return invalid()
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	local := host == "localhost" || strings.HasSuffix(host, ".localhost")
	ip := net.ParseIP(host)
	if ip != nil {
		host = ip.String()
		local = ip.IsLoopback()
	} else {
		host, err = idna.Lookup.ToASCII(host)
		if err != nil || host == "" || len(host) > 253 {
			return invalid()
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "https" && !(u.Scheme == "http" && allowLocalHTTP && local) {
		return invalid()
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid()
		}
		port = strconv.Itoa(n)
		if (u.Scheme == "https" && n == 443) || (u.Scheme == "http" && n == 80) {
			port = ""
		}
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if ip != nil && ip.To4() == nil {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	for _, segment := range strings.Split(u.EscapedPath(), "/") {
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "/\\\x00\r\n") {
			return invalid()
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u.String(), nil
}

// CanonicalPublicOrigin 从已经通过部署边界校验的规范地址提取Origin。
func CanonicalPublicOrigin(publicBaseURL string) (string, error) {
	u, err := url.Parse(publicBaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", platform.NewError(platform.CodeValidation, false, "可信公开地址缺少Origin")
	}
	return u.Scheme + "://" + u.Host, nil
}

// ValidatePrivatePath 在打开私有权威前拒绝目录链链接、特殊文件及硬链接文件。
// allowMissing只允许首次创建时尚不存在的尾部，不改变现有文件的类型要求。
func ValidatePrivatePath(path, kind string, allowMissing bool) error {
	if !filepath.IsAbs(path) || (kind != "directory" && kind != "regular-file") {
		return platform.NewError(platform.CodeValidation, false, "私有路径声明不合法")
	}
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == "." || part == ".." {
			return platform.NewError(platform.CodeIntegrityError, false, "私有路径含相对跳转")
		}
	}
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	current := volume + string(filepath.Separator)
	if clean == current && kind == "regular-file" {
		return platform.NewError(platform.CodeIntegrityError, false, "根目录不能作为私有权威文件")
	}
	parts := strings.Split(strings.TrimPrefix(clean, current), string(filepath.Separator))
	if clean == current {
		parts = nil
	}
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) && allowMissing {
			return nil
		}
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "私有权威路径不可读取")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "私有权威路径包含符号链接")
		}
		leaf := index == len(parts)-1
		if !leaf || kind == "directory" {
			if !info.IsDir() {
				return platform.NewError(platform.CodeIntegrityError, false, "私有目录链包含非目录")
			}
		} else {
			if !info.Mode().IsRegular() {
				return platform.NewError(platform.CodeIntegrityError, false, "私有权威不是正规文件")
			}
			if err := checkSingleFileLink(current, info); err != nil {
				return err
			}
		}
	}
	return nil
}

type flagSet struct {
	configPath string
	dataDir    string
	listen     string
}

// parseFlags 只接受--config/--data-dir/--listen，未知参数拒绝。
func parseFlags(args []string) (flagSet, error) {
	var flags flagSet
	for i := 0; i < len(args); i++ {
		name, value, hasValue := splitFlag(args[i])
		consume := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", platform.NewError(platform.CodeValidation, false, "参数缺少取值 %s", name)
			}
			i++
			return args[i], nil
		}
		switch name {
		case "--config":
			next, err := consume()
			if err != nil {
				return flags, err
			}
			flags.configPath = next
		case "--data-dir":
			next, err := consume()
			if err != nil {
				return flags, err
			}
			flags.dataDir = next
		case "--listen":
			next, err := consume()
			if err != nil {
				return flags, err
			}
			flags.listen = next
		default:
			return flags, platform.NewError(platform.CodeValidation, false, "未知命令行参数")
		}
	}
	return flags, nil
}

func splitFlag(arg string) (name, value string, hasValue bool) {
	if idx := strings.Index(arg, "="); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return arg, "", false
}

// loadConfigFile 读取JSON配置文件；未知字段一律拒绝。
func loadConfigFile(path string, config *platform.ServiceConfig) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "配置文件不可读")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(config); err != nil {
		return platform.NewError(platform.CodeValidation, false, "配置文件包含未知字段或格式不合法")
	}
	return nil
}

// applyEnv 只读取NW_前缀环境变量；依赖地址形如NW_DEP_<SLUG>。
func applyEnv(env []string, config *platform.ServiceConfig) error {
	for _, entry := range env {
		name, value, found := strings.Cut(entry, "=")
		if !found || !strings.HasPrefix(name, envPrefix) {
			continue
		}
		key := strings.TrimPrefix(name, envPrefix)
		switch key {
		case "MODULE":
			config.Module = value
		case "LISTEN":
			config.Listen = value
		case "DATA_DIR":
			config.DataRoot = value
		case "TRUST_ANCHOR_REF":
			config.TrustAnchorRef = value
		case "SIGNING_KEY_REF":
			config.SigningKeyRef = value
		case "DELEGATION_KEY_REF":
			config.DelegationKeyRef = value
		case "MASTER_KEY_REF":
			config.MasterKeyRef = value
		case "BUDGET_KEY_REF":
			config.BudgetKeyRef = value
		case "PUBLIC_BASE_URL":
			config.PublicBaseURL = value
		case "TRUSTED_PROXY_CIDRS":
			config.TrustedProxyCIDRs = nil
			if value != "" {
				for _, cidr := range strings.Split(value, ",") {
					config.TrustedProxyCIDRs = append(config.TrustedProxyCIDRs, strings.TrimSpace(cidr))
				}
			}
		case "ALLOW_LOCAL_HTTP":
			allowed, err := strconv.ParseBool(value)
			if err != nil {
				return platform.NewError(platform.CodeValidation, false, "本地HTTP开关必须为布尔值")
			}
			config.AllowLocalHTTP = allowed
		default:
			if slug, ok := strings.CutPrefix(key, "DEP_"); ok && value != "" {
				config.Dependencies[strings.ToLower(slug)] = value
			}
		}
	}
	return nil
}

func applyFlags(flags flagSet, config *platform.ServiceConfig) {
	if flags.dataDir != "" {
		config.DataRoot = flags.dataDir
	}
	if flags.listen != "" {
		config.Listen = flags.listen
	}
}

// validateConfig 校验端口、数据根、依赖地址与secret引用。
func validateConfig(config platform.ServiceConfig) error {
	if config.Module == "" {
		return platform.NewError(platform.CodeValidation, false, "缺少服务slug")
	}
	if _, err := splitListen(config.Listen); err != nil {
		return err
	}
	if !filepath.IsAbs(config.DataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	if config.TrustAnchorRef == "" || config.SigningKeyRef == "" {
		return platform.NewError(platform.CodeValidation, false, "缺少公钥表或私钥引用")
	}
	if config.DelegationKeyRef != "" && !filepath.IsAbs(config.DelegationKeyRef) {
		return platform.NewError(platform.CodeValidation, false, "委托私钥引用必须为绝对路径")
	}
	if config.BudgetKeyRef != "" && !filepath.IsAbs(config.BudgetKeyRef) {
		return platform.NewError(platform.CodeValidation, false, "预算密钥引用必须为绝对路径")
	}
	if config.MasterKeyRef != "" && !filepath.IsAbs(config.MasterKeyRef) {
		return platform.NewError(platform.CodeValidation, false, "主密钥引用必须为绝对路径")
	}
	if len(config.Dependencies) > maxDependencies {
		return platform.NewError(platform.CodeValidation, false, "依赖地址数量超限")
	}
	if len(config.TrustedProxyCIDRs) > 32 {
		return platform.NewError(platform.CodeValidation, false, "可信代理网段数量超限")
	}
	for _, cidr := range config.TrustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil || prefix != prefix.Masked() || prefix.Addr().Is4In6() {
			return platform.NewError(platform.CodeValidation, false, "可信代理必须为规范IP网段")
		}
	}
	for slug, raw := range config.Dependencies {
		if slug == "" {
			return platform.NewError(platform.CodeValidation, false, "依赖slug不能为空")
		}
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return platform.NewError(platform.CodeValidation, false, "依赖地址不合法")
		}
		if parsed.User != nil {
			return platform.NewError(platform.CodeValidation, false, "依赖地址不得包含URL凭证")
		}
	}

	return nil
}

// splitListen 校验host:port并返回端口。
func splitListen(listen string) (int, error) {
	_, portText, found := strings.Cut(listen, ":")
	if !found {
		return 0, platform.NewError(platform.CodeValidation, false, "监听地址必须为host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return 0, platform.NewError(platform.CodeValidation, false, "监听端口不合法")
	}
	return port, nil
}

func integrity(message string) error {
	return platform.NewError(platform.CodeIntegrityError, false, "%s", message)
}
