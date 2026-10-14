# site-config 域独立镜像；构建上下文为仓库根：
# docker build -f modules/site-config/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/site-config/ modules/site-config/
WORKDIR /src/modules/site-config
RUN CGO_ENABLED=0 go build -trimpath -o /out/site-config ./cmd/site-config

FROM alpine:3.20
RUN adduser -D -u 10002 site-config
COPY --from=build /out/site-config /site-config
COPY modules/site-config/config/service.defaults.json /config/service.defaults.json
USER 10002:10002
EXPOSE 8102
VOLUME ["/data/site-config"]
ENTRYPOINT ["/site-config"]
CMD ["--config", "/config/service.defaults.json"]
