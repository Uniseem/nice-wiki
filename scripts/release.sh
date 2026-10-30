#!/bin/sh
# 发版：交叉编译全部服务、组装并推送 amd64/arm64 镜像、打“下载即部署”的发布包、发布到 GitHub Releases。
# GitHub Actions 在推送 v* 标签时按顺序调用（见 .github/workflows/release.yml）。
# 用法：scripts/release.sh build|images|package|publish <版本>
#   NW_IMAGES  镜像前缀，默认 ghcr.io/uniseem/nice-wiki（镜像为 <前缀>-<服务>:<版本>）
#   images     需要 crane 且已登录目标仓库；web-dist/ 为已构建的前端产物
#   publish    需要 GITHUB_TOKEN；说明取自附注标签正文
set -eu
cd "$(dirname "$0")/.."
step="$1"
tag="$2"
images="${NW_IMAGES:-ghcr.io/uniseem/nice-wiki}"
arches="amd64 arm64"
export COPYFILE_DISABLE=1 # macOS 上演练时不让 tar 带进 ._ 元数据文件

# 指定属主打 tar：GNU tar 与 macOS 的 bsdtar 参数不同。
mktar() {
	owner="$1"
	dir="$2"
	out="$3"
	shift 3
	if tar --version 2>/dev/null | grep -q GNU; then
		tar --numeric-owner --owner="$owner" --group="$owner" -C "$dir" -cf "$out" "$@"
	else
		tar --uid "$owner" --gid "$owner" -C "$dir" -cf "$out" "$@"
	fi
}

