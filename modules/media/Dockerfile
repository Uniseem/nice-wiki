# media 域独立镜像；构建上下文为仓库根：
# docker build -f modules/media/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/media/ modules/media/
WORKDIR /src/modules/media
RUN CGO_ENABLED=0 go build -trimpath -o /out/media ./cmd/media

FROM alpine:3.20
RUN adduser -D -u 10001 media
COPY --from=build /out/media /media
COPY modules/media/config/service.defaults.json /config/service.defaults.json
USER 10001:10001
EXPOSE 8104
VOLUME ["/data/media"]
ENTRYPOINT ["/media"]
CMD ["--config", "/config/service.defaults.json"]
