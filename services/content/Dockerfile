# content 进程镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/ modules/
COPY services/ services/
WORKDIR /src/services/content
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/content ./cmd/content

FROM alpine:3.20
RUN adduser -D -u 10001 content && mkdir -p /data && chown 10001:10001 /data
COPY --from=build /out/content /usr/local/bin/content
USER 10001:10001
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/content"]
CMD ["--config-dir", "/config"]
