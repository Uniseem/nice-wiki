#!/bin/sh
# 过渡方案：从 backup.sh 生成的备份恢复。会覆盖当前数据卷，请先确认。
# 先核对全部归档都在且可读，停机后逐卷解到暂存目录，全部解好才替换；失败时原数据不动或被换回。
# 用法：deploy/restore.sh <备份目录> <时间戳>
set -eu
cd "$(dirname "$0")"
dir="$(cd "$1" && pwd)"; stamp="$2"
volumes="nice-wiki_portal-data nice-wiki_public-site-data nice-wiki_identity-data nice-wiki_site-config-data nice-wiki_content-data nice-wiki_media-data nice-wiki_model-gateway-data nice-wiki_knowledge-data nice-wiki_qa-data nice-wiki_localization-data nice-wiki_memory-data nice-wiki_traffic-guard-data nice-wiki_jobs-data nice-wiki_analytics-data nice-wiki_data-protection-data nice-wiki_agent-data"
for volume in $volumes; do
	[ -f "$dir/$volume-$stamp.tar.gz" ] || { echo "缺少 $volume-$stamp.tar.gz，未做任何改动"; exit 1; }
	docker run --rm -v "$dir":/in:ro alpine tar tzf "/in/$volume-$stamp.tar.gz" > /dev/null || { echo "$volume-$stamp.tar.gz 已损坏，未做任何改动"; exit 1; }
done
printf '将用备份 %s 覆盖当前所有数据卷，输入 yes 继续：' "$stamp"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
trap 'docker compose start' EXIT
for volume in $volumes; do
	if ! docker run --rm -v "$volume":/data -v "$dir":/in:ro alpine sh -c "set -e; rm -rf /data/.restore-incoming /data/.restore-previous; mkdir /data/.restore-incoming; tar xzf /in/$volume-$stamp.tar.gz -C /data/.restore-incoming"; then
		for v in $volumes; do docker run --rm -v "$v":/data alpine rm -rf /data/.restore-incoming || echo "$v 的暂存目录 .restore-incoming 没能清掉，请手动删除"; done
		echo "解包 $volume 失败（空间不足或归档损坏），原数据未改动"
		exit 1
	fi
done
# 换上：原数据挪进 .restore-previous（挪完留标记），再把暂存挪到卷根。
swap='set -e
mkdir /data/.restore-previous
find /data -mindepth 1 -maxdepth 1 ! -name .restore-incoming ! -name .restore-previous -exec sh -c "mv \"\$@\" /data/.restore-previous/" sh {} +
touch /data/.restore-previous/.restore-complete
find /data/.restore-incoming -mindepth 1 -maxdepth 1 -exec sh -c "mv \"\$@\" /data/" sh {} +
rmdir /data/.restore-incoming'
# 换回：有标记说明卷根已是新数据，先清掉；再把原数据挪回卷根。
back='set -e
if [ -d /data/.restore-previous ]; then
	if [ -f /data/.restore-previous/.restore-complete ]; then
		rm /data/.restore-previous/.restore-complete
		find /data -mindepth 1 -maxdepth 1 ! -name .restore-previous -exec rm -rf {} +
	fi
	find /data/.restore-previous -mindepth 1 -maxdepth 1 -exec sh -c "mv \"\$@\" /data/" sh {} +
	rmdir /data/.restore-previous
fi
rm -rf /data/.restore-incoming'
for volume in $volumes; do
	if ! docker run --rm -v "$volume":/data alpine sh -c "$swap"; then
		# 换回逐卷进行，某一卷出错也继续换回其余的卷，最后如实报告。
		failed=""
		for v in $volumes; do docker run --rm -v "$v":/data alpine sh -c "$back" || failed="$failed $v"; done
		if [ -n "$failed" ]; then echo "替换 $volume 失败；以下卷没能换回，原数据在卷内 .restore-previous，需手动挪回：$failed"; else echo "替换 $volume 失败，所有数据卷已换回原数据"; fi
		exit 1
	fi
done
for volume in $volumes; do docker run --rm -v "$volume":/data alpine rm -rf /data/.restore-previous || echo "$volume 的 .restore-previous 没能删除，可手动删除"; done
echo "恢复完成"
