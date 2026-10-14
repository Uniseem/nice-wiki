# deploy 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 5 个：生成 1、配置 4。函数 0 个：约束 0，参考 0。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `deploy/docker-compose.yml` | 配置 | 15个服务与Caddy的编排：restart unless-stopped；每服务独占数据卷/data/<slug>（active、control、restore）；备份卷/backups只挂data-protection；内部网络；GOMEMLIMIT按operations.md；secret以文件挂载 | 无 |
| `deploy/caddy/Caddyfile` | 配置 | HTTPS终止与路由：/api/*、/admin*、/ui/*→portal:8080，其余→public-site:8081；只信任本机Caddy转发头；压缩与静态缓存头 | 无 |
| `deploy/config/services.json` | 配置 | 服务清单唯一来源：slug→内部URL、端口、数据卷与依赖；各服务ServiceConfig.dependencies与nw-keys从此读取 | 无 |
| `deploy/config/trust-anchors.json` | 生成 | 由nw-keys生成的服务与portal委托公钥表；不含私钥 | 无 |
| `deploy/.env.example` | 配置 | 部署变量模板（域名、站点时区、镜像版本）；不含任何密钥 | 无 |
