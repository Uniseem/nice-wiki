# memory 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/memory/ modules/memory/
WORKDIR /src/modules/memory
RUN CGO_ENABLED=0 go build -trimpath -o /out/memory ./cmd/memory

FROM alpine:3.20
RUN adduser -D -u 10010 memory && mkdir -p /data/memory && chown 10010:10010 /data/memory
COPY --from=build /out/memory /usr/local/bin/memory
USER 10010:10010
EXPOSE 8109
VOLUME ["/data/memory"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/memory"]
CMD ["--config", "/config/memory.json"]
