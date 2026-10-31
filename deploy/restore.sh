#!/bin/sh
# 从 backup.sh 的备份恢复：停应用，恢复数据库与文件卷，再启动。会覆盖当前数据，请先确认。
# 用法：deploy/restore.sh <备份目录> <时间戳>
# 备份加密过时，用环境变量 BACKUP_PASSPHRASE_FILE 给出同一个口令文件。
# 密钥要用同一套：secrets-<时间戳>.tar.gz（加密的是 .enc）在 deploy 目录里解开（得到 secrets/ 和 config/trust-anchors.json），本脚本不动密钥。
set -eu
cd "$(dirname "$0")"
project="${COMPOSE_PROJECT_NAME:-nice-wiki}"
dir="$(cd "$1" && pwd)"; stamp="$2"
# 输出备份文件的内容：有加密版就解密，没有就原样输出。
unseal() {
	if [ -f "$1.enc" ]; then
		[ -n "${BACKUP_PASSPHRASE_FILE:-}" ] || { echo "备份是加密的，请设置 BACKUP_PASSPHRASE_FILE" >&2; exit 1; }
		openssl enc -d -aes-256-cbc -pbkdf2 -pass "file:$BACKUP_PASSPHRASE_FILE" -in "$1.enc"
	else
		cat "$1"
	fi
}
[ -f "$dir/db-$stamp.dump" ] || [ -f "$dir/db-$stamp.dump.enc" ] || { echo "缺少 db-$stamp.dump，未做任何改动"; exit 1; }
[ -d "$dir/content-data" ] || { echo "缺少 content-data 目录，未做任何改动"; exit 1; }
unseal "$dir/db-$stamp.dump" | head -c 5 | grep -q PGDMP || { echo "数据库导出读不出来（口令不对或文件损坏），未做任何改动"; exit 1; }
for volume in account-data ai-data ops-data search-data; do
	[ -f "$dir/$volume-$stamp.tar.gz" ] || { echo "缺少 $volume-$stamp.tar.gz，未做任何改动"; exit 1; }
done
printf '将用备份 %s 覆盖当前数据库和全部数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop caddy edge account content ai ops search
docker compose up -d postgres
docker compose exec -T postgres sh -c 'until pg_isready -U postgres -d nicewiki; do sleep 1; done'
# pg_dump 不写 CREATE SCHEMA public（默认就有），删光后要先建回来，扩展和表才建得进去。
docker compose exec -T postgres psql -U postgres -d nicewiki -c "DROP SCHEMA IF EXISTS public CASCADE; DO \$\$ DECLARE r record; BEGIN FOR r IN SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg\\_%' AND nspname <> 'information_schema' LOOP EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.nspname) || ' CASCADE'; END LOOP; END \$\$; CREATE SCHEMA public;"
unseal "$dir/db-$stamp.dump" | docker compose exec -T postgres pg_restore -U postgres --single-transaction -d nicewiki
docker run --rm -v "${project}_content-data":/data -v "$dir/content-data":/in:ro alpine sh -c "find /data -mindepth 1 -delete && cp -a /in/. /data/"
for volume in account-data ai-data ops-data search-data; do
	docker run --rm -v "${project}_$volume":/data -v "$dir":/in:ro alpine sh -c "find /data -mindepth 1 -delete && tar xzf /in/$volume-$stamp.tar.gz -C /data"
done
docker compose up -d
echo "恢复完成"
