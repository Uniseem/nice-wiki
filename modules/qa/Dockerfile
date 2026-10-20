# qa 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/qa/ modules/qa/
WORKDIR /src/modules/qa
RUN CGO_ENABLED=0 go build -trimpath -o /out/qa ./cmd/qa

FROM alpine:3.20
RUN adduser -D -u 10008 qa && mkdir -p /data/qa && chown 10008:10008 /data/qa
COPY --from=build /out/qa /qa
USER 10008:10008
EXPOSE 8107
VOLUME ["/data/qa"]
WORKDIR /config
ENTRYPOINT ["/qa"]
CMD ["--config", "/config/qa.json"]
