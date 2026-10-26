use std::collections::HashMap;
use std::net::IpAddr;
use std::sync::Mutex;
use std::time::Instant;

use crate::config::Rate;

struct Bucket {
    tokens: f64,
    at: Instant,
}

/// 按客户端地址的令牌桶（内存、单实例）。读和写各一套桶，互不挤占。
pub struct Limiter {
    read: Rate,
    write: Rate,
    buckets: Mutex<HashMap<(IpAddr, bool), Bucket>>,
}

impl Limiter {
    pub fn new(read: Rate, write: Rate) -> Self {
        Self { read, write, buckets: Mutex::new(HashMap::new()) }
    }

    /// 取一个令牌；没有就返回需要等待的秒数。
    pub fn take(&self, ip: IpAddr, write: bool) -> Result<(), u64> {
        let rate = if write { &self.write } else { &self.read };
        let now = Instant::now();
        let mut buckets = self.buckets.lock().unwrap();
        if buckets.len() > 100_000 {
            // 空闲很久的桶已经回满，丢掉不影响限流效果。
            buckets.retain(|_, bucket| now.duration_since(bucket.at).as_secs() < 60);
        }
        let bucket = buckets.entry((ip, write)).or_insert(Bucket { tokens: rate.burst, at: now });
        let elapsed = now.duration_since(bucket.at).as_secs_f64();
        bucket.tokens = (bucket.tokens + elapsed * rate.per_second).min(rate.burst);
        bucket.at = now;
        if bucket.tokens >= 1.0 {
            bucket.tokens -= 1.0;
            Ok(())
        } else {
            Err(((1.0 - bucket.tokens) / rate.per_second).ceil().max(1.0) as u64)
        }
    }
}
