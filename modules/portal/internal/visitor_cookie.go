package internal

import (
	"crypto/hmac"
	"net/http"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// analyticsVisitor 使用本站随机cookie区分访客；不按IP或设备指纹合并，不作为授权身份。
func analyticsVisitor(runtime *ProxyRuntime) (string, error) {
	sign := func(id string) (string, error) {
		return serviceauth.PrivateOperationDigest(runtime.DelegationKey, "portal.analytics-visitor.v1", []byte(runtime.SiteID+":"+id))
	}
	if cookie, err := runtime.Request.Cookie("nw_visitor"); err == nil {
		parts := strings.Split(cookie.Value, ".")
		if len(parts) == 2 && platform.ValidIdentifier(parts[0]) {
			expected, err := sign(parts[0])
			if err != nil {
				return "", err
			}
			if hmac.Equal([]byte(expected), []byte(parts[1])) {
				return parts[0], nil
			}
		}
	}
	id := platform.NewIdentifier()
	signature, err := sign(id)
	if err != nil {
		return "", err
	}
	secure := false
	for _, origin := range runtime.ConfiguredOrigins {
		if strings.HasPrefix(origin, "https://") {
			secure = true
		}
	}
	http.SetCookie(runtime.Writer, &http.Cookie{Name: "nw_visitor", Value: id + "." + signature, Path: "/", MaxAge: 180 * 24 * 3600, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	return id, nil
}
