# public-site 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/public-site/ modules/public-site/
WORKDIR /src/modules/public-site
RUN CGO_ENABLED=0 go build -trimpath -o /out/public-site ./cmd/public-site

FROM alpine:3.20
RUN adduser -D -u 10001 public-site
COPY --from=build /out/public-site /public-site
USER 10001:10001
EXPOSE 8081
VOLUME ["/data/public-site"]
WORKDIR /config
ENTRYPOINT ["/public-site"]
CMD ["--config", "/config/public-site.json"]
