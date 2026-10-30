# 部署编排

单机部署的唯一配置来源，拓扑见 [docs/architecture-v2.md](../docs/architecture-v2.md) 第 14 节。

- `docker-compose.yml`、各进程的 `Dockerfile`（`services/<进程>/`、`rust/`）：由 `scripts/gen_deploy.py` 生成，不要手改。
- `release/`：发布包用的编排（拉取 ghcr.io 镜像）、说明和镜像规格 `images.tsv`，同样由 `scripts/gen_deploy.py` 生成。发版由 GitHub Actions（`.github/workflows/release.yml`）在推送 `v*` 标签时执行：Rust 程序在原生 amd64、arm64 机器上编成 musl 静态程序，Go 程序交叉编译，再用 crane 组装并推送镜像、打发布包，用 Actions 自带令牌，不需要另配密钥。
- `caddy/Caddyfile`：Caddy 终止 HTTPS，证书按需签发——先问 `edge` 的 `/_tls/allow`（它再问 identity），只给平台域名、`SITE_DOMAIN`、已验证的自定义域名和平台子域名签。流量交给 `edge`（Rust）：它是唯一入口，以 portal 的服务身份（`config/portal.json`、`secrets/portal`）鉴权并转发到各服务，提供后台静态页，做页面缓存与限流。
- `postgres/init/`：首次启动数据库时建非超级用户的应用角色（行级安全对它生效）并装 pgvector。
- `config/*.json`：各组件的运行配置；`config/trust-anchors.json` 由 `nw-keys` 生成，只含公钥。
- `init-secrets.sh`：首次部署生成各组件的签名密钥、主密钥、数据库与检索口令（写入 `deploy/secrets`，已被 git 忽略）。
- `backup.sh` / `restore.sh`：整站备份与恢复。`backup.sh` 在线 `pg_dump` 导出数据库，增量复制正文与媒体（第二次起只复制新增的），再打包其余文件卷和密钥；`BACKUP_PASSPHRASE_FILE` 给出口令文件就加密数据库导出和密钥包，`BACKUP_REMOTE` 给出 rclone 目标（S3、WebDAV、各家网盘都行）就在备份后同步到异地。`restore.sh <备份目录> <时间戳>` 先停机，恢复要同一套密钥，会覆盖当前数据。用户各自站点的内容备份在后台“数据与同步”里配置，不在这里。

## 进程与端口（容器内部）

| 进程 | 组件 |
|---|---|
| `account` | site-config 8102、identity 8101 |
| `content` | content 8103、media 8104、localization 8108、public-site 8081 |
| `ai` | model-gateway 8105、qa 8107、agent 8114（内含 Node 进程）、memory 8109、knowledge 8106 |
| `ops` | jobs 8111、analytics 8112、traffic-guard 8110 |
| `search` 8120、`edge` 8090（portal 的服务身份）、`postgres`、`caddy` | |

## 用发布包部署（推荐）

在 GitHub Releases 下载对应架构的 `nice-wiki-<版本>-linux-<amd64|arm64>.tar.gz` 并解压，服务以 ghcr.io 上的发布镜像运行，服务器上不编译、不需要 Go：

```bash
sh deploy/init-secrets.sh
cp deploy/.env.example deploy/.env   # 改域名；NW_VERSION 已是发布包版本
docker compose -f deploy/docker-compose.yml up -d
```

升级和其他说明见发布包里的 `README.md`（即 `release/README.md`）。

## 从源码构建部署

需要 Go（生成密钥用）和足够内存（编译 8 个镜像，Rust 部分较吃内存）。首次部署（需要用户当次同意才能在服务器上执行）：

```bash
sh deploy/init-secrets.sh
cp deploy/.env.example deploy/.env   # 改域名与时区，不进 git
docker compose -f deploy/docker-compose.yml up -d --build
```

内存不足 4GB 的机器上，`--build` 同时编译多个进程会把编译器 OOM 杀掉，改为逐个构建后再启动：

```bash
for s in postgres account content ai ops search edge; do docker compose -f deploy/docker-compose.yml build "$s" 2>/dev/null || true; done
docker compose -f deploy/docker-compose.yml up -d
```

备份必须连同 `deploy/secrets` 一起保管（含主密钥，没有它加密保存的第三方登录凭证、用户自带的 AI 密钥和备份凭据都解不开）。

## 域名

- 平台域名（用户登录、管理站点）与用户站点的子域名后缀在后台“平台管理”里填；`SITE_DOMAIN` 只用来签第一张证书。
- 用户站点的子域名需要把泛域名（`*.后缀`）解析到本机；用户自己绑定的域名由用户在域名服务商处解析到本机，再在后台验证。
- 用户把备份存到自己的网盘（Google Drive、OneDrive、Dropbox）需要站主先在三家各创建一个 OAuth 应用，回调地址填 `https://<平台域名>/admin/oauth/<gdrive|onedrive|dropbox>`，并只申请应用文件夹权限；Client ID 与密钥在后台“平台管理 → 设置 → 网盘备份”里填。没填的网盘，用户那边的授权会被拒绝。
- Let's Encrypt 对同一注册域名每周最多签 50 张新证书；用户子域名多了以后改用带 DNS 模块的 Caddy 签泛域名证书（在 `deploy/caddy/conf.d/` 里加本地定制）。

## 放在 Cloudflare 后面（小黄云，只代理平台自己的域名）

在部署机的 `deploy/caddy/conf.d/` 下放两个本地文件（不进 git），并把 Cloudflare 源站证书放到可挂载的位置：

- `global-cloudflare.caddy`：`servers { trusted_proxies static <Cloudflare 的 IP 段> ; client_ip_headers CF-Connecting-IP }`，让 Caddy 算出真实访客 IP；自带的 Caddyfile 再用 `header_up X-Forwarded-For {client_ip}` 只把这个 IP 交给 edge（防滥用计数和问答的匿名访客身份都按它）。
- `site-cloudflare.caddy`：只放行 Cloudflare 回源（`@notcf not remote_ip <Cloudflare 的 IP 段>` + `respond @notcf 403`）。

Cloudflare 的 SSL 模式用“完全（Full）”；`deploy/.env` 里 `NW_BIND=0.0.0.0`。用自己的反向代理（不用自带 Caddy）时同理：转给 edge 的 `X-Forwarded-For` 只放真实访客 IP，末尾如果是 Cloudflare 边缘 IP，访客身份会随边缘轮换而变，人机验证和问答会报 FORBIDDEN。
