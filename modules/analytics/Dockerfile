# analytics 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/analytics/ modules/analytics/
WORKDIR /src/modules/analytics
RUN CGO_ENABLED=0 go build -trimpath -o /out/analytics ./cmd/analytics

FROM alpine:3.20
RUN adduser -D -u 10013 analytics && mkdir -p /data/analytics && chown 10013:10013 /data/analytics
COPY --from=build /out/analytics /usr/local/bin/analytics
USER 10013:10013
EXPOSE 8112
VOLUME ["/data/analytics"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/analytics"]
CMD ["--config", "/config/analytics.json"]
