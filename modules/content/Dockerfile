# content 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/content/ modules/content/
WORKDIR /src/modules/content
RUN CGO_ENABLED=0 go build -trimpath -o /out/content ./cmd/content

FROM alpine:3.20
RUN adduser -D -u 10004 content && mkdir -p /data/content && chown 10004:10004 /data/content
COPY --from=build /out/content /usr/local/bin/content
USER 10004:10004
EXPOSE 8103
VOLUME ["/data/content"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/content"]
CMD ["--config", "/config/content.json"]
