#!/usr/bin/env python3
"""生成部署文件：各域 Dockerfile、deploy/docker-compose.yml、Caddyfile、services.json、各服务配置与 .env.example。
服务清单只在本文件的 SERVICES 里维护，改完后重新运行即可。"""
import json, os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SERVICES = {  # slug: 端口
    "portal": 8080, "public-site": 8081, "identity": 8101, "site-config": 8102, "content": 8103, "media": 8104,
    "model-gateway": 8105, "knowledge": 8106, "qa": 8107, "localization": 8108, "memory": 8109,
    "traffic-guard": 8110, "jobs": 8111, "analytics": 8112, "data-protection": 8113,
}
WEB_MODULES = ["content", "site-config", "identity", "media", "localization", "model-gateway", "knowledge", "memory", "qa", "traffic-guard", "jobs", "analytics", "data-protection"]
SUBNET = "198.51.100.0/24"  # 文档保留地址段，仅作内部桥接网络的子网


def write(path, text):
    full = os.path.join(ROOT, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    open(full, "w").write(text)


def dockerfile(slug, port):
    uid = 10000 + list(SERVICES).index(slug)
    web = ""
    copy_web = ""
    if slug == "portal":
        builds = "\n".join(
            f"COPY modules/{m}/web/ modules/{m}/web/\nRUN cd modules/{m}/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/{m} --emptyOutDir"
            for m in WEB_MODULES)
        web = f"""FROM node:24-slim AS web
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY libs/platform/web/ libs/platform/web/
COPY modules/portal/web/ modules/portal/web/
RUN cd modules/portal/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web --emptyOutDir
{builds}

"""
        copy_web = "COPY --from=web /out/web /web\nCOPY modules/portal/config/routes.json /config/routes.json\nENV NW_WEB_DIR=/web\n"
    return f"""# {slug} 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
{web}FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/{slug}/ modules/{slug}/
WORKDIR /src/modules/{slug}
RUN CGO_ENABLED=0 go build -trimpath -o /out/{slug} ./cmd/{slug}

FROM alpine:3.20
RUN adduser -D -u {uid} {slug} && mkdir -p /data/{slug} && chown {uid}:{uid} /data/{slug}
COPY --from=build /out/{slug} /usr/local/bin/{slug}
{copy_web}USER {uid}:{uid}
EXPOSE {port}
VOLUME ["/data/{slug}"]
WORKDIR {"/" if slug == "portal" else "/config"}
ENTRYPOINT ["/usr/local/bin/{slug}"]
CMD ["--config", "/config/{slug}.json"]
"""


def service_config(slug, port):
    cfg = {
        "module": slug, "listen": f"0.0.0.0:{port}", "dataRoot": f"/data/{slug}",
        "trustAnchorRef": "/run/secrets/trust-anchors.json", "signingKeyRef": "/run/secrets/signing-key.json",
        "dependencies": {s: f"http://{s}:{p}" for s, p in SERVICES.items() if s != slug},
    }
    if slug in ("portal", "traffic-guard"):
        cfg["budgetKeyRef"] = "/run/secrets/budget-key.json"
    if slug == "portal":
        cfg["delegationKeyRef"] = "/run/secrets/delegation-key.json"
        cfg["trustedProxyCIDRs"] = [SUBNET]
    if slug == "analytics":
        cfg["reportingTimeZone"] = "Asia/Shanghai"
    if slug in ("model-gateway", "data-protection"):
        cfg["masterKeyRef"] = "/run/secrets/master-key"
    if slug == "data-protection":
        cfg["backupSources"] = {s: f"/sources/{s}" for s in SERVICES if s != slug}
        cfg["backupSecretsDir"] = "/backup-secrets"
    return cfg


def compose():
    lines = ["# 单机部署编排：由 scripts/gen_deploy.py 生成，勿手改。", "name: nice-wiki", "", "networks:", "  internal:", "    driver: bridge",
             "    ipam:", "      config:", f"        - subnet: {SUBNET}", "", "services:"]
    for slug, port in SERVICES.items():
        lines += [f"  {slug}:", f"    build:", "      context: ..", f"      dockerfile: modules/{slug}/Dockerfile",
                  f"    image: nice-wiki/{slug}:${{NW_VERSION:-dev}}", "    restart: unless-stopped", "    networks: [internal]",
                  "    environment:", "      GOMEMLIMIT: ${GOMEMLIMIT:-256MiB}"]
        if slug in ("model-gateway", "data-protection"):
            # 主密钥文件要求权限 0600（只有属主可读），容器里的普通服务用户读不到，这两个服务以 root 运行。
            lines.insert(len(lines) - 3, '    user: "0:0"')
        if slug in ("portal", "identity", "site-config"):
            lines += ["      NW_ALLOW_LOCAL_HTTP: \"false\"", "      NW_PUBLIC_BASE_URL: https://${SITE_DOMAIN}" if slug != "site-config" else "      NW_PUBLIC_BASE_URL: https://${SITE_DOMAIN}"]
        lines += ["    volumes:", f"      - ./config/{slug}.json:/config/{slug}.json:ro",
                  f"      - ./secrets/{slug}/signing-key.json:/run/secrets/signing-key.json:ro",
                  f"      - ./secrets/{slug}/signing.pem:/run/secrets/signing.pem:ro",
                  "      - ./config/trust-anchors.json:/run/secrets/trust-anchors.json:ro",
                  f"      - {slug}-data:/data/{slug}"]
        if slug == "portal":
            lines += ["      - ./secrets/portal/delegation-key.json:/run/secrets/delegation-key.json:ro", "      - ./secrets/portal/delegation.pem:/run/secrets/delegation.pem:ro"]
        if slug in ("portal", "traffic-guard"):
            lines += ["      - ./secrets/budget/budget-key.json:/run/secrets/budget-key.json:ro", "      - ./secrets/budget/budget.pem:/run/secrets/budget.pem:ro"]
        if slug in ("model-gateway", "data-protection"):
            lines += [f"      - ./secrets/{slug}/master-key:/run/secrets/master-key:ro"]
        if slug == "data-protection":
            # 简易备份只读取各服务数据目录（代码里只读打开，SQLite 用 VACUUM INTO 取快照）；WAL 库需要能访问 -shm，所以不加 :ro。
            lines += ["      - backups:/backups", "      - ./secrets:/backup-secrets:ro"] + [f"      - {s}-data:/sources/{s}" for s in SERVICES if s != slug]
        lines.append("")
    lines += ["  caddy:", "    image: caddy:2-alpine", "    restart: unless-stopped", "    networks: [internal]",
              "    ports: [\"${NW_BIND:-0.0.0.0}:80:80\", \"${NW_BIND:-0.0.0.0}:443:443\"]", "    environment:", "      SITE_DOMAIN: ${SITE_DOMAIN}",
              "    volumes:", "      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro", "      - ./caddy/conf.d:/etc/caddy/conf.d:ro", "      - ./secrets/tls:/run/tls:ro", "      - caddy-data:/data", "      - caddy-config:/config",
              "    depends_on: [portal]", "", "volumes:"]
    lines += [f"  {slug}-data:" for slug in SERVICES] + ["  backups:", "  caddy-data:", "  caddy-config:", ""]
    return "\n".join(lines)


def main():
    for slug, port in SERVICES.items():
        write(f"modules/{slug}/Dockerfile", dockerfile(slug, port))
        write(f"deploy/config/{slug}.json", json.dumps(service_config(slug, port), indent=2, ensure_ascii=False) + "\n")
    write("deploy/docker-compose.yml", compose())
    write("deploy/caddy/Caddyfile", """# 只有 portal 对外：访客页面、后台和接口都经它代理到各服务。
# 部署相关的定制（例如放在 Cloudflare 后面：源站证书、只放行 Cloudflare 回源、取真实访客 IP）
# 写成 conf.d 下的 global-*.caddy / site-*.caddy，本地文件不进 git，见 deploy/README.md。
{
	import /etc/caddy/conf.d/global-*.caddy
}
{$SITE_DOMAIN} {
	import /etc/caddy/conf.d/site-*.caddy
	encode zstd gzip
	header {
		Strict-Transport-Security "max-age=31536000"
		X-Content-Type-Options nosniff
		Referrer-Policy strict-origin-when-cross-origin
	}
	reverse_proxy portal:8080
}
""")
    write("deploy/config/services.json", json.dumps({"version": 1, "services": [
        {"slug": s, "url": f"http://{s}:{p}", "port": p, "dataVolume": f"/data/{s}", "secretsDir": f"deploy/secrets/{s}"} for s, p in SERVICES.items()]}, indent=2, ensure_ascii=False) + "\n")
    write(".dockerignore", """# 构建上下文是仓库根：不带上本机的依赖目录、构建产物、开发数据和密钥。
**/node_modules
**/dist
.git
.local
deploy/secrets
deploy/.env
vps.txt
""")
    write("deploy/.env.example", """# 部署变量模板；不含任何密钥。复制为 .env 后修改。
SITE_DOMAIN=docs.example.com
NW_VERSION=dev
GOMEMLIMIT=256MiB
# 对外监听地址：默认全部网卡；尚未开放公网、只经 SSH 隧道访问时改成 127.0.0.1。
# Docker 发布的端口不经过宿主机 INPUT 防火墙规则，要限制访问必须在这里绑定。
NW_BIND=0.0.0.0
""")
    write("deploy/init-secrets.sh", """#!/bin/sh
# 首次部署：生成各服务的签名密钥与公钥表，并写出容器内使用的引用文件。
# 需要 Go；私钥只写入 deploy/secrets（已在 .gitignore 里），不要提交。
set -eu
cd "$(dirname "$0")/.."
# secretsDir 是相对仓库根的路径，所以要在仓库根运行工具（先编译出来）。
tool=$(mktemp)
(cd libs/platform && go build -o "$tool" ./cmd/nw-keys)
"$tool" --services deploy/config/services.json --out deploy/config/trust-anchors.json --budget-dir deploy/secrets/budget
rm -f "$tool"
budget=$(ls deploy/secrets/budget/budget-*.pem | head -n1)
cp "$budget" deploy/secrets/budget/budget.pem
printf '{"keyId":"%s","privateKeyRef":"/run/secrets/budget.pem"}\\n' "$(basename "$budget" .pem | sed 's/^budget-//')" > deploy/secrets/budget/budget-key.json
for dir in deploy/secrets/*/; do
	slug=$(basename "$dir")
	[ "$slug" = budget ] && continue
	pem=$(ls "$dir"signing-*.pem | head -n1)
	keyid=$(basename "$pem" .pem | sed 's/^signing-//')
	cp "$pem" "$dir/signing.pem"
	printf '{"keyId":"%s","privateKeyRef":"/run/secrets/signing.pem"}\\n' "$keyid" > "$dir/signing-key.json"
	if [ "$slug" = portal ]; then
		dpem=$(ls "$dir"delegation-*.pem | head -n1)
		did=$(basename "$dpem" .pem | sed 's/^delegation-//')
		cp "$dpem" "$dir/delegation.pem"
		printf '{"keyId":"%s","privateKeyRef":"/run/secrets/delegation.pem"}\\n' "$did" > "$dir/delegation-key.json"
	fi
done
for s in model-gateway data-protection; do
	[ -f deploy/secrets/$s/master-key ] || head -c 32 /dev/urandom > deploy/secrets/$s/master-key
done
# 容器里的服务用户（uid 与宿主机不同）要能读到挂载进去的密钥文件：文件放开读，靠顶层目录 0700 挡住宿主机上的其他用户。
find deploy/secrets -type d -exec chmod 755 {} +
find deploy/secrets -type f -exec chmod 644 {} +
chmod 600 deploy/secrets/model-gateway/master-key deploy/secrets/data-protection/master-key
chmod 700 deploy/secrets
echo "密钥已生成；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d --build"
""")
    os.chmod(os.path.join(ROOT, "deploy/init-secrets.sh"), 0o755)
    volumes = " ".join(f"nice-wiki_{s}-data" for s in SERVICES)
    write("deploy/backup.sh", f"""#!/bin/sh
# 过渡方案：整站停机，把各服务数据卷打成一个 tar.gz，再启动。data-protection 的在线一致备份完成后由它取代。
# 用法：deploy/backup.sh [输出目录，默认 ./backups]
set -eu
cd "$(dirname "$0")"
out="${{1:-./backups}}"
mkdir -p "$out"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker compose stop
trap 'docker compose start' EXIT
for volume in {volumes}; do
	docker run --rm -v "$volume":/data -v "$(cd "$out" && pwd)":/out alpine tar czf "/out/$volume-$stamp.tar.gz" -C /data .
done
echo "备份已写入 $out（时间戳 $stamp）；请同时妥善保管 deploy/secrets，没有密钥无法恢复。"
""")
    os.chmod(os.path.join(ROOT, "deploy/backup.sh"), 0o755)
    write("deploy/restore.sh", f"""#!/bin/sh
# 过渡方案：从 backup.sh 生成的备份恢复。会覆盖当前数据卷，请先确认。
# 用法：deploy/restore.sh <备份目录> <时间戳>
set -eu
cd "$(dirname "$0")"
dir="$1"; stamp="$2"
printf '将用备份 %s 覆盖当前所有数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
for volume in {volumes}; do
	docker run --rm -v "$volume":/data -v "$(cd "$dir" && pwd)":/in alpine sh -c "rm -rf /data/* /data/.[!.]* 2>/dev/null; tar xzf /in/$volume-$stamp.tar.gz -C /data"
done
docker compose start
""")
    os.chmod(os.path.join(ROOT, "deploy/restore.sh"), 0o755)
    write("deploy/restore-archive.sh", f"""#!/bin/sh
# 从后台“备份与恢复”页下载的 tar.gz 备份恢复（归档里每个顶层目录对应一个服务的数据卷）。会覆盖当前数据卷，请先确认。
# 恢复后目录统一收紧为仅属主可访问（服务要求数据目录私有）。
# 密钥不在默认备份里：需要原来的 deploy/secrets（同一套签名密钥与主密钥）。
# 用法：deploy/restore-archive.sh <备份 tar.gz 或 tar.gz.enc 的绝对路径>
set -eu
cd "$(dirname "$0")"
archive="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
# 带 .enc 后缀的是设了下载口令的加密备份：先用 openssl 解密到临时文件（会提示输入口令）。
case "$archive" in
	*.enc)
		plain="$(mktemp "$PWD/.restore-plain.XXXXXX")"   # 放在 deploy 目录下，容器才能挂载到；脚本结束会删除
		trap 'rm -f "$plain"' EXIT
		# 口令默认交互输入；脚本化时可设环境变量 NW_BACKUP_PASSPHRASE。
		if [ -n "${{NW_BACKUP_PASSPHRASE:-}}" ]; then pass_arg="-pass env:NW_BACKUP_PASSPHRASE"; else pass_arg=""; fi
		openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 $pass_arg -in "$archive" -out "$plain" || {{ echo "解密失败（口令不对或文件损坏），未做任何改动"; exit 1; }}
		archive="$plain"
		;;
esac
docker run --rm -v "$archive":/in/backup.tar.gz:ro alpine tar tzf /in/backup.tar.gz > /dev/null || {{ echo "备份文件不可读，未做任何改动"; exit 1; }}
printf '将用 %s 覆盖当前所有数据卷，输入 yes 继续：' "$archive"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
for slug in {" ".join(s for s in SERVICES if s != "data-protection")}; do
	docker run --rm -v "nice-wiki_$slug-data":/data -v "$archive":/in/backup.tar.gz:ro alpine sh -c "
		set -e
		tar tzf /in/backup.tar.gz | grep '^$slug/' > /tmp/list || exit 0
		owner=\\$(stat -c %u:%g /data)
		rm -rf /data/incoming-restore && mkdir /data/incoming-restore
		tar xzf /in/backup.tar.gz -C /data/incoming-restore -T /tmp/list --strip-components=1
		find /data -mindepth 1 -maxdepth 1 ! -name incoming-restore -exec rm -rf {{}} +
		find /data/incoming-restore -mindepth 1 -maxdepth 1 -exec mv {{}} /data/ \\\\;
		rmdir /data/incoming-restore
		chmod -R go-rwx /data && chown -R \\$owner /data"
done
# 备份若包含密钥（超管勾选过），解到 ./secrets-restored，请自行核对后替换 deploy/secrets。
if docker run --rm -v "$archive":/in/backup.tar.gz:ro alpine tar tzf /in/backup.tar.gz | grep -q '^secrets/'; then
	mkdir -p secrets-restored && tar xzf "$archive" -C secrets-restored --strip-components=1 secrets && echo "备份里的密钥已解到 deploy/secrets-restored，确认后用它替换 deploy/secrets"
fi
docker compose start
""")
    os.chmod(os.path.join(ROOT, "deploy/restore-archive.sh"), 0o755)


if __name__ == "__main__":
    main()
