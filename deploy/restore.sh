#!/bin/sh
# 过渡方案：从 backup.sh 生成的备份恢复。会覆盖当前数据卷，请先确认。
# 用法：deploy/restore.sh <备份目录> <时间戳>
set -eu
cd "$(dirname "$0")"
dir="$1"; stamp="$2"
printf '将用备份 %s 覆盖当前所有数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
for volume in nice-wiki_portal-data nice-wiki_public-site-data nice-wiki_identity-data nice-wiki_site-config-data nice-wiki_content-data nice-wiki_media-data nice-wiki_model-gateway-data nice-wiki_knowledge-data nice-wiki_qa-data nice-wiki_localization-data nice-wiki_memory-data nice-wiki_traffic-guard-data nice-wiki_jobs-data nice-wiki_analytics-data nice-wiki_data-protection-data; do
	docker run --rm -v "$volume":/data -v "$(cd "$dir" && pwd)":/in alpine sh -c "rm -rf /data/* /data/.[!.]* 2>/dev/null; tar xzf /in/$volume-$stamp.tar.gz -C /data"
done
docker compose start
