# 入口 Caddy：带 Cloudflare DNS 模块，用来给平台子域名签泛域名证书（令牌由 account 经管理接口下发）。由 scripts/gen_deploy.py 生成。
FROM caddy:2-builder AS build
RUN xcaddy build --with github.com/caddy-dns/cloudflare

FROM caddy:2-alpine
COPY --from=build /usr/bin/caddy /usr/bin/caddy
