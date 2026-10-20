# data-protection 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/data-protection/ modules/data-protection/
WORKDIR /src/modules/data-protection
RUN CGO_ENABLED=0 go build -trimpath -o /out/data-protection ./cmd/data-protection

FROM alpine:3.20
RUN adduser -D -u 10014 data-protection && mkdir -p /data/data-protection && chown 10014:10014 /data/data-protection
COPY --from=build /out/data-protection /data-protection
USER 10014:10014
EXPOSE 8113
VOLUME ["/data/data-protection"]
WORKDIR /config
ENTRYPOINT ["/data-protection"]
CMD ["--config", "/config/data-protection.json"]
