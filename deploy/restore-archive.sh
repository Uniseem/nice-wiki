#!/bin/sh
# 从后台“备份与恢复”页下载的 tar.gz 备份恢复（归档里每个顶层目录对应一个服务的数据卷）。会覆盖当前数据卷，请先确认。
# 恢复后目录统一收紧为仅属主可访问（服务要求数据目录私有）。
# 密钥不在默认备份里：需要原来的 deploy/secrets（同一套签名密钥与主密钥）。
# 用法：deploy/restore-archive.sh <备份 tar.gz 或 tar.gz.enc 的绝对路径>
set -eu
cd "$(dirname "$0")"
archive="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
# 带 .enc 后缀的是设了下载口令的加密备份：先用 openssl 解密到临时文件（会提示输入口令）。
case "$archive" in
	*.enc)
		plain="$(mktemp "$PWD/.restore-plain.XXXXXX")"   # 放在 deploy 目录下，容器才能挂载到；脚本结束会删除
		trap 'rm -f "$plain"' EXIT
		# 口令默认交互输入；脚本化时可设环境变量 NW_BACKUP_PASSPHRASE。
		if [ -n "${NW_BACKUP_PASSPHRASE:-}" ]; then pass_arg="-pass env:NW_BACKUP_PASSPHRASE"; else pass_arg=""; fi
		openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 $pass_arg -in "$archive" -out "$plain" || { echo "解密失败（口令不对或文件损坏），未做任何改动"; exit 1; }
		archive="$plain"
		;;
esac
docker run --rm -v "$archive":/in/backup.tar.gz:ro alpine tar tzf /in/backup.tar.gz > /dev/null || { echo "备份文件不可读，未做任何改动"; exit 1; }
printf '将用 %s 覆盖当前所有数据卷，输入 yes 继续：' "$archive"
read answer
[ "$answer" = yes ] || exit 1
docker compose stop
for slug in portal public-site identity site-config content media model-gateway knowledge qa localization memory traffic-guard jobs analytics; do
	docker run --rm -v "nice-wiki_$slug-data":/data -v "$archive":/in/backup.tar.gz:ro alpine sh -c "
		set -e
		tar tzf /in/backup.tar.gz | grep '^$slug/' > /tmp/list || exit 0
		owner=\$(stat -c %u:%g /data)
		rm -rf /data/incoming-restore && mkdir /data/incoming-restore
		tar xzf /in/backup.tar.gz -C /data/incoming-restore -T /tmp/list --strip-components=1
		find /data -mindepth 1 -maxdepth 1 ! -name incoming-restore -exec rm -rf {} +
		find /data/incoming-restore -mindepth 1 -maxdepth 1 -exec mv {} /data/ \\;
		rmdir /data/incoming-restore
		chmod -R go-rwx /data && chown -R \$owner /data"
done
# 备份若包含密钥（超管勾选过），解到 ./secrets-restored，请自行核对后替换 deploy/secrets。
if docker run --rm -v "$archive":/in/backup.tar.gz:ro alpine tar tzf /in/backup.tar.gz | grep -q '^secrets/'; then
	mkdir -p secrets-restored && tar xzf "$archive" -C secrets-restored --strip-components=1 secrets && echo "备份里的密钥已解到 deploy/secrets-restored，确认后用它替换 deploy/secrets"
fi
docker compose start
