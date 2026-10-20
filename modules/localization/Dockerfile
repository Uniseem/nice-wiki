# localization 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/localization/ modules/localization/
WORKDIR /src/modules/localization
RUN CGO_ENABLED=0 go build -trimpath -o /out/localization ./cmd/localization

FROM alpine:3.20
RUN adduser -D -u 10009 localization && mkdir -p /data/localization && chown 10009:10009 /data/localization
COPY --from=build /out/localization /usr/local/bin/localization
USER 10009:10009
EXPOSE 8108
VOLUME ["/data/localization"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/localization"]
CMD ["--config", "/config/localization.json"]
