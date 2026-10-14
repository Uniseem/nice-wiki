# content 域独立镜像；构建上下文为仓库根。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/content/ modules/content/
WORKDIR /src/modules/content
RUN CGO_ENABLED=0 go build -trimpath -o /out/content ./cmd/content

FROM alpine:3.20
RUN adduser -D -u 10003 content
COPY --from=build /out/content /content
COPY modules/content/config/service.defaults.json /config/service.defaults.json
USER 10003:10003
EXPOSE 8103
VOLUME ["/data/content"]
ENTRYPOINT ["/content"]
CMD ["--config", "/config/service.defaults.json"]
