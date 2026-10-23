# 部署编排

单机部署的唯一配置来源。决策见 [ADR0005](../docs/adr/0005-engineering-and-deployment.md)，运维要求见 [部署与运维](../docs/operations.md)。

- `docker-compose.yml`、各域 `modules/*/Dockerfile`：由 `scripts/gen_deploy.py` 生成，不要手改。
- `release/`：发布包用的编排（拉取 ghcr.io 镜像）、说明和镜像规格 `images.tsv`，同样由 `scripts/gen_deploy.py` 生成。发版由 GitHub Actions（`.github/workflows/release.yml`）在推送 `v*` 标签时执行 `scripts/release.sh`：交叉编译、用 crane 组装并推送 amd64/arm64 镜像、打发布包，再发布到 GitHub Releases，用 Actions 自带令牌，不需要另配密钥。
- `caddy/Caddyfile`：只在这里终止 HTTPS，全部流量交给 portal（访客页面、后台和接口都经它代理）。
- `config/*.json`：各服务的运行配置；`config/trust-anchors.json` 由 `nw-keys` 生成，只含公钥。
- `init-secrets.sh`：首次部署生成各服务的签名密钥（写入 `deploy/secrets`，已被 git 忽略）。
- `backup.sh` / `restore.sh`：整站停机备份与恢复（含全部卷，恢复用）；日常也可在后台“备份与恢复”页在线创建 tar.gz 备份（data-protection 读取各服务数据卷：先依次快照全部 SQLite 库，再复制文件，库里引用的文件一定在；不暂停写入，各库不是同一时点，备份中的保存/发布可能只进一部分。默认不含密钥，超管可勾选包含），恢复用 `restore-archive.sh <tar.gz>`。迁移服务器等需要严格一致时用 `backup.sh`。

## 用发布包部署（推荐）

在 GitHub Releases 下载对应架构的 `nice-wiki-<版本>-linux-<amd64|arm64>.tar.gz` 并解压，服务以 ghcr.io 上的发布镜像运行，服务器上不编译、不需要 Go：

```bash
sh deploy/init-secrets.sh
cp deploy/.env.example deploy/.env   # 改域名；NW_VERSION 已是发布包版本
docker compose -f deploy/docker-compose.yml up -d
```

升级和其他说明见发布包里的 `README.md`（即本目录的 `release/README.md`）。

## 从源码构建部署

需要 Go（生成密钥用）和足够内存（在服务器上编译 15 个镜像）。首次部署（需要用户当次同意才能在服务器上执行）：

```bash
sh deploy/init-secrets.sh
cp deploy/.env.example deploy/.env   # 改域名与时区，不进 git
docker compose -f deploy/docker-compose.yml up -d --build
```

内存不足 4GB 的机器上，`--build` 同时编译多个服务会把 Go 编译器 OOM 杀掉，改为逐个构建后再启动：

```bash
for s in portal public-site identity site-config content media model-gateway knowledge qa localization memory traffic-guard jobs analytics data-protection; do docker compose -f deploy/docker-compose.yml build "$s"; done
docker compose -f deploy/docker-compose.yml up -d
```

备份必须连同 `deploy/secrets` 一起保管，没有密钥无法恢复。

## 放在 Cloudflare 后面（小黄云）

在部署机的 `deploy/caddy/conf.d/` 下放两个本地文件（不进 git），并把 Cloudflare 源站证书放到 `deploy/secrets/tls/origin.pem`、`origin.key`：

- `global-cloudflare.caddy`：`servers { trusted_proxies static <Cloudflare 的 IP 段> ; client_ip_headers CF-Connecting-IP }`，让 Caddy 算出真实访客 IP；自带的 Caddyfile 再用 `header_up X-Forwarded-For {client_ip}` 只把这个 IP 交给 portal（防滥用计数和问答的匿名访客身份都按它）。
- `site-cloudflare.caddy`：`tls /run/tls/origin.pem /run/tls/origin.key`，再用 `@notcf not remote_ip <Cloudflare 的 IP 段>` + `respond @notcf 403` 只放行 Cloudflare 回源。

Cloudflare 的 SSL 模式用“完全（Full）”。`deploy/.env` 里 `SITE_DOMAIN` 填域名，`NW_BIND=0.0.0.0`。

用自己的反向代理（不用自带 Caddy）时同理：转给 portal 的 `X-Forwarded-For` 只放真实访客 IP。如果末尾是 Cloudflare 边缘 IP，访客身份会随边缘轮换而变，人机验证和问答会报 FORBIDDEN。
