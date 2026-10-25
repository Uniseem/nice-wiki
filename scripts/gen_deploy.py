#!/usr/bin/env python3
"""生成部署文件：各域 Dockerfile、deploy/docker-compose.yml、Caddyfile、services.json、各服务配置与 .env.example。
服务清单只在本文件的 SERVICES 里维护，改完后重新运行即可。"""
import json, os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SERVICES = {  # slug: 端口
    "portal": 8080, "public-site": 8081, "identity": 8101, "site-config": 8102, "content": 8103, "media": 8104,
    "model-gateway": 8105, "knowledge": 8106, "qa": 8107, "localization": 8108, "memory": 8109,
    "traffic-guard": 8110, "jobs": 8111, "analytics": 8112, "data-protection": 8113, "agent": 8114,
}
STATELESS = {"portal", "public-site", "agent"}  # 不存业务数据，在线备份里没有它们的目录，恢复时不动它们的卷
WEB_MODULES = ["portal", "content", "site-config", "identity", "media", "localization", "model-gateway", "knowledge", "memory", "qa", "traffic-guard", "jobs", "analytics", "data-protection", "public-site"]
SUBNET = "10.231.97.0/24"  # 内部桥接网络：用私有网段（服务器防火墙通常只给私有网段放行转发，文档保留段的容器出不了网），避开 Docker 默认地址池
IMAGES = "ghcr.io/uniseem/nice-wiki"  # 发版流程（scripts/release.sh）推送的镜像：{IMAGES}-{slug}:{版本}


