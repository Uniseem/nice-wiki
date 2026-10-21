// Package geoip 用离线 IP 国家库（MaxMind 格式，如 DB-IP Lite Country）把访客地址换成两位国家代码。
// 只查国家，不保存也不转发地址本身。
package geoip

import (
	"net"
	"net/netip"

	"github.com/oschwald/maxminddb-golang"
	"nice-wiki/libs/platform"
)

// Countries 是打开的离线国家库；并发只读安全。
type Countries struct {
	reader *maxminddb.Reader
}

// Open 打开国家库文件。
func Open(path string) (*Countries, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "离线IP国家库无法打开")
	}
	return &Countries{reader: reader}, nil
}

// Country 返回地址所在国家的两位代码（大写）；库里查不到时返回空串。
func (c *Countries) Country(addr netip.Addr) string {
	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := c.reader.Lookup(net.IP(addr.AsSlice()), &record); err != nil {
		return ""
	}
	return record.Country.ISOCode
}
