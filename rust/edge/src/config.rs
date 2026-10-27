use anyhow::Context;
use serde::Deserialize;

#[derive(Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Rate {
    pub per_second: f64,
    pub burst: f64,
}

#[derive(Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Config {
    pub listen: String,
    /// 上游（portal）地址，形如 127.0.0.1:8080。
    pub upstream: String,
    /// 页面（HTML）在缓存里停留的秒数；上游说 no-cache 也按这个短缓存处理，发布后靠清缓存接口立即失效。
    #[serde(default = "default_html_ttl")]
    pub html_ttl_seconds: u64,
    /// 不可变资源（带内容哈希的媒体、样式脚本）最长缓存秒数。
    #[serde(default = "default_asset_ttl")]
    pub asset_ttl_seconds: u64,
    /// 缓存总字节数上限。
    #[serde(default = "default_cache_bytes")]
    pub cache_bytes: usize,
    /// 单个响应超过这个大小就不缓存。
    #[serde(default = "default_max_object")]
    pub max_object_bytes: usize,
    /// 媒体磁盘缓存目录；缺省时不开启磁盘缓存。
    #[serde(default)]
    pub media_disk_dir: Option<String>,
    /// 媒体磁盘缓存总字节数上限。
    #[serde(default = "default_media_disk_bytes")]
    pub media_disk_bytes: u64,
    /// 单个媒体超过这个大小就不落盘。
    #[serde(default = "default_media_max_object")]
    pub media_max_object_bytes: usize,
    #[serde(default = "default_read_rate")]
    pub read_rate: Rate,
    #[serde(default = "default_write_rate")]
    pub write_rate: Rate,
    /// 前面是可信的反向代理（Caddy）时，取 X-Forwarded-For 的第一个地址当客户端地址。
    #[serde(default)]
    pub trust_forwarded: bool,
    /// 清缓存接口的共享密钥文件；缺省时不开放清缓存接口。
    #[serde(default)]
    pub purge_secret_file: Option<String>,
}

fn default_html_ttl() -> u64 {
    10
}
fn default_asset_ttl() -> u64 {
    86_400
}
fn default_cache_bytes() -> usize {
    256 << 20
}
fn default_max_object() -> usize {
    4 << 20
}
fn default_media_disk_bytes() -> u64 {
    2 << 30
}
fn default_media_max_object() -> usize {
    64 << 20
}
fn default_read_rate() -> Rate {
    Rate { per_second: 30.0, burst: 120.0 }
}
fn default_write_rate() -> Rate {
    Rate { per_second: 5.0, burst: 30.0 }
}

pub fn load(path: &str) -> anyhow::Result<Config> {
    let raw = std::fs::read_to_string(path).with_context(|| format!("读取配置 {path} 失败"))?;
    serde_json::from_str(&raw).with_context(|| "配置不是合法 JSON")
}
