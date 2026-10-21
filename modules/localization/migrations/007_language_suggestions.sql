-- 语言建议的模型判断结果按信号组合缓存：同样的浏览器语言、时区、国家、当前语言和站点语言集合直接复用，不再调用模型。
CREATE TABLE language_suggestions (
 cache_key TEXT PRIMARY KEY,
 data_epoch TEXT NOT NULL,
 language TEXT NOT NULL,
 confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
 created_at TEXT NOT NULL
);
