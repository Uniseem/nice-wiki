#!/bin/sh
# 过渡方案：整站停机，把各服务数据卷打成一个 tar.gz，再启动。data-protection 的在线一致备份完成后由它取代。
# 用法：deploy/backup.sh [输出目录，默认 ./backups]
set -eu
cd "$(dirname "$0")"
out="${1:-./backups}"
mkdir -p "$out"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker compose stop
trap 'docker compose start' EXIT
for volume in nice-wiki_portal-data nice-wiki_public-site-data nice-wiki_identity-data nice-wiki_site-config-data nice-wiki_content-data nice-wiki_media-data nice-wiki_model-gateway-data nice-wiki_knowledge-data nice-wiki_qa-data nice-wiki_localization-data nice-wiki_memory-data nice-wiki_traffic-guard-data nice-wiki_jobs-data nice-wiki_analytics-data nice-wiki_data-protection-data nice-wiki_agent-data; do
	docker run --rm -v "$volume":/data -v "$(cd "$out" && pwd)":/out alpine tar czf "/out/$volume-$stamp.tar.gz" -C /data .
done
echo "备份已写入 $out（时间戳 $stamp）；请同时妥善保管 deploy/secrets，没有密钥无法恢复。"
