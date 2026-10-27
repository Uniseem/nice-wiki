# ops 进程镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/ modules/
COPY services/ services/
WORKDIR /src/services/ops
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/ops ./cmd/ops

FROM alpine:3.20
RUN adduser -D -u 10003 ops && mkdir -p /data && chown 10003:10003 /data
COPY --from=build /out/ops /usr/local/bin/ops
USER 10003:10003
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/ops"]
CMD ["--config-dir", "/config"]
