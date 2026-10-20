# jobs 域独立镜像；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/jobs/ modules/jobs/
WORKDIR /src/modules/jobs
RUN CGO_ENABLED=0 go build -trimpath -o /out/jobs ./cmd/jobs

FROM alpine:3.20
RUN adduser -D -u 10012 jobs
COPY --from=build /out/jobs /jobs
USER 10012:10012
EXPOSE 8111
VOLUME ["/data/jobs"]
WORKDIR /config
ENTRYPOINT ["/jobs"]
CMD ["--config", "/config/jobs.json"]
