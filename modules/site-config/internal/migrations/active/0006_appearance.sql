-- 站点外观（docs/theme-standard.md）：本站装的外观包与当前选择。内置外观随程序发布，不进库。
-- package 存校验后重新打包的 zip，按摘要（只由文件内容决定）去重；site_appearance 同时记 id，内置外观随程序升级换了内容时按 id 接上新版。
CREATE TABLE theme_packages (
    site_id      TEXT NOT NULL,
    digest       TEXT NOT NULL,
    theme_id     TEXT NOT NULL,
    version      TEXT NOT NULL,
    manifest     TEXT NOT NULL,
    package      BLOB NOT NULL,
    byte_size    INTEGER NOT NULL CHECK (byte_size > 0),
    installed_at TEXT NOT NULL,
    PRIMARY KEY (site_id, digest)
);

CREATE TABLE site_appearance (
    site_id      TEXT PRIMARY KEY,
    theme_id     TEXT NOT NULL,
    theme_digest TEXT NOT NULL,
    color_scheme TEXT NOT NULL CHECK (color_scheme IN ('system', 'light', 'dark')),
    updated_at   TEXT NOT NULL
);
