#!/bin/sh
# 从 backup.sh 的备份恢复：停应用，恢复数据库与文件卷，再启动。会覆盖当前数据，请先确认。
# 用法：deploy/restore.sh <备份目录> <时间戳>；密钥要用同一套（secrets-<时间戳>.tar.gz 里有，解到 deploy/secrets）。
set -eu
cd "$(dirname "$0")"
dir="$(cd "$1" && pwd)"; stamp="$2"
[ -f "$dir/db-$stamp.dump" ] || { echo "缺少 db-$stamp.dump，未做任何改动"; exit 1; }
for volume in account-data content-data ai-data ops-data knowledge-data portal-data search-data; do
	[ -f "$dir/$volume-$stamp.tar.gz" ] || { echo "缺少 $volume-$stamp.tar.gz，未做任何改动"; exit 1; }
done
printf '将用备份 %s 覆盖当前数据库和全部数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop caddy edge portal account content ai ops knowledge search
docker compose up -d postgres
docker compose exec -T postgres sh -c 'until pg_isready -U postgres -d nicewiki; do sleep 1; done'
docker compose exec -T postgres psql -U postgres -d nicewiki -c "DROP SCHEMA IF EXISTS public CASCADE; DO \$\$ DECLARE r record; BEGIN FOR r IN SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg\\_%' AND nspname <> 'information_schema' LOOP EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.nspname) || ' CASCADE'; END LOOP; END \$\$;"
docker compose exec -T postgres pg_restore -U postgres -d nicewiki --no-owner --role=nw_app < "$dir/db-$stamp.dump"
for volume in account-data content-data ai-data ops-data knowledge-data portal-data search-data; do
	docker run --rm -v "nice-wiki_$volume":/data -v "$dir":/in:ro alpine sh -c "find /data -mindepth 1 -delete && tar xzf /in/$volume-$stamp.tar.gz -C /data"
done
docker compose up -d
echo "恢复完成"
