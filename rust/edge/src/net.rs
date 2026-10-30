//! 访客地址与网络作用域：只信任部署配置里的代理网段带来的 X-Forwarded-For；
//! 原始地址只在内存里用来派生 HMAC 作用域、查国家，不落日志（与 serviceauth 相同）。

use std::net::{IpAddr, Ipv6Addr, SocketAddr};

use crate::platform::{sha256_hex, Key, Problem};

pub struct Cidr {
    network: IpAddr,
    bits: u8,
}

impl Cidr {
    pub fn parse(text: &str) -> anyhow::Result<Self> {
        let (addr, bits) = text.split_once('/').ok_or_else(|| anyhow::anyhow!("代理网段不合法：{text}"))?;
        Ok(Self { network: unmap(addr.parse()?), bits: bits.parse()? })
    }

    fn contains(&self, addr: IpAddr) -> bool {
        match (self.network, addr) {
            (IpAddr::V4(net), IpAddr::V4(ip)) => self.bits <= 32 && mask(u32::from(net) as u128, u32::from(ip) as u128, 32, self.bits),
            (IpAddr::V6(net), IpAddr::V6(ip)) => self.bits <= 128 && mask(u128::from(net), u128::from(ip), 128, self.bits),
            _ => false,
        }
    }
}

fn mask(net: u128, ip: u128, width: u8, bits: u8) -> bool {
    if bits == 0 {
        return true;
    }
    let shift = width - bits;
    (net >> shift) == (ip >> shift)
}

fn unmap(addr: IpAddr) -> IpAddr {
    match addr {
        IpAddr::V6(v6) => v6.to_ipv4_mapped().map(IpAddr::V4).unwrap_or(IpAddr::V6(v6)),
        v4 => v4,
    }
}

/// 访客真实地址：连接来自受信代理时，从 X-Forwarded-For 末尾往前跳过受信代理。
pub fn client_address(peer: SocketAddr, forwarded_for: &str, trusted: &[Cidr]) -> Result<IpAddr, Problem> {
    let invalid = || Problem::new("VALIDATION", "网络来源或代理链不合法");
    let is_trusted = |addr: IpAddr| trusted.iter().any(|c| c.contains(addr));
    let mut addr = unmap(peer.ip());
    if is_trusted(addr) {
        if forwarded_for.is_empty() || forwarded_for.len() > 4096 {
            return Err(invalid());
        }
        let hops: Vec<&str> = forwarded_for.split(',').collect();
        if hops.len() > 16 {
            return Err(invalid());
        }
        for hop in hops.iter().rev() {
            if !is_trusted(addr) {
                break;
            }
            addr = unmap(hop.trim().parse().map_err(|_| invalid())?);
        }
    }
    Ok(addr)
}

/// 登录用的网络作用域：当天与前一天（UTC）的私有摘要，服务签名密钥派生。
pub fn login_scopes(key: &Key, addr: IpAddr) -> Vec<String> {
    let today = time::OffsetDateTime::now_utc().date();
    [today, today.previous_day().expect("日期有前一天")]
        .iter()
        .map(|day| key.private_digest(&format!("network/day/v1/{:04}-{:02}-{:02}", day.year(), u8::from(day.month()), day.day()), addr.to_string().as_bytes()))
        .collect()
}

/// 匿名预算的网络作用域：30 天一期（当期与上期），IPv6 按 /64 合并，预算密钥派生。
pub fn budget_scopes(key: &Key, addr: IpAddr, site: &str) -> Vec<String> {
    let addr = match addr {
        IpAddr::V6(v6) => IpAddr::V6(Ipv6Addr::from(u128::from(v6) & !((1u128 << 64) - 1))),
        v4 => v4,
    };
    let period = time::OffsetDateTime::now_utc().unix_timestamp() / (30 * 24 * 60 * 60);
    let site = sha256_hex(site.as_bytes());
    [period, period - 1].iter().map(|index| key.private_digest(&format!("guard/network/30d/v1/{site}/{index}"), addr.to_string().as_bytes())).collect()
}