def write(path, text):
    full = os.path.join(ROOT, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    open(full, "w").write(text)


def dockerfile(slug, port):
    uid = 10000 + list(SERVICES).index(slug)
    web = ""
    copy_web = ""
    if slug == "portal":
        # 前端是仓库根的一个 pnpm 工程：先装依赖（只依赖清单，便于缓存），再拷入各域界面源码与公开页模板（样式构建要扫描其中的类名）。
        sources = "\n".join(f"COPY modules/{m}/web/ modules/{m}/web/" for m in WEB_MODULES)
        web = f"""FROM node:24-slim AS web
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY libs/platform/zcode/upstream/ libs/platform/zcode/upstream/
RUN pnpm install --frozen-lockfile
COPY contracts/permission-groups.json contracts/permission-groups.json
COPY libs/platform/web/ libs/platform/web/
{sources}
COPY modules/public-site/internal/ modules/public-site/internal/
RUN NW_WEB_OUT=/out/web pnpm build:web

"""
        copy_web = "COPY --from=web /out/web /web\nCOPY modules/portal/config/routes.json /config/routes.json\nENV NW_WEB_DIR=/web\n"
    if slug == "agent":
        web = """FROM node:24-slim AS native
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY libs/platform/zcode/ libs/platform/zcode/
RUN pnpm install --frozen-lockfile
COPY scripts/build_zcode.mjs scripts/build_zcode.mjs
COPY THIRD_PARTY_NOTICES.md ./
COPY docs/licenses/ docs/licenses/
RUN pnpm build:agent

"""
        copy_web = "COPY --from=native /src/agent-dist/ /opt/agent/\nCOPY --from=native /src/docs/licenses/ /opt/agent/licenses/\nCOPY THIRD_PARTY_NOTICES.md /opt/agent/THIRD_PARTY_NOTICES.md\nENV NW_ZCODE_WORKER=/opt/agent/worker.mjs\n"
    return f"""# {slug} 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
{web}FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/{slug}/ modules/{slug}/
WORKDIR /src/modules/{slug}
RUN CGO_ENABLED=0 go build -trimpath -o /out/{slug} ./cmd/{slug}

{runtime_stage(slug, port, f"COPY --from=build /out/{slug} /usr/local/bin/{slug}", copy_web)}"""


def runtime_stage(slug, port, copy_binary, copy_web):
    uid = 10000 + list(SERVICES).index(slug)
    return f"""FROM {"node:24-alpine" if slug == "agent" else "alpine:3.20"}
RUN adduser -D -u {uid} {slug} && mkdir -p /data/{slug} && chown {uid}:{uid} /data/{slug}
{copy_binary}
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


def compose(release=False):
    # 源码仓库里的编排在本机构建镜像；发布包里的编排只拉取发版推送的镜像，服务器上不用编译。
    header = "# 单机部署编排（发布包）：拉取 ghcr.io 上的发布镜像。由 scripts/gen_deploy.py 生成，勿手改。" if release else "# 单机部署编排：由 scripts/gen_deploy.py 生成，勿手改。"
    lines = [header, "name: nice-wiki", "", "networks:", "  internal:", "    driver: bridge",
             "    ipam:", "      config:", f"        - subnet: {SUBNET}", "", "services:"]
    for slug, port in SERVICES.items():
        if release:
            lines += [f"  {slug}:", f"    image: ${{NW_IMAGES:-{IMAGES}}}-{slug}:${{NW_VERSION:?请在 deploy/.env 里设置 NW_VERSION}}"]
        else:
            lines += [f"  {slug}:", f"    build:", "      context: ..", f"      dockerfile: modules/{slug}/Dockerfile", f"    image: nice-wiki/{slug}:${{NW_VERSION:-dev}}"]
        lines += ["    restart: unless-stopped", "    networks: [internal]",
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
            # 简易备份只读取各服务数据目录：只读挂载，保护服务（以 root 运行）也改不了其他域的数据。
            # SQLite 只读打开时会退回进程内的 WAL 索引，仍能读到 WAL 里的最新提交（VACUUM INTO 取快照），写入只落在自己的 backups 卷。
            lines += ["      - backups:/backups", "      - ./secrets:/backup-secrets:ro"] + [f"      - {s}-data:/sources/{s}:ro" for s in SERVICES if s != slug]
        lines.append("")
    lines += ["  caddy:", "    image: caddy:2-alpine", "    restart: unless-stopped", "    networks: [internal]",
              "    ports: [\"${NW_BIND:-0.0.0.0}:80:80\", \"${NW_BIND:-0.0.0.0}:443:443\"]", "    environment:", "      SITE_DOMAIN: ${SITE_DOMAIN}",
              "    volumes:", "      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro", "      - ./caddy/conf.d:/etc/caddy/conf.d:ro", "      - ./secrets/tls:/run/tls:ro", "      - caddy-data:/data", "      - caddy-config:/config",
              "    depends_on: [portal]", "", "volumes:"]
    lines += [f"  {slug}-data:" for slug in SERVICES] + ["  backups:", "  caddy-data:", "  caddy-config:", ""]
    return "\n".join(lines)


RELEASE_README = """# nice-wiki 部署包

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
"""


def main():
    for slug, port in SERVICES.items():
        write(f"modules/{slug}/Dockerfile", dockerfile(slug, port))
        write(f"deploy/config/{slug}.json", json.dumps(service_config(slug, port), indent=2, ensure_ascii=False) + "\n")
    write("deploy/docker-compose.yml", compose())
    write("deploy/release/docker-compose.yml", compose(release=True))
    # 发布镜像的规格（scripts/release.sh images 读取）：与上面源码镜像的运行阶段一致——同一 uid、端口和工作目录。
    write("deploy/release/images.tsv", "".join(f"{slug}\t{10000 + i}\t{port}\t{'/' if slug == 'portal' else '/config'}\n" for i, (slug, port) in enumerate(SERVICES.items())))
    write("deploy/release/README.md", RELEASE_README)
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
web-dist
agent-dist
libs/platform/zcode/upstream/dynamic-workflow/src/compiler/libs.generated.ts
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
# 发布包自带 bin/nw-keys，不需要 Go；在源码仓库里运行时用 Go 现场编译。私钥只写入 deploy/secrets，不要提交。
set -eu
cd "$(dirname "$0")/.."
# secretsDir 是相对发布包根（或仓库根）的路径，所以要在这里运行工具。
if [ -x bin/nw-keys ]; then
	tool=bin/nw-keys
else
	tool=$(mktemp)
	(cd libs/platform && go build -o "$tool" ./cmd/nw-keys)
fi
"$tool" --services deploy/config/services.json --out deploy/config/trust-anchors.json --budget-dir deploy/secrets/budget
[ "$tool" = bin/nw-keys ] || rm -f "$tool"
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
echo "密钥已生成；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d（在源码仓库里加 --build 本机构建）"
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
    # 两个恢复脚本共用的换卷步骤：数据先解到卷内 .restore-incoming（同一文件系统，换上只是改名），
    # 全部解好才逐卷换上，原数据挪进 .restore-previous；任何一卷失败就把所有卷换回原数据。
    swap_steps = r"""# 换上：原数据挪进 .restore-previous（挪完留标记），再把暂存挪到卷根。
swap='set -e
mkdir /data/.restore-previous
find /data -mindepth 1 -maxdepth 1 ! -name .restore-incoming ! -name .restore-previous -exec sh -c "mv \"\$@\" /data/.restore-previous/" sh {} +
touch /data/.restore-previous/.restore-complete
find /data/.restore-incoming -mindepth 1 -maxdepth 1 -exec sh -c "mv \"\$@\" /data/" sh {} +
rmdir /data/.restore-incoming'
# 换回：有标记说明卷根已是新数据，先清掉；再把原数据挪回卷根。
back='set -e
if [ -d /data/.restore-previous ]; then
	if [ -f /data/.restore-previous/.restore-complete ]; then
		rm /data/.restore-previous/.restore-complete
		find /data -mindepth 1 -maxdepth 1 ! -name .restore-previous -exec rm -rf {} +
	fi
	find /data/.restore-previous -mindepth 1 -maxdepth 1 -exec sh -c "mv \"\$@\" /data/" sh {} +
	rmdir /data/.restore-previous
fi
rm -rf /data/.restore-incoming'
for volume in $volumes; do
	if ! docker run --rm -v "$volume":/data alpine sh -c "$swap"; then
		# 换回逐卷进行，某一卷出错也继续换回其余的卷，最后如实报告。
		failed=""
		for v in $volumes; do docker run --rm -v "$v":/data alpine sh -c "$back" || failed="$failed $v"; done
		if [ -n "$failed" ]; then echo "替换 $volume 失败；以下卷没能换回，原数据在卷内 .restore-previous，需手动挪回：$failed"; else echo "替换 $volume 失败，所有数据卷已换回原数据"; fi
		exit 1
	fi
done
for volume in $volumes; do docker run --rm -v "$volume":/data alpine rm -rf /data/.restore-previous || echo "$volume 的 .restore-previous 没能删除，可手动删除"; done
echo "恢复完成"
"""
    abort_staging = r"""for v in $volumes; do docker run --rm -v "$v":/data alpine rm -rf /data/.restore-incoming || echo "$v 的暂存目录 .restore-incoming 没能清掉，请手动删除"; done"""
    write("deploy/restore.sh", r"""#!/bin/sh
# 过渡方案：从 backup.sh 生成的备份恢复。会覆盖当前数据卷，请先确认。
# 先核对全部归档都在且可读，停机后逐卷解到暂存目录，全部解好才替换；失败时原数据不动或被换回。
# 用法：deploy/restore.sh <备份目录> <时间戳>
set -eu
cd "$(dirname "$0")"
dir="$(cd "$1" && pwd)"; stamp="$2"
volumes="@VOLUMES@"
for volume in $volumes; do
	[ -f "$dir/$volume-$stamp.tar.gz" ] || { echo "缺少 $volume-$stamp.tar.gz，未做任何改动"; exit 1; }
	docker run --rm -v "$dir":/in:ro alpine tar tzf "/in/$volume-$stamp.tar.gz" > /dev/null || { echo "$volume-$stamp.tar.gz 已损坏，未做任何改动"; exit 1; }
done
printf '将用备份 %s 覆盖当前所有数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
trap 'docker compose start' EXIT
for volume in $volumes; do
	if ! docker run --rm -v "$volume":/data -v "$dir":/in:ro alpine sh -c "set -e; rm -rf /data/.restore-incoming /data/.restore-previous; mkdir /data/.restore-incoming; tar xzf /in/$volume-$stamp.tar.gz -C /data/.restore-incoming"; then
		@ABORT@
		echo "解包 $volume 失败（空间不足或归档损坏），原数据未改动"
		exit 1
	fi
done
@SWAP@""".replace("@VOLUMES@", volumes).replace("@ABORT@", abort_staging).replace("@SWAP@", swap_steps))
    os.chmod(os.path.join(ROOT, "deploy/restore.sh"), 0o755)
    write("deploy/restore-archive.sh", r"""#!/bin/sh
# 从后台“备份与恢复”页下载的 tar.gz 备份恢复（归档里每个顶层目录对应一个服务的数据卷）。会覆盖当前数据卷，请先确认。
# 先核对归档可读且每个有状态服务都有数据（入口和公开站不存数据，不在归档里），停机后逐卷解到暂存目录，全部解好才替换；失败时原数据不动或被换回。
# 恢复后目录统一收紧为仅属主可访问（服务要求数据目录私有）。
# 密钥不在默认备份里：需要原来的 deploy/secrets（同一套签名密钥与主密钥）。
# 用法：deploy/restore-archive.sh <备份 tar.gz 或 tar.gz.enc 的绝对路径>
set -eu
cd "$(dirname "$0")"
archive="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
list="$(mktemp "$PWD/.restore-list.XXXXXX")"
plain=""
cleanup() { rm -f "$list"; [ -z "$plain" ] || rm -f "$plain"; }
trap cleanup EXIT
# 带 .enc 后缀的是设了下载口令的加密备份：先用 openssl 解密到临时文件（会提示输入口令）。
case "$archive" in
	*.enc)
		plain="$(mktemp "$PWD/.restore-plain.XXXXXX")"   # 放在 deploy 目录下，容器才能挂载到；脚本结束会删除
		# 口令默认交互输入；脚本化时可设环境变量 NW_BACKUP_PASSPHRASE。
		if [ -n "${NW_BACKUP_PASSPHRASE:-}" ]; then pass_arg="-pass env:NW_BACKUP_PASSPHRASE"; else pass_arg=""; fi
		openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 $pass_arg -in "$archive" -out "$plain" || { echo "解密失败（口令不对或文件损坏），未做任何改动"; exit 1; }
		archive="$plain"
		;;
