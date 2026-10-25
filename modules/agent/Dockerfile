# agent 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM node:24-slim AS native
RUN npm install -g pnpm@9.15.9
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY libs/platform/zcode/ libs/platform/zcode/
RUN pnpm install --frozen-lockfile
COPY scripts/build_zcode.mjs scripts/build_zcode.mjs
COPY THIRD_PARTY_NOTICES.md ./
COPY docs/licenses/ docs/licenses/
RUN pnpm build:agent

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/agent/ modules/agent/
WORKDIR /src/modules/agent
RUN CGO_ENABLED=0 go build -trimpath -o /out/agent ./cmd/agent

FROM node:24-alpine
RUN adduser -D -u 10015 agent && mkdir -p /data/agent && chown 10015:10015 /data/agent
COPY --from=build /out/agent /usr/local/bin/agent
COPY --from=native /src/agent-dist/ /opt/agent/
COPY --from=native /src/docs/licenses/ /opt/agent/licenses/
COPY THIRD_PARTY_NOTICES.md /opt/agent/THIRD_PARTY_NOTICES.md
ENV NW_ZCODE_WORKER=/opt/agent/worker.mjs
USER 10015:10015
EXPOSE 8114
VOLUME ["/data/agent"]
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/agent"]
CMD ["--config", "/config/agent.json"]
