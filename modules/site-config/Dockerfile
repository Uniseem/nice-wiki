# site-config 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/site-config/ modules/site-config/
WORKDIR /src/modules/site-config
RUN CGO_ENABLED=0 go build -trimpath -o /out/site-config ./cmd/site-config

FROM alpine:3.20
RUN adduser -D -u 10003 site-config && mkdir -p /data/site-config && chown 10003:10003 /data/site-config
COPY --from=build /out/site-config /usr/local/bin/site-config
USER 10003:10003
EXPOSE 8102
VOLUME ["/data/site-config"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/site-config"]
CMD ["--config", "/config/site-config.json"]
