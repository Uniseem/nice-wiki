#!/bin/sh
# 整站备份：数据库用 pg_dump 在线导出（一致快照），再打包文件卷和密钥。文件只增不改，数据库先导出，
# 之后打包的文件一定包含数据库里引用的全部文件。
# 用法：deploy/backup.sh [输出目录，默认 ./backups]；密钥含主密钥，压缩包请放在受控的存储里。
set -eu
cd "$(dirname "$0")"
out="${1:-./backups}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker compose exec -T postgres pg_dump -U postgres -Fc nicewiki > "$out/db-$stamp.dump"
for volume in account-data content-data ai-data ops-data knowledge-data portal-data search-data; do
	docker run --rm -v "nice-wiki_$volume":/data:ro -v "$out":/out alpine tar czf "/out/$volume-$stamp.tar.gz" -C /data .
done
tar czf "$out/secrets-$stamp.tar.gz" -C . secrets
echo "备份已写入 $out（时间戳 $stamp）。可以用 rclone 同步到异地存储：rclone copy $out remote:nicewiki"
