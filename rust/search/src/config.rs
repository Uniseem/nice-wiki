use anyhow::Context;
use serde::Deserialize;

/// 配置只放地址和文件路径；密钥与数据库连接串在各自的私有文件里，不进配置、不进 git。
#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Config {
    pub listen: String,
    pub data_dir: String,
    pub secret_file: String,
    pub database_url_file: String,
}

pub fn load(path: &str) -> anyhow::Result<Config> {
    let raw = std::fs::read_to_string(path).with_context(|| format!("读取配置 {path} 失败"))?;
    serde_json::from_str(&raw).with_context(|| "配置不是合法 JSON")
}
