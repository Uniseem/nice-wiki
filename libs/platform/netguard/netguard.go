// Package netguard 防 SSRF：用户自带的 AI 渠道、导入、Webhook 这类“用户给地址、服务端去连”的出站请求都先过这里。
// 两道关：保存前解析地址检查一遍；真正连接时（拨号前）再按实际 IP 检查一遍，防 DNS 重绑定。
package netguard

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

var blockedNets = mustCIDRs(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24",
	"192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8", "64:ff9b::/96",
)

func mustCIDRs(items ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(items))
	for _, item := range items {
		_, network, err := net.ParseCIDR(item)
		if err != nil {
			panic(err)
		}
		nets = append(nets, network)
	}
	return nets
}

// Blocked 判断地址是否属于私网、环回、链路本地（含云元数据 169.254.169.254）、组播等不能被用户指向的范围。
func Blocked(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range blockedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Loopback 判断是否环回地址；本地开发（allowLocalHTTP）时才放行。
func Loopback(ip net.IP) bool { return ip.IsLoopback() }

func allowed(ip net.IP, allowLoopback bool) bool {
	return !Blocked(ip) || (allowLoopback && Loopback(ip))
}

// Control 用作 net.Dialer.Control：拨号前检查即将连接的实际 IP。
func Control(allowLoopback bool) func(network, address string, _ syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("出站地址不合法")
		}
		ip := net.ParseIP(host)
		if ip == nil || !allowed(ip, allowLoopback) {
			return fmt.Errorf("出站地址不允许")
		}
		return nil
	}
}

// CheckHost 保存前检查：域名解析出的每个地址都必须允许。
func CheckHost(ctx context.Context, host string, allowLoopback bool) error {
	if ip := net.ParseIP(host); ip != nil {
		if !allowed(ip, allowLoopback) {
			return fmt.Errorf("地址不允许")
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("域名无法解析")
	}
	for _, ip := range ips {
		if !allowed(ip.IP, allowLoopback) {
			return fmt.Errorf("地址不允许")
		}
	}
	return nil
}
