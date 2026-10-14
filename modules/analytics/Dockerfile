# analytics 域独立镜像；构建上下文为仓库根：
# docker build -f modules/analytics/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/analytics/ modules/analytics/
WORKDIR /src/modules/analytics
RUN CGO_ENABLED=0 go build -trimpath -o /out/analytics ./cmd/analytics

FROM alpine:3.20
RUN adduser -D -u 10001 analytics
COPY --from=build /out/analytics /analytics
COPY modules/analytics/config/defaults.json /config/defaults.json
USER 10001:10001
EXPOSE 8112
VOLUME ["/data/analytics"]
ENTRYPOINT ["/analytics"]
CMD ["--config", "/config/defaults.json"]
