//! nice-wiki 的 Rust 平台库：与 Go 平台库（libs/platform）兼容的服务身份、断言与委托、出站调用、
//! 入站认证、事件队列消费和模型网关调用。Rust 服务只从这里取这些公共机制，不各自复制。

pub mod inbound;
pub mod model;
pub mod platform;
pub mod upstream;

#[cfg(feature = "db")]
pub mod db;
#[cfg(feature = "db")]
pub mod queue;
