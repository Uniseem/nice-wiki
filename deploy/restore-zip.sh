#!/bin/sh
# 从后台“备份与恢复”页下载的 zip 备份恢复（zip 里每个顶层目录对应一个服务的数据卷）。会覆盖当前数据卷，请先确认。
# 密钥不在备份里：需要原来的 deploy/secrets（同一套签名密钥与主密钥）。
# 用法：deploy/restore-zip.sh <备份zip路径>
set -eu
cd "$(dirname "$0")"
zip="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
printf '将用 %s 覆盖当前所有数据卷，输入 yes 继续：' "$zip"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
for slug in portal public-site identity site-config content media model-gateway knowledge qa localization memory traffic-guard jobs analytics; do
	docker run --rm -v "nice-wiki_$slug-data":/data -v "$zip":/in/backup.zip:ro alpine sh -c "
		mkdir -p /tmp/x && unzip -q /in/backup.zip '$slug/*' -d /tmp/x 2>/dev/null || exit 0
		owner=\$(stat -c %u:%g /data)
		rm -rf /data/* /data/.[!.]* 2>/dev/null
		cp -a /tmp/x/$slug/. /data/ && chown -R \$owner /data"
done
docker compose start
