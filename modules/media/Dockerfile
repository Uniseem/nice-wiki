# media 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/media/ modules/media/
WORKDIR /src/modules/media
RUN CGO_ENABLED=0 go build -trimpath -o /out/media ./cmd/media

FROM alpine:3.20
RUN adduser -D -u 10005 media
COPY --from=build /out/media /media
USER 10005:10005
EXPOSE 8104
VOLUME ["/data/media"]
WORKDIR /config
ENTRYPOINT ["/media"]
CMD ["--config", "/config/media.json"]
