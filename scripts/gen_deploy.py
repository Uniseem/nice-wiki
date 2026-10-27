#!/usr/bin/env python3
"""生成部署文件：各进程的 Dockerfile、deploy/docker-compose.yml、Caddyfile、各组件配置、密钥初始化与备份恢复脚本。
拓扑见 docs/architecture-v2.md：Caddy（按需证书）→ edge（Rust，缓存与限流）→ portal；account、content、ai、ops 四个宿主进程各跑一组组件，
knowledge（检索与向量）配 search（Rust）；全部数据在一个带 pgvector 的 PostgreSQL 里，正文和媒体文件在 content 的数据卷。
进程与组件清单只在本文件里维护，改完后重新运行即可。"""
import json, os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PORTS = {  # 组件: 端口
    "portal": 8080, "public-site": 8081, "identity": 8101, "site-config": 8102, "content": 8103, "media": 8104,
    "model-gateway": 8105, "knowledge": 8106, "qa": 8107, "localization": 8108, "memory": 8109,
    "traffic-guard": 8110, "jobs": 8111, "analytics": 8112, "agent": 8114,
}
# 宿主进程 → 组件（同一进程里运行多个组件，各自监听、各自配置）；portal、knowledge 是独立进程。
HOSTS = {"account": ["site-config", "identity"], "content": ["content", "media", "localization", "public-site"],
         "ai": ["model-gateway", "qa", "agent", "memory"], "ops": ["jobs", "analytics", "traffic-guard"]}
PROCESSES = list(HOSTS) + ["portal", "knowledge"]
PROCESS_OF = {c: p for p, comps in HOSTS.items() for c in comps}
PROCESS_OF.update({"portal": "portal", "knowledge": "knowledge"})
SEARCH_PORT, EDGE_PORT = 8120, 8090
WEB_MODULES = ["portal", "content", "site-config", "identity", "media", "localization", "model-gateway", "knowledge", "memory", "qa", "traffic-guard", "jobs", "analytics", "public-site"]
MASTER_KEY_COMPONENTS = ["identity", "model-gateway", "content"]  # 需要主密钥（加密保存第三方凭据、自带密钥、备份凭据）
SUBNET = "10.231.97.0/24"  # 内部桥接网络：用私有网段（服务器防火墙通常只给私有网段放行转发，文档保留段的容器出不了网），避开 Docker 默认地址池
IMAGES = "ghcr.io/uniseem/nice-wiki"  # 发版流程推送的镜像：{IMAGES}-{进程}:{版本}
UIDS = {name: 10000 + i for i, name in enumerate(PROCESSES + ["search", "edge"])}


