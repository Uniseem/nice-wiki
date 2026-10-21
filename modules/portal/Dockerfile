# portal 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM node:24-slim AS web
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY libs/platform/web/ libs/platform/web/
COPY modules/portal/web/ modules/portal/web/
RUN cd modules/portal/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web --emptyOutDir
COPY modules/content/web/ modules/content/web/
RUN cd modules/content/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/content --emptyOutDir
COPY modules/site-config/web/ modules/site-config/web/
RUN cd modules/site-config/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/site-config --emptyOutDir
COPY modules/identity/web/ modules/identity/web/
RUN cd modules/identity/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/identity --emptyOutDir
COPY modules/media/web/ modules/media/web/
RUN cd modules/media/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/media --emptyOutDir
COPY modules/localization/web/ modules/localization/web/
RUN cd modules/localization/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/localization --emptyOutDir
COPY modules/model-gateway/web/ modules/model-gateway/web/
RUN cd modules/model-gateway/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/model-gateway --emptyOutDir
COPY modules/knowledge/web/ modules/knowledge/web/
RUN cd modules/knowledge/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/knowledge --emptyOutDir
COPY modules/memory/web/ modules/memory/web/
RUN cd modules/memory/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/memory --emptyOutDir
COPY modules/qa/web/ modules/qa/web/
RUN cd modules/qa/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/qa --emptyOutDir
COPY modules/traffic-guard/web/ modules/traffic-guard/web/
RUN cd modules/traffic-guard/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/traffic-guard --emptyOutDir
COPY modules/jobs/web/ modules/jobs/web/
RUN cd modules/jobs/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/jobs --emptyOutDir
COPY modules/analytics/web/ modules/analytics/web/
RUN cd modules/analytics/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/analytics --emptyOutDir
COPY modules/data-protection/web/ modules/data-protection/web/
RUN cd modules/data-protection/web && pnpm install --frozen-lockfile && pnpm exec vite build --outDir /out/web/modules/data-protection --emptyOutDir

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
