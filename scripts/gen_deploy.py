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
WEB_MODULES = ["content", "site-config", "identity", "media", "localization", "traffic-guard", "jobs", "analytics", "data-protection"]
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
        web = f"""FROM node:24-alpine AS web
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
RUN adduser -D -u {uid} {slug}
COPY --from=build /out/{slug} /{slug}
{copy_web}USER {uid}:{uid}
EXPOSE {port}
VOLUME ["/data/{slug}"]
WORKDIR /config
ENTRYPOINT ["/{slug}"]
CMD ["--config", "/config/{slug}.json"]
"""


def service_config(slug, port):
    cfg = {
        "module": slug, "listen": f"0.0.0.0:{port}", "dataRoot": f"/data/{slug}",
        "trustAnchorRef": "/run/secrets/trust-anchors.json", "signingKeyRef": "/run/secrets/signing-key.json",
        "dependencies": {s: f"http://{s}:{p}" for s, p in SERVICES.items() if s != slug},
    }
    if slug == "portal":
        cfg["delegationKeyRef"] = "/run/secrets/delegation-key.json"
        cfg["trustedProxyCIDRs"] = [SUBNET]
    if slug == "analytics":
        cfg["reportingTimeZone"] = "Asia/Shanghai"
    if slug == "data-protection":
        cfg["masterKeyRef"] = "/run/secrets/master-key"
    return cfg


def compose():
    lines = ["# 单机部署编排：由 scripts/gen_deploy.py 生成，勿手改。", "name: nice-wiki", "", "networks:", "  internal:", "    driver: bridge",
             "    ipam:", "      config:", f"        - subnet: {SUBNET}", "", "services:"]
    for slug, port in SERVICES.items():
        lines += [f"  {slug}:", f"    build:", "      context: ..", f"      dockerfile: modules/{slug}/Dockerfile",
                  f"    image: nice-wiki/{slug}:${{NW_VERSION:-dev}}", "    restart: unless-stopped", "    networks: [internal]",
                  "    environment:", "      GOMEMLIMIT: ${GOMEMLIMIT:-256MiB}"]
        if slug in ("portal", "identity", "site-config"):
            lines += ["      NW_ALLOW_LOCAL_HTTP: \"false\"", "      NW_PUBLIC_BASE_URL: https://${SITE_DOMAIN}" if slug != "site-config" else "      NW_PUBLIC_BASE_URL: https://${SITE_DOMAIN}"]
        lines += ["    volumes:", f"      - ./config/{slug}.json:/config/{slug}.json:ro",
                  f"      - ./secrets/{slug}/signing-key.json:/run/secrets/signing-key.json:ro",
                  f"      - ./secrets/{slug}/signing.pem:/run/secrets/signing.pem:ro",
                  "      - ./config/trust-anchors.json:/run/secrets/trust-anchors.json:ro",
                  f"      - {slug}-data:/data/{slug}"]
        if slug == "portal":
            lines += ["      - ./secrets/portal/delegation-key.json:/run/secrets/delegation-key.json:ro", "      - ./secrets/portal/delegation.pem:/run/secrets/delegation.pem:ro"]
        if slug == "data-protection":
            lines += ["      - ./secrets/data-protection/master-key:/run/secrets/master-key:ro", "      - backups:/backups"]
        lines.append("")
    lines += ["  caddy:", "    image: caddy:2-alpine", "    restart: unless-stopped", "    networks: [internal]",
              "    ports: [\"80:80\", \"443:443\"]", "    environment:", "      SITE_DOMAIN: ${SITE_DOMAIN}",
              "    volumes:", "      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro", "      - caddy-data:/data", "      - caddy-config:/config",
              "    depends_on: [portal]", "", "volumes:"]
    lines += [f"  {slug}-data:" for slug in SERVICES] + ["  backups:", "  caddy-data:", "  caddy-config:", ""]
    return "\n".join(lines)


def main():
    for slug, port in SERVICES.items():
        write(f"modules/{slug}/Dockerfile", dockerfile(slug, port))
        write(f"deploy/config/{slug}.json", json.dumps(service_config(slug, port), indent=2, ensure_ascii=False) + "\n")
    write("deploy/docker-compose.yml", compose())
    write("deploy/caddy/Caddyfile", """# 只有 portal 对外：访客页面、后台和接口都经它代理到各服务。
{$SITE_DOMAIN} {
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
    write("deploy/.env.example", """# 部署变量模板；不含任何密钥。复制为 .env 后修改。
SITE_DOMAIN=docs.example.com
NW_VERSION=dev
GOMEMLIMIT=256MiB
""")
    write("deploy/init-secrets.sh", """#!/bin/sh
# 首次部署：生成各服务的签名密钥与公钥表，并写出容器内使用的引用文件。
# 需要 Go；私钥只写入 deploy/secrets（已在 .gitignore 里），不要提交。
set -eu
cd "$(dirname "$0")/.."
(cd libs/platform && go run ./cmd/nw-keys --services ../../deploy/config/services.json --out ../../deploy/config/trust-anchors.json)
for dir in deploy/secrets/*/; do
	slug=$(basename "$dir")
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
[ -f deploy/secrets/data-protection/master-key ] || head -c 32 /dev/urandom > deploy/secrets/data-protection/master-key
chmod -R go-rwx deploy/secrets
echo "密钥已生成；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d --build"
""")
    os.chmod(os.path.join(ROOT, "deploy/init-secrets.sh"), 0o755)


if __name__ == "__main__":
    main()
