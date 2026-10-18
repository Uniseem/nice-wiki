package analytics

import (
	"context"
	"net"
	"strings"

	"nice-wiki/libs/platform"
)

// ValidateBeacon 只接受已登记访问类型和不含路径、端口或查询的来源域。
// 原始 HTTP 正文的 2048 字节上限由入口控制，不以重编码代替原始大小。
func ValidateBeacon(ctx context.Context, op OperationContext, in ValidateBeaconInput) (ValidateBeaconResult, error) {
	if in.EventType != "page-view" && in.EventType != "navigation-click" {
		return ValidateBeaconResult{}, platform.NewError(platform.CodeValidation, false, "访问类型未登记")
	}
	domain := in.ReferrerDomain
	if domain != "" {
		if len(domain) > 253 || strings.ContainsAny(domain, "/:?#@\\\r\n\t ") || net.ParseIP(domain) != nil {
			return ValidateBeaconResult{}, platform.NewError(platform.CodeValidation, false, "来源必须为域名")
		}
		for _, label := range strings.Split(domain, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return ValidateBeaconResult{}, platform.NewError(platform.CodeValidation, false, "来源域名不合法")
			}
			for _, c := range label {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
					return ValidateBeaconResult{}, platform.NewError(platform.CodeValidation, false, "来源域名不合法")
				}
			}
		}
	}
	return ValidateBeaconResult{Valid: true}, nil
}
