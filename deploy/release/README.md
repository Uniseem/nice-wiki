# nice-wiki 部署包

单机部署：公开入口（Caddy 按需证书 → edge）、account、content、ai、ops、search 与带 pgvector 的 PostgreSQL，全部以 Docker 镜像运行。
服务器上不编译、不需要 Go。

需要：Linux 服务器（amd64 或 arm64）、Docker 及 compose 插件、已解析到服务器的域名、可用的 80/443 端口。
用户站点的子域名需要把泛域名（`*.你的子域名后缀`）解析到本机；用户自己绑定的域名，由用户在自己的域名服务商处解析到本机后在后台验证。

## 首次部署

```sh
sh deploy/init-secrets.sh                 # 生成各服务密钥、主密钥与数据库口令，写入 deploy/secrets（务必另行备份）
cp deploy/.env.example deploy/.env        # 把 SITE_DOMAIN 改成你的域名；NW_VERSION 已是本包版本
docker compose -f deploy/docker-compose.yml up -d
```

然后用浏览器打开 `https://你的域名/admin`，选语言、注册站主并填站名，再到“平台管理”填写平台域名与站点子域名后缀、登录方式等。
注册站主要填安装令牌（只有登得上服务器的人拿得到，防止别人抢先注册），在服务器上查看：

```sh
docker compose -f deploy/docker-compose.yml exec account cat /data/identity/control/setup-token
```

## 升级

数据在 Docker 卷里，不随目录变化。升级前先跑一次 `deploy/backup.sh`：数据库结构随新版本迁移后不能退回旧版本。
下载新版本的部署包并解压，把旧目录里的 `deploy/secrets`、`deploy/config/trust-anchors.json`、`deploy/.env`、`deploy/caddy/conf.d` 拷到新目录，
把 `deploy/.env` 里的 `NW_VERSION` 改成新版本，再在新目录执行 `docker compose -f deploy/docker-compose.yml up -d`。数据库结构随服务启动自动迁移。

## 备份

`deploy/backup.sh` 在线导出数据库、增量复制正文与媒体、打包其余文件卷与密钥，可选用口令加密（`BACKUP_PASSPHRASE_FILE`）并用 rclone 同步到异地（`BACKUP_REMOTE`，支持 S3、WebDAV、各家网盘）；`deploy/restore.sh` 恢复（先停机）。用户各自站点的内容备份在后台“数据与同步”里配置。
