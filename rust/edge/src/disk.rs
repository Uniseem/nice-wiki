//! 媒体磁盘缓存：带内容哈希的媒体字节是不可变的，落到磁盘，按最近使用淘汰；重启后沿用目录里已有的内容。
//! 尽力而为：任何读写失败都当没有缓存，不影响请求。

use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::{Instant, SystemTime, UNIX_EPOCH};

use bytes::Bytes;
use hyper::header::{HeaderName, HeaderValue};
use hyper::StatusCode;
use serde::{Deserialize, Serialize};

use crate::cache::Entry;

#[derive(Serialize, Deserialize)]
struct Meta {
    key: String,
    status: u16,
    headers: Vec<(String, String)>,
    site: String,
    /// 过期时刻（Unix 秒）。
    expires: u64,
}

struct Item {
    size: u64,
    used: u64,
    site: String,
}

struct Inner {
    items: HashMap<String, Item>,
    bytes: u64,
    tick: u64,
}

pub struct DiskCache {
    dir: PathBuf,
    capacity: u64,
    inner: Mutex<Inner>,
}

fn now_secs() -> u64 {
    SystemTime::now().duration_since(UNIX_EPOCH).map(|d| d.as_secs()).unwrap_or(0)
}

fn fnv(data: &[u8], seed: u64) -> u64 {
    let mut hash = 0xcbf2_9ce4_8422_2325u64 ^ seed;
    for byte in data {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(0x0100_0000_01b3);
    }
    hash
}

/// 缓存键对应的文件名（128 位，读取时还会核对元数据里的完整键）。
fn stem_of(key: &str) -> String {
    format!("{:016x}{:016x}", fnv(key.as_bytes(), 0), fnv(key.as_bytes(), 0x9e37_79b9_7f4a_7c15))
}

impl DiskCache {
    /// 打开（必要时创建）缓存目录，并把目录里已有的条目登记进来；总量超出上限时按旧到新淘汰。
    pub fn open(dir: &str, capacity: u64) -> anyhow::Result<Self> {
        let dir = PathBuf::from(dir);
        fs::create_dir_all(&dir)?;
        let mut found = Vec::new();
        for entry in fs::read_dir(&dir)?.flatten() {
            let path = entry.path();
            if path.extension().and_then(|e| e.to_str()) != Some("meta") {
                continue;
            }
            let stem = path.file_stem().and_then(|s| s.to_str()).unwrap_or_default().to_owned();
            let parsed = fs::read(&path).ok().and_then(|raw| serde_json::from_slice::<Meta>(&raw).ok());
            let size = fs::metadata(dir.join(format!("{stem}.body"))).map(|m| m.len()).ok();
            match (parsed, size) {
                (Some(meta), Some(size)) => {
                    let modified = entry.metadata().and_then(|m| m.modified()).ok();
                    found.push((modified, stem, Item { size, used: 0, site: meta.site }));
                }
                _ => {
                    let _ = fs::remove_file(&path);
                    let _ = fs::remove_file(dir.join(format!("{stem}.body")));
                }
            }
        }
        found.sort_by_key(|(modified, _, _)| *modified);
        let mut inner = Inner { items: HashMap::new(), bytes: 0, tick: 0 };
        for (_, stem, mut item) in found {
            inner.tick += 1;
            item.used = inner.tick;
            inner.bytes += item.size;
            inner.items.insert(stem, item);
        }
        let cache = Self { dir, capacity, inner: Mutex::new(inner) };
        cache.evict(&mut cache.inner.lock().unwrap(), "");
        Ok(cache)
    }

    fn remove_files(&self, stem: &str) {
        let _ = fs::remove_file(self.dir.join(format!("{stem}.meta")));
        let _ = fs::remove_file(self.dir.join(format!("{stem}.body")));
    }

