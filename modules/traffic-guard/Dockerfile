# traffic-guard 域独立镜像；构建上下文为仓库根：
# docker build -f modules/traffic-guard/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/traffic-guard/ modules/traffic-guard/
WORKDIR /src/modules/traffic-guard
RUN CGO_ENABLED=0 go build -trimpath -o /out/traffic-guard ./cmd/traffic-guard

FROM alpine:3.20
RUN adduser -D -u 10012 traffic-guard
COPY --from=build /out/traffic-guard /traffic-guard
COPY modules/traffic-guard/config/defaults.json /config/defaults.json
USER 10012:10012
EXPOSE 8110
VOLUME ["/data/traffic-guard"]
ENTRYPOINT ["/traffic-guard"]
CMD ["--config", "/config/defaults.json"]
