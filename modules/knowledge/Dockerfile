# knowledge 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/knowledge/ modules/knowledge/
WORKDIR /src/modules/knowledge
RUN CGO_ENABLED=0 go build -trimpath -o /out/knowledge ./cmd/knowledge

FROM alpine:3.20
RUN adduser -D -u 10007 knowledge && mkdir -p /data/knowledge && chown 10007:10007 /data/knowledge
COPY --from=build /out/knowledge /knowledge
USER 10007:10007
EXPOSE 8106
VOLUME ["/data/knowledge"]
WORKDIR /config
ENTRYPOINT ["/knowledge"]
CMD ["--config", "/config/knowledge.json"]