case "$step" in
build)
	# 各程序互不依赖，但共用大量依赖（platform、SQLite 等）：每种架构先编一个服务，把共用依赖放进构建缓存，
	# 再按 CPU 数并行编其余的。直接全部并行时每个进程都会各自重编一遍共用依赖，反而更慢。任一失败整步失败。
	export ROOT="$PWD"
	one='cd "$ROOT/${1%/cmd/*}" && CGO_ENABLED=0 GOOS=linux GOARCH="$0" go build -trimpath -ldflags "-s -w" -o "$ROOT/release/linux-$0/${1##*/}" "./cmd/${1##*/}"'
	first=services/content/cmd/content
	for arch in $arches; do
		mkdir -p "release/linux-$arch"
		echo "$arch $first"
	done | xargs -P 2 -n 2 sh -c "$one"
	for arch in $arches; do
		for cmd in services/*/cmd/* libs/platform/cmd/nw-keys; do
			[ "$cmd" = "$first" ] || echo "$arch $cmd"
		done
	done | xargs -P "$(getconf _NPROCESSORS_ONLN)" -n 2 sh -c "$one"
	;;
images)
	# 镜像 = alpine 基础层 + 程序文件层（root 所有）+ 数据目录层（服务 uid 所有）；规格见 deploy/release/images.tsv。
	# 直接在镜像仓库里组装，不需要 docker 守护进程，也不用模拟 arm64。各服务互不依赖、耗时在网络上，全部并行推送。
	work=$(mktemp -d)
	image() {
		slug="$1" uid="$2" port="$3" workdir="$4" binary="$5" cmd="$6"
		for arch in $arches; do
			files="$work/$slug-$arch/files"
			data="$work/$slug-$arch/data"
			# 可写目录归服务用户：edge 只写 /cache（媒体磁盘缓存），其余写 /data/<服务>。
			datadir="data/$slug"
			[ "$slug" = edge ] && datadir=cache
			mkdir -p "$files/usr/local/bin" "$data/$datadir"
			cp "release/linux-$arch/$binary" "$files/usr/local/bin/$binary"
			set -- usr
			env=""
			base=alpine:3.20
			if [ "$slug" = edge ]; then
				cp -R web-dist "$files/web"
				mkdir -p "$files/config"
				cp modules/portal/config/routes.json "$files/config/routes.json"
				set -- usr web config
			fi
			if [ "$slug" = ai ]; then
				mkdir -p "$files/opt/agent/licenses"
				cp -R agent-dist/. "$files/opt/agent/"
				cp -R docs/licenses/. "$files/opt/agent/licenses/"
				cp THIRD_PARTY_NOTICES.md "$files/opt/agent/"
				set -- usr opt
				env="--env=NW_ZCODE_WORKER=/opt/agent/worker.mjs"
				base=node:24-alpine
			fi
			mktar 0 "$files" "$work/$slug-$arch-files.tar" "$@"
			mktar "$uid" "$data" "$work/$slug-$arch-data.tar" "${datadir%%/*}"
			crane append --platform "linux/$arch" -b "$base" -f "$work/$slug-$arch-files.tar" -f "$work/$slug-$arch-data.tar" -t "$images-$slug:$tag-$arch"
			crane mutate "$images-$slug:$tag-$arch" --entrypoint="/usr/local/bin/$binary" --cmd="$cmd" --user="$uid:$uid" --workdir="$workdir" --exposed-ports="$port/tcp" $env \
				--label=org.opencontainers.image.source=https://github.com/Uniseem/nice-wiki --label="org.opencontainers.image.version=$tag" --label=org.opencontainers.image.licenses=AGPL-3.0-only \
				-t "$images-$slug:$tag-$arch"
		done
		crane index append -m "$images-$slug:$tag-amd64" -m "$images-$slug:$tag-arm64" -t "$images-$slug:$tag"
		crane tag "$images-$slug:$tag" latest
	}
	pids=""
	while IFS="$(printf '\t')" read -r slug uid port workdir binary cmd; do
		image "$slug" "$uid" "$port" "$workdir" "$binary" "$cmd" &
		pids="$pids $!"
	done <deploy/release/images.tsv
	failed=0
	for pid in $pids; do
		wait "$pid" || failed=1
	done
	rm -rf "$work"
	[ "$failed" = 0 ] || { echo "有镜像没推送成功" >&2; exit 1; }
	;;
package)
	# 发布包解压即可部署：deploy/ 里是拉取上面镜像的编排、配置、Caddy 与脚本，bin/ 自带 nw-keys，服务器上不需要 Go。
	rm -rf pkg dist
	mkdir -p dist
	for arch in $arches; do
		name="nice-wiki-$tag-linux-$arch"
		root="pkg/$name"
		mkdir -p "$root/config" "$root/deploy"
		cp -R "release/linux-$arch" "$root/bin"
		cp -R web-dist "$root/web"
		cp -R agent-dist "$root/agent"
		# nw-search、nw-edge 是 Rust 程序（工作流里按架构原生编译），随 bin 一起放进包里。
		cp modules/portal/config/routes.json "$root/config/routes.json"
		cp -R deploy/config deploy/caddy deploy/postgres "$root/deploy/"
		# 本机生成的公钥表和 Caddy 本地定制不属于发布包（CI 的干净检出里本来就没有）。
		rm -f "$root/deploy/config/trust-anchors.json"
		find "$root/deploy/caddy/conf.d" -type f ! -name README.md -exec rm -f {} +
		cp deploy/init-secrets.sh deploy/backup.sh deploy/restore.sh "$root/deploy/"
		cp deploy/release/docker-compose.yml "$root/deploy/docker-compose.yml"
		sed "s/^NW_VERSION=.*/NW_VERSION=$tag/" deploy/.env.example >"$root/deploy/.env.example"
		cp deploy/release/README.md "$root/README.md"
		cp LICENSE "$root/"
		cp THIRD_PARTY_NOTICES.md "$root/"
		tar -C pkg -czf "dist/$name.tar.gz" "$name"
	done
	if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
	(cd dist && $sum ./*.tar.gz | sed 's# \./# #' >SHA256SUMS)
	;;
publish)
	notes=$(mktemp)
	git for-each-ref "refs/tags/$tag" --format='%(contents)' >"$notes"
	python3 scripts/release_github.py "$tag" "$notes" dist/*
	rm -f "$notes"
	;;
*)
	echo "用法：scripts/release.sh build|images|package|publish <版本>" >&2
	exit 2
	;;
esac