def write(path, text, mode=None):
    full = os.path.join(ROOT, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    open(full, "w").write(text)
    if mode:
        os.chmod(full, mode)


def web_stage():
    sources = "\n".join(f"COPY modules/{m}/web/ modules/{m}/web/" for m in WEB_MODULES)
    return f"""FROM node:24-slim AS web
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY libs/platform/zcode/upstream/ libs/platform/zcode/upstream/
RUN pnpm install --frozen-lockfile
COPY contracts/permission-groups.json contracts/permission-groups.json
COPY docs/licenses/ docs/licenses/
COPY libs/platform/web/ libs/platform/web/
{sources}
COPY modules/public-site/internal/ modules/public-site/internal/
RUN NW_WEB_OUT=/out/web pnpm build:web

"""


def agent_stage():
    return """FROM node:24-slim AS native
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


def go_dockerfile(proc):
    """Go 进程镜像；构建上下文为仓库根。宿主进程在 services/<进程>，portal 与 knowledge 在 modules/<进程>。"""
    host = proc in HOSTS
    workdir = f"services/{proc}" if host else f"modules/{proc}"
    pre, copy_extra, base, env = "", "", "alpine:3.20", ""
    if proc == "portal":
        pre = web_stage()
        copy_extra = "COPY --from=web /out/web /web\nCOPY modules/portal/config/routes.json /config/routes.json\n"
        env = "ENV NW_WEB_DIR=/web\n"
    if proc == "ai":
        pre = agent_stage()
        copy_extra = "COPY --from=native /src/agent-dist/ /opt/agent/\nCOPY --from=native /src/docs/licenses/ /opt/agent/licenses/\nCOPY THIRD_PARTY_NOTICES.md /opt/agent/THIRD_PARTY_NOTICES.md\n"
        env = "ENV NW_ZCODE_WORKER=/opt/agent/worker.mjs\n"
        base = "node:24-alpine"
    uid = UIDS[proc]
    cmd = '["--config-dir", "/config"]' if host else f'["--config", "/config/{proc}.json"]'
    wd = "/config" if proc != "portal" else "/"
    return f"""# {proc} 进程镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
{pre}FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/ modules/
COPY services/ services/
WORKDIR /src/{workdir}
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/{proc} ./cmd/{proc}

FROM {base}
RUN adduser -D -u {uid} {proc} && mkdir -p /data && chown {uid}:{uid} /data
COPY --from=build /out/{proc} /usr/local/bin/{proc}
{copy_extra}{env}USER {uid}:{uid}
WORKDIR {wd}
ENTRYPOINT ["/usr/local/bin/{proc}"]
CMD {cmd}
"""


RUST_DOCKERFILE = f"""# Rust 进程镜像（检索 search、公开入口 edge）；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM rust:1.95-bookworm AS build
WORKDIR /src
COPY rust/ rust/
WORKDIR /src/rust
RUN cargo build --release -p nw-search -p nw-edge

FROM debian:bookworm-slim AS search
RUN useradd -u {UIDS['search']} -m search && mkdir -p /data && chown {UIDS['search']}:{UIDS['search']} /data
COPY --from=build /src/rust/target/release/nw-search /usr/local/bin/nw-search
USER {UIDS['search']}:{UIDS['search']}
ENTRYPOINT ["/usr/local/bin/nw-search"]
CMD ["/config/search.json"]

FROM debian:bookworm-slim AS edge
RUN useradd -u {UIDS['edge']} -m edge
COPY --from=build /src/rust/target/release/nw-edge /usr/local/bin/nw-edge
USER {UIDS['edge']}:{UIDS['edge']}
ENTRYPOINT ["/usr/local/bin/nw-edge"]
CMD ["/config/edge.json"]
"""


def secret_dir(slug):
    return f"/run/secrets/{slug}"


def component_config(slug):
    port = PORTS[slug]
    cfg = {
        "module": slug, "listen": f"0.0.0.0:{port}", "dataRoot": f"/data/{slug}",
        "trustAnchorRef": "/run/secrets/trust-anchors.json", "signingKeyRef": f"{secret_dir(slug)}/signing-key.json",
        "dependencies": {s: f"http://{PROCESS_OF[s]}:{p}" for s, p in PORTS.items() if s != slug},
        "databaseRef": "/run/secrets/database-url",
    }
    if slug in ("portal", "traffic-guard"):
        cfg["budgetKeyRef"] = "/run/secrets/budget/budget-key.json"
    if slug == "portal":
        cfg["delegationKeyRef"] = f"{secret_dir(slug)}/delegation-key.json"
        cfg["trustedProxyCIDRs"] = [SUBNET]
    if slug == "analytics":
        cfg["reportingTimeZone"] = "Asia/Shanghai"
    if slug in MASTER_KEY_COMPONENTS:
        cfg["masterKeyRef"] = "/run/secrets/master-key"
    if slug == "knowledge":
        cfg["dependencies"]["search"] = f"http://search:{SEARCH_PORT}"
        cfg["searchSecretRef"] = "/run/secrets/search-secret"
    return cfg


def compose(release=False):
    header = "# 单机部署编排（发布包）：拉取 ghcr.io 上的发布镜像。由 scripts/gen_deploy.py 生成，勿手改。" if release else "# 单机部署编排：由 scripts/gen_deploy.py 生成，勿手改。"
    lines = [header, "name: nice-wiki", "", "networks:", "  internal:", "    driver: bridge", "    ipam:", "      config:", f"        - subnet: {SUBNET}", "", "services:"]

    def image(name, dockerfile, target=None):
        if release:
            return [f"    image: ${{NW_IMAGES:-{IMAGES}}}-{name}:${{NW_VERSION:?请在 deploy/.env 里设置 NW_VERSION}}"]
        out = ["    build:", "      context: ..", f"      dockerfile: {dockerfile}"]
        if target:
            out.append(f"      target: {target}")
        return out + [f"    image: nice-wiki/{name}:${{NW_VERSION:-dev}}"]

    lines += ["  postgres:", "    image: pgvector/pgvector:pg17", "    restart: unless-stopped", "    networks: [internal]",
              "    environment:", "      POSTGRES_USER: postgres", "      POSTGRES_DB: nicewiki", "      POSTGRES_PASSWORD_FILE: /run/secrets/pg-superuser-password",
              "    volumes:", "      - postgres-data:/var/lib/postgresql/data", "      - ./postgres/init:/docker-entrypoint-initdb.d:ro",
              "      - ./secrets/postgres/superuser-password:/run/secrets/pg-superuser-password:ro", "      - ./secrets/postgres/app-password:/run/secrets/pg-app-password:ro",
              "    healthcheck:", "      test: [\"CMD-SHELL\", \"pg_isready -U postgres -d nicewiki\"]", "      interval: 5s", "      timeout: 3s", "      retries: 20", ""]

    def app(proc, comps, extra_volumes=(), extra_env=None, user=None, depends=("postgres",)):
        block = [f"  {proc}:"] + image(proc, f"{'services' if proc in HOSTS else 'modules'}/{proc}/Dockerfile") + ["    restart: unless-stopped", "    networks: [internal]"]
        if user:
            block.append(f'    user: "{user}"')
        env = {"GOMEMLIMIT": "${GOMEMLIMIT:-256MiB}", **(extra_env or {})}
        block += ["    environment:"] + [f"      {k}: {v}" for k, v in env.items()]
        vols = [f"./config/{c}.json:/config/{c}.json:ro" for c in comps] + [f"./secrets/{c}:/run/secrets/{c}:ro" for c in comps]
        vols += ["./config/trust-anchors.json:/run/secrets/trust-anchors.json:ro", "./secrets/database-url:/run/secrets/database-url:ro", f"{proc}-data:/data"]
        if any(c in MASTER_KEY_COMPONENTS for c in comps):
            vols.append("./secrets/master-key:/run/secrets/master-key:ro")
        if any(c in ("portal", "traffic-guard") for c in comps):
            vols.append("./secrets/budget:/run/secrets/budget:ro")
        vols += list(extra_volumes)
        block += ["    volumes:"] + [f"      - {v}" for v in vols]
        block += ["    depends_on:"] + [f"      {d}: {{condition: service_healthy}}" if d == "postgres" else f"      {d}: {{condition: service_started}}" for d in depends]
        return block + [""]

    # 主密钥文件权限 0600，宿主进程里有用到它的服务以 root 运行才读得到。
    public = {"NW_ALLOW_LOCAL_HTTP": '"false"', "NW_PUBLIC_BASE_URL": "https://${SITE_DOMAIN}"}
    lines += app("account", HOSTS["account"], extra_env=public, user="0:0")
    lines += app("content", HOSTS["content"], extra_env=public, user="0:0")
    lines += app("ai", HOSTS["ai"], user="0:0")
    lines += app("ops", HOSTS["ops"], user="0:0")
    lines += app("knowledge", ["knowledge"], extra_volumes=["./secrets/search/secret:/run/secrets/search-secret:ro"])
    lines += app("portal", ["portal"], extra_env={**public, "NW_EDGE_PURGE_URL": f"http://edge:{EDGE_PORT}", "NW_EDGE_PURGE_SECRET_FILE": "/run/secrets/edge-purge-secret"},
                 extra_volumes=["./secrets/edge/purge-secret:/run/secrets/edge-purge-secret:ro"], depends=("postgres", "account", "content"))
    lines += ["  search:"] + image("search", "rust/Dockerfile", "search") + ["    restart: unless-stopped", "    networks: [internal]", "    volumes:", "      - ./config/search.json:/config/search.json:ro",
              "      - ./secrets/search/secret:/run/secrets/search-secret:ro", "      - ./secrets/database-url:/run/secrets/database-url:ro", "      - search-data:/data", "    depends_on:", "      postgres: {condition: service_healthy}", ""]
    lines += ["  edge:"] + image("edge", "rust/Dockerfile", "edge") + ["    restart: unless-stopped", "    networks: [internal]", "    volumes:", "      - ./config/edge.json:/config/edge.json:ro", "      - ./secrets/edge/purge-secret:/run/secrets/purge-secret:ro", "    depends_on: [portal]", ""]
    lines += ["  caddy:", "    image: caddy:2-alpine", "    restart: unless-stopped", "    networks: [internal]",
              "    ports: [\"${NW_BIND:-0.0.0.0}:80:80\", \"${NW_BIND:-0.0.0.0}:443:443\"]", "    environment:", "      SITE_DOMAIN: ${SITE_DOMAIN}",
              "    volumes:", "      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro", "      - ./caddy/conf.d:/etc/caddy/conf.d:ro", "      - caddy-data:/data", "      - caddy-config:/config",
              "    depends_on: [edge]", "", "volumes:"]
    lines += [f"  {v}:" for v in ["postgres-data", "account-data", "content-data", "ai-data", "ops-data", "knowledge-data", "portal-data", "search-data", "caddy-data", "caddy-config"]] + [""]
    return "\n".join(lines)


CADDYFILE = """# 公开入口：Caddy 终止 HTTPS，证书按需签发——只给平台域名、已验证的自定义域名和平台子域名签（由 portal 的 /_tls/allow 判断），
# 再把流量交给 edge（页面缓存与限流），edge 转给 portal。
# 部署相关的定制（例如放在 Cloudflare 后面：只放行 Cloudflare 回源、取真实访客 IP）写成 conf.d 下的 global-*.caddy / site-*.caddy，本地文件不进 git。
{
	on_demand_tls {
		ask http://portal:8080/_tls/allow
	}
	import /etc/caddy/conf.d/global-*.caddy
}
http:// {
	redir https://{host}{uri} permanent
}
https:// {
	tls {
		on_demand
	}
	import /etc/caddy/conf.d/site-*.caddy
	encode zstd gzip
	header {
		Strict-Transport-Security "max-age=31536000"
		X-Content-Type-Options nosniff
		Referrer-Policy strict-origin-when-cross-origin
	}
	reverse_proxy edge:8090 {
		header_up X-Forwarded-For {client_ip}
	}
}
"""

POSTGRES_INIT = """#!/bin/sh
# 首次启动数据库时运行：建应用角色（非超级用户、不绕过行级安全）、授权、装 pgvector。
set -eu
psql -v ON_ERROR_STOP=1 -U postgres -d nicewiki <<SQL
CREATE ROLE nw_app LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD '$(cat /run/secrets/pg-app-password)';
GRANT CONNECT, CREATE ON DATABASE nicewiki TO nw_app;
ALTER SCHEMA public OWNER TO nw_app;
CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;
SQL
"""

INIT_SECRETS = """#!/bin/sh
# 首次部署：生成各组件的签名密钥与公钥表、主密钥、数据库与检索口令，并写出容器内使用的引用文件。
# 发布包自带 bin/nw-keys，不需要 Go；在源码仓库里运行时用 Go 现场编译。私钥只写入 deploy/secrets，不要提交。
set -eu
cd "$(dirname "$0")/.."
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
printf '{"keyId":"%s","privateKeyRef":"/run/secrets/budget/budget.pem"}\\n' "$(basename "$budget" .pem | sed 's/^budget-//')" > deploy/secrets/budget/budget-key.json
for dir in deploy/secrets/*/; do
	slug=$(basename "$dir")
	case "$slug" in budget|postgres|search|edge) continue ;; esac
	pem=$(ls "$dir"signing-*.pem | head -n1)
	keyid=$(basename "$pem" .pem | sed 's/^signing-//')
	cp "$pem" "$dir/signing.pem"
	printf '{"keyId":"%s","privateKeyRef":"/run/secrets/%s/signing.pem"}\\n' "$keyid" "$slug" > "$dir/signing-key.json"
	if [ "$slug" = portal ]; then
		dpem=$(ls "$dir"delegation-*.pem | head -n1)
		did=$(basename "$dpem" .pem | sed 's/^delegation-//')
		cp "$dpem" "$dir/delegation.pem"
		printf '{"keyId":"%s","privateKeyRef":"/run/secrets/portal/delegation.pem"}\\n' "$did" > "$dir/delegation-key.json"
	fi
done
mkdir -p deploy/secrets/postgres deploy/secrets/search deploy/secrets/edge
rand() { head -c 24 /dev/urandom | base64 | tr -d '=+/\\n'; }
[ -f deploy/secrets/master-key ] || head -c 32 /dev/urandom > deploy/secrets/master-key
[ -f deploy/secrets/postgres/superuser-password ] || rand > deploy/secrets/postgres/superuser-password
[ -f deploy/secrets/postgres/app-password ] || rand > deploy/secrets/postgres/app-password
[ -f deploy/secrets/search/secret ] || head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \\n' > deploy/secrets/search/secret
[ -f deploy/secrets/edge/purge-secret ] || head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \\n' > deploy/secrets/edge/purge-secret
printf 'postgres://nw_app:%s@postgres:5432/nicewiki?sslmode=disable' "$(cat deploy/secrets/postgres/app-password)" > deploy/secrets/database-url
# 容器里的服务用户（uid 与宿主机不同）要能读到挂载进去的密钥文件：文件放开读，靠顶层目录 0700 挡住宿主机上的其他用户；主密钥与数据库口令只给 root。
find deploy/secrets -type d -exec chmod 755 {} +
find deploy/secrets -type f -exec chmod 644 {} +
chmod 600 deploy/secrets/master-key deploy/secrets/postgres/superuser-password
chmod 700 deploy/secrets
echo "密钥已生成（务必另行备份 deploy/secrets，丢了无法恢复数据）；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d（在源码仓库里加 --build 本机构建）"
"""

BACKUP_SH = """#!/bin/sh
# 整站备份：数据库用 pg_dump 在线导出（一致快照）；正文修订与媒体文件只增不改，增量复制到输出目录里的
# content-data/（第二次起只复制新增的）；其余小卷整卷打包；再打包密钥。数据库先导出、文件后复制，
# 所以文件一定包含数据库里引用的全部文件。
# 用法：deploy/backup.sh [输出目录，默认 ./backups]
# 可选环境变量：
#   BACKUP_PASSPHRASE_FILE  口令文件。设置后数据库导出和密钥包用它加密（openssl aes-256-cbc + pbkdf2，文件名多 .enc）；
#                           恢复时要给同一个口令文件。口令丢了备份就解不开，请和密钥分开保管。
#   BACKUP_REMOTE           rclone 目标（如 s3:桶/nicewiki、dav:nicewiki、gdrive:nicewiki，需先 rclone config）。
#                           设置后备份完成会把输出目录同步过去（rclone copy，已有的文件不重传）；要加密正文和媒体，
#                           把它配成 rclone 的 crypt 远端。
# 密钥含主密钥：没有加密时，备份目录请放在受控的存储里。
set -eu
cd "$(dirname "$0")"
project="${COMPOSE_PROJECT_NAME:-nice-wiki}"
out="${1:-./backups}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
seal() {
	[ -n "${BACKUP_PASSPHRASE_FILE:-}" ] || return 0
	openssl enc -aes-256-cbc -pbkdf2 -salt -pass "file:$BACKUP_PASSPHRASE_FILE" -in "$1" -out "$1.enc"
	rm "$1"
}
docker compose exec -T postgres pg_dump -U postgres -Fc nicewiki > "$out/db-$stamp.dump"
seal "$out/db-$stamp.dump"
mkdir -p "$out/content-data"
docker run --rm -v "${project}_content-data":/data:ro -v "$out/content-data":/out alpine sh -c "cp -au /data/. /out/ && chown -R $(id -u):$(id -g) /out"
for volume in @VOLUMES@; do
	docker run --rm -v "${project}_$volume":/data:ro -v "$out":/out alpine tar czf "/out/$volume-$stamp.tar.gz" -C /data .
done
tar czf "$out/secrets-$stamp.tar.gz" -C . secrets
seal "$out/secrets-$stamp.tar.gz"
if [ -n "${BACKUP_REMOTE:-}" ]; then
	rclone copy "$out" "$BACKUP_REMOTE"
fi
echo "备份已写入 $out（时间戳 $stamp）"
"""

RESTORE_SH = """#!/bin/sh
# 从 backup.sh 的备份恢复：停应用，恢复数据库与文件卷，再启动。会覆盖当前数据，请先确认。
# 用法：deploy/restore.sh <备份目录> <时间戳>
# 备份加密过时，用环境变量 BACKUP_PASSPHRASE_FILE 给出同一个口令文件。
# 密钥要用同一套：secrets-<时间戳>.tar.gz（加密的是 .enc）解到 deploy/secrets，本脚本不动密钥。
set -eu
cd "$(dirname "$0")"
project="${COMPOSE_PROJECT_NAME:-nice-wiki}"
dir="$(cd "$1" && pwd)"; stamp="$2"
# 输出备份文件的内容：有加密版就解密，没有就原样输出。
unseal() {
	if [ -f "$1.enc" ]; then
		[ -n "${BACKUP_PASSPHRASE_FILE:-}" ] || { echo "备份是加密的，请设置 BACKUP_PASSPHRASE_FILE" >&2; exit 1; }
		openssl enc -d -aes-256-cbc -pbkdf2 -pass "file:$BACKUP_PASSPHRASE_FILE" -in "$1.enc"
	else
		cat "$1"
	fi
}
[ -f "$dir/db-$stamp.dump" ] || [ -f "$dir/db-$stamp.dump.enc" ] || { echo "缺少 db-$stamp.dump，未做任何改动"; exit 1; }
[ -d "$dir/content-data" ] || { echo "缺少 content-data 目录，未做任何改动"; exit 1; }
unseal "$dir/db-$stamp.dump" | head -c 5 | grep -q PGDMP || { echo "数据库导出读不出来（口令不对或文件损坏），未做任何改动"; exit 1; }
for volume in @VOLUMES@; do
	[ -f "$dir/$volume-$stamp.tar.gz" ] || { echo "缺少 $volume-$stamp.tar.gz，未做任何改动"; exit 1; }
done
printf '将用备份 %s 覆盖当前数据库和全部数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop caddy edge portal account content ai ops knowledge search
docker compose up -d postgres
docker compose exec -T postgres sh -c 'until pg_isready -U postgres -d nicewiki; do sleep 1; done'
docker compose exec -T postgres psql -U postgres -d nicewiki -c "DROP SCHEMA IF EXISTS public CASCADE; DO \\$\\$ DECLARE r record; BEGIN FOR r IN SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg\\\\_%' AND nspname <> 'information_schema' LOOP EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.nspname) || ' CASCADE'; END LOOP; END \\$\\$;"
unseal "$dir/db-$stamp.dump" | docker compose exec -T postgres pg_restore -U postgres -d nicewiki --no-owner --role=nw_app
docker run --rm -v "${project}_content-data":/data -v "$dir/content-data":/in:ro alpine sh -c "find /data -mindepth 1 -delete && cp -a /in/. /data/"
for volume in @VOLUMES@; do
	docker run --rm -v "${project}_$volume":/data -v "$dir":/in:ro alpine sh -c "find /data -mindepth 1 -delete && tar xzf /in/$volume-$stamp.tar.gz -C /data"
done
docker compose up -d
echo "恢复完成"
"""

ENV_EXAMPLE = """# 部署变量模板；不含任何密钥。复制为 .env 后修改。
# 首次用这个域名访问后台，之后在“平台管理”里填写平台域名和用户站点的子域名后缀（这里只用来签第一张证书）。
SITE_DOMAIN=docs.example.com
NW_VERSION=dev
GOMEMLIMIT=256MiB
# 对外监听地址：默认全部网卡；尚未开放公网、只经 SSH 隧道访问时改成 127.0.0.1。
# Docker 发布的端口不经过宿主机 INPUT 防火墙规则，要限制访问必须在这里绑定。
NW_BIND=0.0.0.0
"""

RELEASE_README = """# nice-wiki 部署包

单机部署：公开入口（Caddy 按需证书 → edge → portal）、account、content、ai、ops、knowledge、search 与带 pgvector 的 PostgreSQL，全部以 Docker 镜像运行。
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

## 升级

数据在 Docker 卷里，不随目录变化。下载新版本的部署包并解压，把旧目录里的 `deploy/secrets`、`deploy/.env`、`deploy/caddy/conf.d` 拷到新目录，
把 `deploy/.env` 里的 `NW_VERSION` 改成新版本，再在新目录执行 `docker compose -f deploy/docker-compose.yml up -d`。数据库结构随服务启动自动迁移。

## 备份

`deploy/backup.sh` 在线导出数据库、增量复制正文与媒体、打包其余文件卷与密钥，可选用口令加密（`BACKUP_PASSPHRASE_FILE`）并用 rclone 同步到异地（`BACKUP_REMOTE`，支持 S3、WebDAV、各家网盘）；`deploy/restore.sh` 恢复（先停机）。用户各自站点的内容备份在后台“数据与同步”里配置。
"""


def main():
    for proc in PROCESSES:
        write(f"{'services' if proc in HOSTS else 'modules'}/{proc}/Dockerfile", go_dockerfile(proc))
    write("rust/Dockerfile", RUST_DOCKERFILE)
    for slug in PORTS:
        write(f"deploy/config/{slug}.json", json.dumps(component_config(slug), indent=2, ensure_ascii=False) + "\n")
    write("deploy/config/search.json", json.dumps({"listen": f"0.0.0.0:{SEARCH_PORT}", "dataDir": "/data/index", "secretFile": "/run/secrets/search-secret", "databaseUrlFile": "/run/secrets/database-url"}, indent=2) + "\n")
    write("deploy/config/edge.json", json.dumps({"listen": f"0.0.0.0:{EDGE_PORT}", "upstream": "portal:8080", "trustForwarded": True, "purgeSecretFile": "/run/secrets/purge-secret"}, indent=2) + "\n")
    write("deploy/config/services.json", json.dumps({"version": 1, "services": [
        {"slug": s, "url": f"http://{PROCESS_OF[s]}:{p}", "port": p, "dataVolume": f"/data/{s}", "secretsDir": f"deploy/secrets/{s}"} for s, p in PORTS.items()]}, indent=2, ensure_ascii=False) + "\n")
    write("deploy/docker-compose.yml", compose())
    write("deploy/release/docker-compose.yml", compose(release=True))
    rows = [f"{p}\t{UIDS[p]}\t{PORTS[p] if p in PORTS else PORTS[HOSTS[p][0]]}\t{'/' if p == 'portal' else '/config'}\t{p}\t" + ("--config-dir,/config" if p in HOSTS else f"--config,/config/{p}.json") for p in PROCESSES]
    rows += [f"search\t{UIDS['search']}\t{SEARCH_PORT}\t/config\tnw-search\t/config/search.json", f"edge\t{UIDS['edge']}\t{EDGE_PORT}\t/config\tnw-edge\t/config/edge.json"]
    write("deploy/release/images.tsv", "\n".join(rows) + "\n")
    write("deploy/release/README.md", RELEASE_README)
    write("deploy/caddy/Caddyfile", CADDYFILE)
    write("deploy/postgres/init/01-app-role.sh", POSTGRES_INIT, 0o755)
    write("deploy/.env.example", ENV_EXAMPLE)
    write("deploy/init-secrets.sh", INIT_SECRETS, 0o755)
    volumes = " ".join(["account-data", "ai-data", "ops-data", "knowledge-data", "portal-data", "search-data"])  # content-data 单独增量复制
    write("deploy/backup.sh", BACKUP_SH.replace("@VOLUMES@", volumes), 0o755)
    write("deploy/restore.sh", RESTORE_SH.replace("@VOLUMES@", volumes), 0o755)
    write(".dockerignore", """# 构建上下文是仓库根：不带上本机的依赖目录、构建产物、开发数据和密钥。
**/node_modules
**/dist
**/target
web-dist
agent-dist
libs/platform/zcode/upstream/dynamic-workflow/src/compiler/libs.generated.ts
.git
.local
deploy/secrets
deploy/.env
vps.txt
""")


if __name__ == "__main__":
    main()
