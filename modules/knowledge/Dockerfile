# 知识域独立镜像，构建上下文为仓库根；生成与锁文件先由GitLab源码生成阶段回传。
FROM node:22-alpine AS ui-build
WORKDIR /src
COPY modules/knowledge/ui/package.json modules/knowledge/ui/pnpm-lock.yaml ./modules/knowledge/ui/
RUN corepack enable && pnpm --dir modules/knowledge/ui install --frozen-lockfile
COPY libs/platform/web/ ./libs/platform/web/
COPY modules/knowledge/ui/ ./modules/knowledge/ui/
RUN pnpm --dir modules/knowledge/ui run check && pnpm --dir modules/knowledge/ui run build

FROM golang:1.25-alpine AS server-build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/knowledge/ modules/knowledge/
WORKDIR /src/modules/knowledge
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -o /out/knowledge ./cmd/knowledge

FROM alpine:3.20
RUN adduser -D -u 10008 knowledge
COPY --from=server-build /out/knowledge /knowledge
COPY modules/knowledge/deploy/service.config.json /config/service.config.json
COPY --from=ui-build /src/modules/knowledge/ui/dist /ui
USER 10008:10008
EXPOSE 8106
VOLUME ["/data/knowledge"]
ENTRYPOINT ["/knowledge"]
CMD ["--config", "/config/service.config.json"]
