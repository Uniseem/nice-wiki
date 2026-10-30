# account 进程镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/ modules/
COPY services/ services/
WORKDIR /src/services/account
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/account ./cmd/account

FROM alpine:3.20
RUN adduser -D -u 10000 account && mkdir -p /data && chown 10000:10000 /data
COPY --from=build /out/account /usr/local/bin/account
USER 10000:10000
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/account"]
CMD ["--config-dir", "/config"]
