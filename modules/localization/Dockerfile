# localization 域独立镜像；构建上下文为仓库根：
# docker build -f modules/localization/Dockerfile .
FROM node:22-alpine AS ui-build
WORKDIR /src
COPY modules/localization/ui/package.json modules/localization/ui/pnpm-lock.yaml ./modules/localization/ui/
RUN corepack enable && pnpm --dir modules/localization/ui install --frozen-lockfile
COPY modules/localization/ui/ ./modules/localization/ui/
RUN pnpm --dir modules/localization/ui run build

FROM golang:1.25-alpine AS server-build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/localization/ modules/localization/
WORKDIR /src/modules/localization
RUN CGO_ENABLED=0 go build -trimpath -o /out/localization ./cmd/localization

FROM alpine:3.20
RUN adduser -D -u 10010 localization
COPY --from=server-build /out/localization /localization
COPY modules/localization/deploy/service.config.json /config/service.config.json
COPY --from=ui-build /src/modules/localization/ui/dist /ui
USER 10010:10010
EXPOSE 8108
VOLUME ["/data/localization"]
ENTRYPOINT ["/localization"]
CMD ["--config", "/config/service.config.json"]
