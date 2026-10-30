//! PostgreSQL 连接池：连接串放在配置 databaseRef 指向的私有文件里（与 Go 的 pgdb.ReadDSN 相同）。

use anyhow::Context;
use deadpool_postgres::{Config, Pool, PoolConfig, Runtime};
use tokio_postgres::NoTls;

pub fn pool(database_ref: &str, size: usize) -> anyhow::Result<Pool> {
    let dsn = std::fs::read_to_string(database_ref).with_context(|| "读取数据库连接串失败")?;
    let mut config = Config::new();
    config.url = Some(dsn.trim().to_owned());
    config.pool = Some(PoolConfig::new(size));
    config.create_pool(Some(Runtime::Tokio1), NoTls).with_context(|| "创建数据库连接池失败")
}