    /// 总量超出上限时淘汰最久没用的（keep 指定的那一条除外）。
    fn evict(&self, inner: &mut Inner, keep: &str) {
        while inner.bytes > self.capacity {
            let Some(oldest) = inner.items.iter().filter(|(stem, _)| stem.as_str() != keep).min_by_key(|(_, item)| item.used).map(|(stem, _)| stem.clone()) else { break };
            if let Some(item) = inner.items.remove(&oldest) {
                inner.bytes -= item.size;
            }
            self.remove_files(&oldest);
        }
    }

    pub fn get(&self, key: &str) -> Option<Entry> {
        let stem = stem_of(key);
        if !self.inner.lock().unwrap().items.contains_key(&stem) {
            return None;
        }
        let meta: Option<Meta> = fs::read(self.dir.join(format!("{stem}.meta"))).ok().and_then(|raw| serde_json::from_slice(&raw).ok());
        let body = fs::read(self.dir.join(format!("{stem}.body"))).ok();
        let (Some(meta), Some(body)) = (meta, body) else {
            self.forget(&stem);
            return None;
        };
        if meta.key != key {
            return None;
        }
        if meta.expires <= now_secs() {
            self.forget(&stem);
            return None;
        }
        {
            let mut inner = self.inner.lock().unwrap();
            inner.tick += 1;
            let tick = inner.tick;
            if let Some(item) = inner.items.get_mut(&stem) {
                item.used = tick;
            }
        }
        let headers = meta.headers.iter().filter_map(|(name, value)| Some((HeaderName::from_bytes(name.as_bytes()).ok()?, HeaderValue::from_str(value).ok()?))).collect();
        Some(Entry { status: StatusCode::from_u16(meta.status).ok()?, headers, body: Bytes::from(body), expires: Instant::now(), site: meta.site })
    }

    fn forget(&self, stem: &str) {
        let mut inner = self.inner.lock().unwrap();
        if let Some(item) = inner.items.remove(stem) {
            inner.bytes -= item.size;
        }
        drop(inner);
        self.remove_files(stem);
    }

    pub fn put(&self, key: &str, entry: &Entry, ttl_seconds: u64) {
        let size = entry.body.len() as u64;
        if size > self.capacity {
            return;
        }
        let stem = stem_of(key);
        let meta = Meta {
            key: key.to_owned(),
            status: entry.status.as_u16(),
            headers: entry.headers.iter().filter_map(|(name, value)| Some((name.as_str().to_owned(), value.to_str().ok()?.to_owned()))).collect(),
            site: entry.site.clone(),
            expires: now_secs() + ttl_seconds,
        };
        let written = (|| -> std::io::Result<()> {
            let body_tmp = self.dir.join(format!("{stem}.body.tmp"));
            fs::write(&body_tmp, &entry.body)?;
            fs::rename(&body_tmp, self.dir.join(format!("{stem}.body")))?;
            let meta_tmp = self.dir.join(format!("{stem}.meta.tmp"));
            fs::write(&meta_tmp, serde_json::to_vec(&meta)?)?;
            fs::rename(&meta_tmp, self.dir.join(format!("{stem}.meta")))
        })();
        if let Err(error) = written {
            eprintln!("写媒体磁盘缓存失败：{error}");
            self.remove_files(&stem);
            return;
        }
        let mut inner = self.inner.lock().unwrap();
        if let Some(old) = inner.items.remove(&stem) {
            inner.bytes -= old.size;
        }
        inner.tick += 1;
        let used = inner.tick;
        inner.bytes += size;
        inner.items.insert(stem.clone(), Item { size, used, site: entry.site.clone() });
        self.evict(&mut inner, &stem);
    }

    fn purge_where(&self, matches: impl Fn(&Item) -> bool) -> usize {
        let mut inner = self.inner.lock().unwrap();
        let stems: Vec<String> = inner.items.iter().filter(|(_, item)| matches(item)).map(|(stem, _)| stem.clone()).collect();
        for stem in &stems {
            if let Some(item) = inner.items.remove(stem) {
                inner.bytes -= item.size;
            }
            self.remove_files(stem);
        }
        stems.len()
    }

    pub fn purge_site(&self, site: Option<&str>) -> usize {
        self.purge_where(|item| site.is_none_or(|s| item.site == s))
    }
}
