package analytics

import (
	"context"
	"net"
	"net/netip"
	"nice-wiki/libs/platform"
)

// LookupCountry 仅查询启动装配的离线数据库；无库或无命中返回ZZ。
func LookupCountry(ctx context.Context, op OperationContext, in LookupCountryInput) (LookupCountryResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return LookupCountryResult{}, err
	}
	if rt.GeoIP == nil {
		return LookupCountryResult{CountryCode: "ZZ"}, nil
	}
	addr, err := trustedPeer(in.PeerAddress)
	if err != nil {
		return LookupCountryResult{}, err
	}
	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err = rt.GeoIP.Lookup(net.IP(addr.AsSlice()), &record); err != nil {
		return LookupCountryResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "离线国家数据库不可读")
	}
	code := record.Country.ISOCode
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		code = "ZZ"
	}
	return LookupCountryResult{CountryCode: code}, nil
}

func trustedPeer(peer string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(peer)
	if err != nil || addr.Zone() != "" {
		return netip.Addr{}, platform.NewError(platform.CodeValidation, false, "可信连接地址必须为规范IP")
	}
	return addr.Unmap(), nil
}
