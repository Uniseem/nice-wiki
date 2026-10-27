use std::collections::{HashMap, VecDeque};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use bytes::Bytes;
use hyper::header::{HeaderName, HeaderValue};
use hyper::StatusCode;

#[derive(Clone)]
pub struct Entry {
    pub status: StatusCode,
    pub headers: Vec<(HeaderName, HeaderValue)>,
    pub body: Bytes,
    pub expires: Instant,
    pub host: String,
    /// 上游（portal）在响应头 X-Nw-Site 里标明的站点，按站点清缓存时用。
    pub site: String,
}

/// 内存缓存：总字节数有上限，超出时按写入先后淘汰最旧的（简单、够用）。
pub struct Cache {
    capacity: usize,
    inner: Mutex<Inner>,
}

struct Inner {
    map: HashMap<String, Entry>,
    order: VecDeque<String>,
    bytes: usize,
}

impl Cache {
    pub fn new(capacity: usize) -> Self {
        Self { capacity, inner: Mutex::new(Inner { map: HashMap::new(), order: VecDeque::new(), bytes: 0 }) }
    }

    pub fn get(&self, key: &str) -> Option<Entry> {
        let mut inner = self.inner.lock().unwrap();
        match inner.map.get(key) {
            Some(entry) if entry.expires > Instant::now() => Some(entry.clone()),
            Some(_) => {
                Self::remove(&mut inner, key);
                None
            }
            None => None,
        }
    }

    pub fn put(&self, key: String, entry: Entry, ttl: Duration) {
        let size = entry.body.len() + key.len() + 256;
        if size > self.capacity {
            return;
        }
        let mut entry = entry;
        entry.expires = Instant::now() + ttl;
        let mut inner = self.inner.lock().unwrap();
        Self::remove(&mut inner, &key);
        while inner.bytes + size > self.capacity {
            let Some(oldest) = inner.order.pop_front() else { break };
            if let Some(old) = inner.map.remove(&oldest) {
                inner.bytes -= old.body.len() + oldest.len() + 256;
            }
        }
        inner.bytes += size;
        inner.order.push_back(key.clone());
        inner.map.insert(key, entry);
    }

    fn remove(inner: &mut Inner, key: &str) {
        if let Some(old) = inner.map.remove(key) {
            inner.bytes -= old.body.len() + key.len() + 256;
            inner.order.retain(|k| k != key);
        }
    }

    /// 清掉某个域名下的全部缓存页面，返回清掉的个数。
    pub fn purge_host(&self, host: &str) -> usize {
        let mut inner = self.inner.lock().unwrap();
        let keys: Vec<String> = inner.map.iter().filter(|(_, entry)| entry.host == host).map(|(key, _)| key.clone()).collect();
        for key in &keys {
            Self::remove(&mut inner, key);
        }
        keys.len()
    }

    /// 清掉某个站点的全部缓存页面（站点的任何域名下的），返回清掉的个数。
    pub fn purge_site(&self, site: &str) -> usize {
        let mut inner = self.inner.lock().unwrap();
        let keys: Vec<String> = inner.map.iter().filter(|(_, entry)| entry.site == site).map(|(key, _)| key.clone()).collect();
        for key in &keys {
            Self::remove(&mut inner, key);
        }
        keys.len()
    }

    pub fn stats(&self) -> (usize, usize) {
        let inner = self.inner.lock().unwrap();
        (inner.map.len(), inner.bytes)
    }
}
