# identity 域独立镜像；构建上下文为仓库根：
# docker build -f modules/identity/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/identity/ modules/identity/
WORKDIR /src/modules/identity
RUN CGO_ENABLED=0 go build -trimpath -o /out/identity ./cmd/identity

FROM alpine:3.20
RUN adduser -D -u 10001 identity
COPY --from=build /out/identity /identity
COPY modules/identity/config/service.defaults.json /config/service.defaults.json
USER 10001:10001
EXPOSE 8101
VOLUME ["/data/identity"]
ENTRYPOINT ["/identity"]
CMD ["--config", "/config/service.defaults.json"]
