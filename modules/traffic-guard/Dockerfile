# traffic-guard 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/traffic-guard/ modules/traffic-guard/
WORKDIR /src/modules/traffic-guard
RUN CGO_ENABLED=0 go build -trimpath -o /out/traffic-guard ./cmd/traffic-guard

FROM alpine:3.20
RUN adduser -D -u 10011 traffic-guard && mkdir -p /data/traffic-guard && chown 10011:10011 /data/traffic-guard
COPY --from=build /out/traffic-guard /usr/local/bin/traffic-guard
USER 10011:10011
EXPOSE 8110
VOLUME ["/data/traffic-guard"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/traffic-guard"]
CMD ["--config", "/config/traffic-guard.json"]
