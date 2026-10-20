# model-gateway 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/model-gateway/ modules/model-gateway/
WORKDIR /src/modules/model-gateway
RUN CGO_ENABLED=0 go build -trimpath -o /out/model-gateway ./cmd/model-gateway

FROM alpine:3.20
RUN adduser -D -u 10006 model-gateway
COPY --from=build /out/model-gateway /model-gateway
USER 10006:10006
EXPOSE 8105
VOLUME ["/data/model-gateway"]
WORKDIR /config
ENTRYPOINT ["/model-gateway"]
CMD ["--config", "/config/model-gateway.json"]