esac
docker run --rm -v "$archive":/in/backup.tar.gz:ro alpine tar tzf /in/backup.tar.gz > "$list" || { echo "备份文件不可读，未做任何改动"; exit 1; }
volumes=""
for slug in @SLUGS@; do
	grep -q "^$slug/" "$list" || { echo "备份里缺少 $slug 的数据，未做任何改动"; exit 1; }
	volumes="$volumes nice-wiki_$slug-data"
done
printf '将用 %s 覆盖当前所有数据卷，输入 yes 继续：' "$archive"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
trap 'cleanup; docker compose start' EXIT
for volume in $volumes; do
	slug="${volume#nice-wiki_}"; slug="${slug%-data}"
	if ! docker run --rm -v "$volume":/data -v "$archive":/in/backup.tar.gz:ro alpine sh -c "
		set -e
		rm -rf /data/.restore-incoming /data/.restore-previous && mkdir /data/.restore-incoming
		tar tzf /in/backup.tar.gz | grep '^$slug/' > /tmp/list
		tar xzf /in/backup.tar.gz -C /data/.restore-incoming -T /tmp/list --strip-components=1
		chmod -R go-rwx /data/.restore-incoming && chown -R \$(stat -c %u:%g /data) /data/.restore-incoming"; then
		@ABORT@
		echo "解包 $slug 失败（空间不足或归档损坏），原数据未改动"
		exit 1
	fi
done
@SWAP@# 备份若包含密钥（超管勾选过），解到 ./secrets-restored，请自行核对后替换 deploy/secrets。
if grep -q '^secrets/' "$list"; then
	mkdir -p secrets-restored && tar xzf "$archive" -C secrets-restored --strip-components=1 secrets && echo "备份里的密钥已解到 deploy/secrets-restored，确认后用它替换 deploy/secrets"
fi
""".replace("@SLUGS@", " ".join(s for s in SERVICES if s not in STATELESS and s != "data-protection")).replace("@ABORT@", abort_staging).replace("@SWAP@", swap_steps))
    os.chmod(os.path.join(ROOT, "deploy/restore-archive.sh"), 0o755)


if __name__ == "__main__":
    main()
