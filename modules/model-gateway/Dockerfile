# model-gateway 域独立镜像；构建上下文为仓库根。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/model-gateway/ modules/model-gateway/
WORKDIR /src/modules/model-gateway
RUN CGO_ENABLED=0 go build -trimpath -o /out/model-gateway ./cmd/model-gateway

FROM alpine:3.20
RUN adduser -D -u 10007 model-gateway
COPY --from=build /out/model-gateway /model-gateway
COPY modules/model-gateway/deploy/service.config.json /config/service.config.json
USER 10007:10007
EXPOSE 8105
VOLUME ["/data/model-gateway"]
ENTRYPOINT ["/model-gateway"]
CMD ["--config", "/config/service.config.json"]
