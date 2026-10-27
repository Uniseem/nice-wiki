# Rust 进程镜像（检索 search、公开入口 edge）；构建上下文为仓库根。由 scripts/gen_deploy.py 生成。
FROM rust:1.95-bookworm AS build
WORKDIR /src
COPY rust/ rust/
WORKDIR /src/rust
RUN cargo build --release -p nw-search -p nw-edge

FROM debian:bookworm-slim AS search
RUN useradd -u 10004 -m search && mkdir -p /data && chown 10004:10004 /data
COPY --from=build /src/rust/target/release/nw-search /usr/local/bin/nw-search
USER 10004:10004
ENTRYPOINT ["/usr/local/bin/nw-search"]
CMD ["/config/search.json"]

FROM debian:bookworm-slim AS edge
RUN useradd -u 10005 -m edge && mkdir -p /cache && chown 10005:10005 /cache
COPY --from=build /src/rust/target/release/nw-edge /usr/local/bin/nw-edge
USER 10005:10005
ENTRYPOINT ["/usr/local/bin/nw-edge"]
CMD ["/config/edge.json"]
