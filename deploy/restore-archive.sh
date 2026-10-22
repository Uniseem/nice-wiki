#!/bin/sh
# 从后台“备份与恢复”页下载的 tar.gz 备份恢复（归档里每个顶层目录对应一个服务的数据卷）。会覆盖当前数据卷，请先确认。
# 先核对归档可读且每个服务都有数据，停机后逐卷解到暂存目录，全部解好才替换；失败时原数据不动或被换回。
# 恢复后目录统一收紧为仅属主可访问（服务要求数据目录私有）。
# 密钥不在默认备份里：需要原来的 deploy/secrets（同一套签名密钥与主密钥）。
# 用法：deploy/restore-archive.sh <备份 tar.gz 或 tar.gz.enc 的绝对路径>
set -eu
cd "$(dirname "$0")"
archive="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
list="$(mktemp "$PWD/.restore-list.XXXXXX")"
plain=""
cleanup() { rm -f "$list"; [ -z "$plain" ] || rm -f "$plain"; }
trap cleanup EXIT
# 带 .enc 后缀的是设了下载口令的加密备份：先用 openssl 解密到临时文件（会提示输入口令）。
case "$archive" in
	*.enc)
		plain="$(mktemp "$PWD/.restore-plain.XXXXXX")"   # 放在 deploy 目录下，容器才能挂载到；脚本结束会删除
		# 口令默认交互输入；脚本化时可设环境变量 NW_BACKUP_PASSPHRASE。
		if [ -n "${NW_BACKUP_PASSPHRASE:-}" ]; then pass_arg="-pass env:NW_BACKUP_PASSPHRASE"; else pass_arg=""; fi
		openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 $pass_arg -in "$archive" -out "$plain" || { echo "解密失败（口令不对或文件损坏），未做任何改动"; exit 1; }
		archive="$plain"
		;;
esac
docker run --rm -v "$archive":/in/backup.tar.gz:ro alpine tar tzf /in/backup.tar.gz > "$list" || { echo "备份文件不可读，未做任何改动"; exit 1; }
volumes=""
for slug in portal public-site identity site-config content media model-gateway knowledge qa localization memory traffic-guard jobs analytics; do
	grep -q "^$slug/" "$list" || { echo "备份里缺少 $slug 的数据，未做任何改动"; exit 1; }
	volumes="$volumes nice-wiki_$slug-data"
done
printf '将用 %s 覆盖当前所有数据卷，输入 yes 继续：' "$archive"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
trap 'cleanup; docker compose start' EXIT
for volume in $volumes; do
	slug="${volume#nice-wiki_}"; slug="${slug%-data}"
	if ! docker run --rm -v "$volume":/data -v "$archive":/in/backup.tar.gz:ro alpine sh -c "
		set -e
		rm -rf /data/.restore-incoming /data/.restore-previous && mkdir /data/.restore-incoming
		tar tzf /in/backup.tar.gz | grep '^$slug/' > /tmp/list
		tar xzf /in/backup.tar.gz -C /data/.restore-incoming -T /tmp/list --strip-components=1
		chmod -R go-rwx /data/.restore-incoming && chown -R \$(stat -c %u:%g /data) /data/.restore-incoming"; then
		for v in $volumes; do docker run --rm -v "$v":/data alpine rm -rf /data/.restore-incoming; done
		echo "解包 $slug 失败（空间不足或归档损坏），已清掉暂存，原数据未改动"
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
		for v in $volumes; do docker run --rm -v "$v":/data alpine sh -c "$back"; done
		echo "替换 $volume 失败，所有数据卷已换回原数据"
		exit 1
	fi
done
for volume in $volumes; do docker run --rm -v "$volume":/data alpine rm -rf /data/.restore-previous; done
echo "恢复完成"
# 备份若包含密钥（超管勾选过），解到 ./secrets-restored，请自行核对后替换 deploy/secrets。
if grep -q '^secrets/' "$list"; then
	mkdir -p secrets-restored && tar xzf "$archive" -C secrets-restored --strip-components=1 secrets && echo "备份里的密钥已解到 deploy/secrets-restored，确认后用它替换 deploy/secrets"
fi
