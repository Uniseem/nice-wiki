# 部署与 HTTPS

当前实现已接通文档站、后台编辑发布、受保护的 AI 问答、目标语言翻译、匿名统计、自动备份与容器部署，但 P4～P6 仍在逐项业务验收；未通过验收的能力不要直接视为生产就绪。

## 构建

优先在准备好的 VPS 执行：

```sh
make vps-build
```

也可在已安装 Go、Node.js 和 pnpm 的环境执行 `make build`。产物为 `bin/nice-wiki`；前端资源已经嵌入该二进制。部署时只需要二进制及一个可持久化的数据目录。

## 首次启动

```sh
./nice-wiki --data /srv/nice-wiki/data --addr 127.0.0.1:8080
```

环境变量 `NICE_WIKI_DATA`、`NICE_WIKI_ADDR`、`NICE_WIKI_LOG_LEVEL` 和 `NICE_WIKI_DEV` 可覆盖配置文件；同名命令行参数优先级最高。

> **必须立即完成超级管理员注册。** 第一个打开后台并注册的账号会成为唯一超级管理员。注册完成前不要把服务端口直接暴露到公网；可先使用防火墙或反向代理限制 `/admin/`。

注册后按向导填写站点名称和基准语言即可完成初始化。AI 渠道和档位在独立模型管理模块中后续配置，未配置时 AI 保持关闭，文档管理和公开站点可正常使用。此为 [D55](../docs/adr/0055-setup-without-ai.md) 确认的新流程，旧代码待 P2-02 调整，尚未部署。

启用模型目录自动更新或语言建议时，服务会分别访问 models.dev、DB-IP Lite 公开下载地址和已配置模型渠道。DB-IP 数据只保存在 `data/cache/geoip/`，访客原始 IP 不落库。

## 反向代理与 HTTPS

应用只监听 HTTP，TLS/HTTPS 交给 Caddy、Nginx 等反向代理。代理应：

- 将所有路径原样转发到应用；
- 保留 `Host`；
- 只在代理地址已加入后台“可信代理”设置后发送并信任 `X-Forwarded-For`；
- 不缓存 `/admin/` 和 `/api/`；哈希静态资源可按响应头长期缓存；
- 限制请求体大小，并为上传资源预留足够超时时间。

未登记为可信代理的来源头不会影响访客 IP。不要把密钥、账号文件、运行日志或真实访客数据加入代码仓库。

仓库提供 `deploy/Dockerfile`、`docker-compose.yml`、`Caddyfile.example` 和 `nginx.conf.example`。替换示例域名后再部署；只有确认过边界的代理地址才能加入可信代理。SSE 必须关闭响应缓冲。

## 二进制与 Caddy 常驻部署

直连域名已解析到服务器、没有 CDN 时，公开入口使用标准 80/443 端口，应用只监听 `127.0.0.1:18080`。Caddy 为实际域名自动申请、续期公开可信证书，并将 HTTP 重定向到 HTTPS；后台和问答 Cookie 保持 `Secure`。先通过域名 HTTPS 验证，再开放演示入口，不使用开发模式或跳过证书校验。

推荐运行布局：

- `/opt/nice-wiki-demo/releases/<源码版本>/nice-wiki`：不可变发布二进制及来源版本记录。
- `/opt/nice-wiki-demo/current`：指向当前发布目录的符号链接。
- `/srv/nice-wiki-demo/data`：独立持久化数据，所属用户 `nice-wiki-demo`，不与构建缓存混用。
- `/etc/systemd/system/nice-wiki-demo.service`：非 root 常驻服务。
- `/etc/caddy/Caddyfile`：实际域名的私有部署配置；仓库示例只用 `wiki.example.com`。

系统服务的核心配置如下，创建运行账号和数据目录后再启用：

```ini
[Unit]
Description=nice-wiki demo
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=nice-wiki-demo
Group=nice-wiki-demo
WorkingDirectory=/srv/nice-wiki-demo
ExecStart=/opt/nice-wiki-demo/current/nice-wiki --data /srv/nice-wiki-demo/data --addr 127.0.0.1:18080 --log-level info
Environment=GOMAXPROCS=8
Environment=GOMEMLIMIT=512MiB
Restart=on-failure
RestartSec=3
TimeoutStopSec=45
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/srv/nice-wiki-demo/data
MemoryMax=768M

[Install]
WantedBy=multi-user.target
```

将 `Caddyfile.example` 的域名换成实际值，保持后端端口一致；`baseURL` 设置为实际 HTTPS 根地址，可信代理仅加入 `127.0.0.1/32`。反代使用真实连接来源覆盖转发头，关闭问答压缩并即时刷新 SSE；不信任公网提交的来源头。

首次启动时先保持应用只在本机监听，通过受控入口注册超管后再启动公开反代。自动初始化使用随机临时密码时，凭据只保存到受限私有文件，并要求首次登录修改；不能写入源码、会话记录或发布附件。按 D55，没有模型渠道也能结束初始化，AI 保持关闭，不填假渠道。

```sh
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl daemon-reload
systemctl enable --now nice-wiki-demo caddy
curl --fail --silent --show-error https://wiki.example.com/healthz
curl --head http://wiki.example.com/admin
```

验证应包括：公开证书链与域名校验通过、HTTP 308 保留路径/查询、根路径转到基准语言、首页/后台及静态资源返回成功、HTTPS 登录 Cookie 包含 `Secure`/`HttpOnly`、注册入口在已有超管后关闭。记录验证结果即可，不新增测试框架，也不自动调用真实模型。

后续切换版本先备份 data，更新 `current` 后重启 `nice-wiki-demo`，通过 HTTPS 再核对健康与页面。若数据格式没有变化，可以切回旧二进制；涉及迁移时按下文使用对应完整备份恢复。Caddy 的证书数据需保留，不在普通代码更新时删除。

## 健康检查与升级

`GET /healthz` 返回成功即表示 HTTP 服务已启动。升级前使用后台备份功能创建一致备份，并把备份下载到其他存储位置。不要把正在写入的 SQLite 文件直接复制后当作一致备份。

升级后运行一次健康检查，并人工确认后台登录、草稿保存、发布页面和 `/<基准语言>/sitemap.xml`。回滚二进制前先阅读版本说明，避免让旧程序读取更高版本的数据格式。

迁移失败时程序会停止启动，不会跳过失败版本继续运行。应保存现场和错误日志，再恢复升级前的完整备份；不要只替换数据库或正文文件。已经被新版本迁移的数据目录不能交给旧二进制直接读取。

## 数据卷内备份恢复

恢复只在 `/data` 内建立 `.restore-stage-*` 暂存目录，替换其中的内容条目，保持挂载根目录和进程锁不变。备份包必须包含可读的 SQLite 快照与有效正文；路径穿越、链接、运行锁和 WAL/SHM 会被拒绝。

重新启动时先取得数据目录锁，再应用待恢复包。切换日志 `.restore-journal.json` 在所有配置和索引初始化成功后才移除；切换或初始化失败会回滚，进程中断后的下次启动也先恢复原数据。成功后原数据保存在 `.restore-previous/`，原备份暂存目录保留；再次恢复前，停止服务并将这两处旧数据移到独立安全存储。旧数据可能包含密钥，不得上传代码仓库。

容器以 UID/GID `65532` 运行，新建命名卷的 `/data` 由镜像预置所有权。使用已有卷或主机目录挂载时，应事先保证该用户有读写权限。恢复未包含密钥的备份后，需要在后台补填模型和其他服务密钥；不会自动重算付费派生数据。
