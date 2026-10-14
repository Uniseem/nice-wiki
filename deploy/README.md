# 部署编排（L02）

单机部署的唯一配置来源。决策见 [ADR0005](../docs/adr/0005-engineering-and-deployment.md)，运维要求见 [部署与运维](../docs/operations.md)。

- `docker-compose.yml`：15 个服务与 Caddy；`restart: unless-stopped`，恢复切换依赖服务以退出码 75 退出后自动拉起。
- `caddy/Caddyfile`：只在这里终止 HTTPS；`/api/*`、`/admin*`、`/ui/*` 到 portal，其余到 public-site。
- `config/services.json`：服务地址、端口、数据卷与依赖的唯一来源。
- `config/trust-anchors.json`：由 `nw-keys` 生成的公钥表，不含私钥。
- `.env.example`：域名、站点时区、镜像版本模板，不含密钥。

当前为设计阶段，以上文件只在 [机器合同](contracts/implementation.json) 中登记，尚未创建。文件目录见 [docs/files.md](docs/files.md)，进度见 [progress.json](progress.json)。
