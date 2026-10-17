# data-protection 域独立镜像；构建上下文为仓库根：
# docker build -f modules/data-protection/Dockerfile .
# 本阶段没有界面构建产物，镜像只带本域服务与默认配置。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/data-protection/ modules/data-protection/
WORKDIR /src/modules/data-protection
RUN CGO_ENABLED=0 go build -trimpath -o /out/data-protection ./cmd/data-protection

FROM alpine:3.20
RUN adduser -D -u 10014 data-protection
COPY --from=build /out/data-protection /data-protection
COPY modules/data-protection/config/defaults.json /config/defaults.json
USER 10014:10014
EXPOSE 8113
VOLUME ["/data/data-protection"]
ENTRYPOINT ["/data-protection"]
CMD ["--config", "/config/defaults.json", "--listen", "0.0.0.0:8113"]
