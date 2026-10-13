FROM node:24-alpine AS web
WORKDIR /src
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN pnpm --dir web install --frozen-lockfile
COPY web ./web
COPY server/internal/site/templates ./server/internal/site/templates
RUN pnpm --dir web build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY server ./server
COPY api ./api
COPY --from=web /src/server/internal/webui/dist ./server/internal/webui/dist
RUN cd server && CGO_ENABLED=0 go build -trimpath -o /nice-wiki ./cmd/nice-wiki

RUN mkdir -p /empty-data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build --chown=65532:65532 /empty-data /data
COPY --from=build /nice-wiki /nice-wiki
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/nice-wiki","--data","/data","--addr",":8080"]
