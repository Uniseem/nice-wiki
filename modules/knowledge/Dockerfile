# knowledge 进程镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/ modules/
COPY services/ services/
WORKDIR /src/modules/knowledge
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/knowledge ./cmd/knowledge

FROM alpine:3.20
RUN adduser -D -u 10005 knowledge && mkdir -p /data && chown 10005:10005 /data
COPY --from=build /out/knowledge /usr/local/bin/knowledge
USER 10005:10005
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/knowledge"]
CMD ["--config", "/config/knowledge.json"]
