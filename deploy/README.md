# 部署编排

单机部署的唯一配置来源。决策见 [ADR0005](../docs/adr/0005-engineering-and-deployment.md)，运维要求见 [部署与运维](../docs/operations.md)。

- `docker-compose.yml`、各域 `modules/*/Dockerfile`：由 `scripts/gen_deploy.py` 生成，不要手改。
- `caddy/Caddyfile`：只在这里终止 HTTPS，全部流量交给 portal（访客页面、后台和接口都经它代理）。
- `config/*.json`：各服务的运行配置；`config/trust-anchors.json` 由 `nw-keys` 生成，只含公钥。
- `init-secrets.sh`：首次部署生成各服务的签名密钥（写入 `deploy/secrets`，已被 git 忽略）。
- `backup.sh` / `restore.sh`：整站停机备份与恢复（含全部卷，恢复用）；日常也可在后台“备份与恢复”页在线创建 tar.gz 备份（data-protection 读取各服务数据卷，SQLite 取一致快照，默认不含密钥，超管可勾选包含），恢复用 `restore-archive.sh <tar.gz>`。

首次部署（需要用户当次同意才能在服务器上执行）：

```bash
sh deploy/init-secrets.sh
cp deploy/.env.example deploy/.env   # 改域名与时区，不进 git
docker compose -f deploy/docker-compose.yml up -d --build
```

备份必须连同 `deploy/secrets` 一起保管，没有密钥无法恢复。

## 放在 Cloudflare 后面（小黄云）

在部署机的 `deploy/caddy/conf.d/` 下放两个本地文件（不进 git），并把 Cloudflare 源站证书放到 `deploy/secrets/tls/origin.pem`、`origin.key`：

- `global-cloudflare.caddy`：`servers { trusted_proxies static <Cloudflare 的 IP 段> ; client_ip_headers CF-Connecting-IP }`，让 portal 拿到真实访客 IP（防滥用按它计数）。
- `site-cloudflare.caddy`：`tls /run/tls/origin.pem /run/tls/origin.key`，再用 `@notcf not remote_ip <Cloudflare 的 IP 段>` + `respond @notcf 403` 只放行 Cloudflare 回源。

Cloudflare 的 SSL 模式用“完全（Full）”。`deploy/.env` 里 `SITE_DOMAIN` 填域名，`NW_BIND=0.0.0.0`。
