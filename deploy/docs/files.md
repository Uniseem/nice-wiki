# deploy 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 5 个：生成 1、配置 4。

| 路径 | 类别 | 职责 |
|---|---|---|
| `deploy/docker-compose.yml` | 配置 | 15个服务与Caddy的编排：restart unless-stopped；每服务独占数据卷/data/<slug>（active、control、restore）；备份卷/backups只挂data-protection；内部网络；GOMEMLIMIT按operations.md；secret以文件挂载 |
| `deploy/caddy/Caddyfile` | 配置 | HTTPS终止与路由：/api/*、/admin*、/ui/*→portal:8080，其余→public-site:8081；只信任本机Caddy转发头；压缩与静态缓存头 |
| `deploy/config/services.json` | 配置 | 服务清单唯一来源：slug→内部URL、端口、数据卷与依赖；各服务ServiceConfig.dependencies与nw-keys从此读取 |
| `deploy/config/trust-anchors.json` | 生成 | 由nw-keys生成的服务与portal委托公钥表；不含私钥 |
| `deploy/.env.example` | 配置 | 部署变量模板（域名、站点时区、镜像版本）；不含任何密钥 |
