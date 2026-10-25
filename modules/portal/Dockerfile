# portal 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM node:24-slim AS web
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY libs/platform/zcode/upstream/ libs/platform/zcode/upstream/
RUN pnpm install --frozen-lockfile
COPY contracts/permission-groups.json contracts/permission-groups.json
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
COPY modules/data-protection/web/ modules/data-protection/web/
COPY modules/public-site/web/ modules/public-site/web/
COPY modules/public-site/internal/ modules/public-site/internal/
RUN NW_WEB_OUT=/out/web pnpm build:web

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/portal/ modules/portal/
WORKDIR /src/modules/portal
RUN CGO_ENABLED=0 go build -trimpath -o /out/portal ./cmd/portal

FROM alpine:3.20
RUN adduser -D -u 10000 portal && mkdir -p /data/portal && chown 10000:10000 /data/portal
COPY --from=build /out/portal /usr/local/bin/portal
COPY --from=web /out/web /web
COPY modules/portal/config/routes.json /config/routes.json
ENV NW_WEB_DIR=/web
USER 10000:10000
EXPOSE 8080
VOLUME ["/data/portal"]
WORKDIR /
ENTRYPOINT ["/usr/local/bin/portal"]
CMD ["--config", "/config/portal.json"]
