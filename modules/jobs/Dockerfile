# jobs 域独立镜像；构建上下文为仓库根：
# docker build -f modules/jobs/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY libs/platform/ libs/platform/
COPY modules/jobs/ modules/jobs/
WORKDIR /src/modules/jobs
RUN CGO_ENABLED=0 go build -trimpath -o /out/jobs ./cmd/jobs

FROM alpine:3.20
RUN adduser -D -u 10013 jobs
COPY --from=build /out/jobs /jobs
COPY modules/jobs/config/ /config/
USER 10013:10013
EXPOSE 8111
VOLUME ["/data/jobs"]
ENTRYPOINT ["/jobs"]
CMD ["--config", "/config/defaults.json", "--listen", "0.0.0.0:8111"]
