# memory 域独立镜像；构建上下文为仓库根：
# docker build -f modules/memory/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/memory/ modules/memory/
WORKDIR /src/modules/memory
RUN CGO_ENABLED=0 go build -trimpath -o /out/memory ./cmd/memory

FROM alpine:3.20
RUN adduser -D -u 10009 memory
COPY --from=build /out/memory /memory
COPY modules/memory/deploy/service.config.json /config/service.config.json
USER 10009:10009
EXPOSE 8109
VOLUME ["/data/memory"]
ENTRYPOINT ["/memory"]
CMD ["--config", "/config/service.config.json"]
