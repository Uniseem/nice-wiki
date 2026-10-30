# Rust 进程镜像（检索 search、公开入口 edge）；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM node:24-slim AS web
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY libs/platform/zcode/upstream/ libs/platform/zcode/upstream/
RUN pnpm install --frozen-lockfile
COPY contracts/permission-groups.json contracts/permission-groups.json
COPY docs/licenses/ docs/licenses/
COPY libs/platform/web/ libs/platform/web/
COPY modules/portal/web/ modules/portal/web/
COPY modules/content/web/ modules/content/web/
COPY modules/site-config/web/ modules/site-config/web/
COPY modules/identity/web/ modules/identity/web/
COPY modules/media/web/ modules/media/web/
COPY modules/localization/web/ modules/localization/web/
COPY modules/model-gateway/web/ modules/model-gateway/web/
COPY modules/knowledge/web/ modules/knowledge/web/
COPY modules/memory/web/ modules/memory/web/
COPY modules/qa/web/ modules/qa/web/
COPY modules/traffic-guard/web/ modules/traffic-guard/web/
COPY modules/jobs/web/ modules/jobs/web/
COPY modules/analytics/web/ modules/analytics/web/
COPY modules/public-site/web/ modules/public-site/web/
COPY modules/public-site/internal/ modules/public-site/internal/
RUN NW_WEB_OUT=/out/web pnpm build:web

FROM rust:1.95-bookworm AS build
WORKDIR /src
COPY rust/ rust/
WORKDIR /src/rust
RUN cargo build --release -p nw-search -p nw-edge

FROM debian:bookworm-slim AS search
RUN useradd -u 10004 -m search && mkdir -p /data && chown 10004:10004 /data
COPY --from=build /src/rust/target/release/nw-search /usr/local/bin/nw-search
USER 10004:10004
ENTRYPOINT ["/usr/local/bin/nw-search"]
CMD ["/config/search.json"]

FROM debian:bookworm-slim AS edge
RUN useradd -u 10005 -m edge && mkdir -p /cache && chown 10005:10005 /cache
COPY --from=build /src/rust/target/release/nw-edge /usr/local/bin/nw-edge
COPY --from=web /out/web /web
COPY modules/portal/config/routes.json /config/routes.json
USER 10005:10005
ENTRYPOINT ["/usr/local/bin/nw-edge"]
CMD ["/config/edge.json"]
