# nice-wiki 部署包

单机部署：16 个服务以 Docker 镜像运行（从 ghcr.io 拉取发布镜像，服务器上不用编译，也不需要 Go），前面由 Caddy 终止 HTTPS。原生 Agent 镜像自带 Node.js 与 ZCode 运行文件。

需要：Linux 服务器（amd64 或 arm64）、Docker 及 compose 插件、已解析到服务器的域名、可用的 80/443 端口。

## 首次部署

在解压出的目录里执行：

```sh
sh deploy/init-secrets.sh                 # 生成各服务密钥，写入 deploy/secrets（务必另行备份，丢了无法恢复数据）
cp deploy/.env.example deploy/.env        # 把 SITE_DOMAIN 改成你的域名；NW_VERSION 已是本包版本
docker compose -f deploy/docker-compose.yml up -d
```

然后用浏览器打开 `https://你的域名/admin`，选语言、注册超级管理员并填站名。

尚未开放公网、只经 SSH 隧道访问时，把 `deploy/.env` 里的 `NW_BIND` 改成 `127.0.0.1`。

## 升级

数据在 Docker 卷里，不随目录变化。下载新版本的部署包并解压，把旧目录里的 `deploy/secrets`、`deploy/.env`、`deploy/caddy/conf.d` 拷到新目录，
把 `deploy/.env` 里的 `NW_VERSION` 改成新版本，再在新目录执行 `docker compose -f deploy/docker-compose.yml up -d`。

有新增服务时，还需生成该服务的签名身份，并把新公钥追加到已有 `deploy/config/trust-anchors.json`；旧私钥、公钥、模型配置及数据卷都要保留。v0.1.41 新增 `agent` 服务，其镜像自带 Node.js 24 和 ZCode 运行文件。

## 其他

- 备份与恢复：`deploy/backup.sh`、`deploy/restore.sh`、`deploy/restore-archive.sh`，说明见源码仓库的 `deploy/README.md`；放在 Cloudflare 后面等定制也在那里。
- `bin/`、`web/` 与 `agent/` 是同版本运行产物；不用 Docker 部署时，Agent 另需 Node.js 24，并把 `NW_ZCODE_WORKER` 指向 `agent/worker.mjs` 的绝对路径。
