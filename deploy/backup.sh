#!/bin/sh
# 整站备份：数据库用 pg_dump 在线导出（一致快照）；正文修订与媒体文件只增不改，增量复制到输出目录里的
# content-data/（第二次起只复制新增的）；其余小卷整卷打包；再打包密钥。数据库先导出、文件后复制，
# 所以文件一定包含数据库里引用的全部文件。
# 用法：deploy/backup.sh [输出目录，默认 ./backups]
# 可选环境变量：
#   BACKUP_PASSPHRASE_FILE  口令文件。设置后数据库导出和密钥包用它加密（openssl aes-256-cbc + pbkdf2，文件名多 .enc）；
#                           恢复时要给同一个口令文件。口令丢了备份就解不开，请和密钥分开保管。
#   BACKUP_REMOTE           rclone 目标（如 s3:桶/nicewiki、dav:nicewiki、gdrive:nicewiki，需先 rclone config）。
#                           设置后备份完成会把输出目录同步过去（rclone copy，已有的文件不重传）；要加密正文和媒体，
#                           把它配成 rclone 的 crypt 远端。
# 密钥含主密钥：没有加密时，备份目录请放在受控的存储里。
set -eu
cd "$(dirname "$0")"
project="${COMPOSE_PROJECT_NAME:-nice-wiki}"
out="${1:-./backups}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
seal() {
	[ -n "${BACKUP_PASSPHRASE_FILE:-}" ] || return 0
	openssl enc -aes-256-cbc -pbkdf2 -salt -pass "file:$BACKUP_PASSPHRASE_FILE" -in "$1" -out "$1.enc"
	rm "$1"
}
docker compose exec -T postgres pg_dump -U postgres -Fc nicewiki > "$out/db-$stamp.dump"
seal "$out/db-$stamp.dump"
mkdir -p "$out/content-data"
docker run --rm -v "${project}_content-data":/data:ro -v "$out/content-data":/out alpine sh -c "cp -au /data/. /out/ && chown -R $(id -u):$(id -g) /out"
for volume in account-data ai-data ops-data search-data; do
	docker run --rm -v "${project}_$volume":/data:ro -v "$out":/out alpine tar czf "/out/$volume-$stamp.tar.gz" -C /data .
done
tar czf "$out/secrets-$stamp.tar.gz" -C . secrets
seal "$out/secrets-$stamp.tar.gz"
if [ -n "${BACKUP_REMOTE:-}" ]; then
	rclone copy "$out" "$BACKUP_REMOTE"
fi
echo "备份已写入 $out（时间戳 $stamp）"
