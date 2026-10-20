# identity 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/identity/ modules/identity/
WORKDIR /src/modules/identity
RUN CGO_ENABLED=0 go build -trimpath -o /out/identity ./cmd/identity

FROM alpine:3.20
RUN adduser -D -u 10002 identity && mkdir -p /data/identity && chown 10002:10002 /data/identity
COPY --from=build /out/identity /identity
USER 10002:10002
EXPOSE 8101
VOLUME ["/data/identity"]
WORKDIR /config
ENTRYPOINT ["/identity"]
CMD ["--config", "/config/identity.json"]
