#!/bin/sh
# 首次部署：生成各组件的签名密钥与公钥表、主密钥与数据库口令，并写出容器内使用的引用文件。
# 发布包自带 bin/nw-keys，不需要 Go；在源码仓库里运行时用 Go 现场编译。私钥只写入 deploy/secrets，不要提交。
set -eu
cd "$(dirname "$0")/.."
if [ -x bin/nw-keys ]; then
	tool=bin/nw-keys
else
	tool=$(mktemp)
	(cd libs/platform && go build -o "$tool" ./cmd/nw-keys)
fi
"$tool" --services deploy/config/services.json --out deploy/config/trust-anchors.json --budget-dir deploy/secrets/budget
[ "$tool" = bin/nw-keys ] || rm -f "$tool"
budget=$(ls deploy/secrets/budget/budget-*.pem | head -n1)
cp "$budget" deploy/secrets/budget/budget.pem
printf '{"keyId":"%s","privateKeyRef":"/run/secrets/budget/budget.pem"}\n' "$(basename "$budget" .pem | sed 's/^budget-//')" > deploy/secrets/budget/budget-key.json
for dir in deploy/secrets/*/; do
	slug=$(basename "$dir")
	case "$slug" in budget|postgres) continue ;; esac
	pem=$(ls "$dir"signing-*.pem | head -n1)
	keyid=$(basename "$pem" .pem | sed 's/^signing-//')
	cp "$pem" "$dir/signing.pem"
	printf '{"keyId":"%s","privateKeyRef":"/run/secrets/%s/signing.pem"}\n' "$keyid" "$slug" > "$dir/signing-key.json"
	if [ "$slug" = portal ]; then
		dpem=$(ls "$dir"delegation-*.pem | head -n1)
		did=$(basename "$dpem" .pem | sed 's/^delegation-//')
		cp "$dpem" "$dir/delegation.pem"
		printf '{"keyId":"%s","privateKeyRef":"/run/secrets/portal/delegation.pem"}\n' "$did" > "$dir/delegation-key.json"
	fi
done
mkdir -p deploy/secrets/postgres
rand() { head -c 24 /dev/urandom | base64 | tr -d '=+/\n'; }
[ -f deploy/secrets/master-key ] || head -c 32 /dev/urandom > deploy/secrets/master-key
[ -f deploy/secrets/postgres/superuser-password ] || rand > deploy/secrets/postgres/superuser-password
# 每个组件一个数据库角色：口令、连接串放在各自的密钥目录里，建角色的 SQL 给数据库首次启动用。
: > deploy/secrets/postgres/roles.sql
for slug in public-site identity site-config content media model-gateway knowledge qa localization memory traffic-guard jobs analytics agent; do
	dir=deploy/secrets/$slug
	role=nw_$(echo "$slug" | tr - _)
	[ -f "$dir/db-password" ] || rand > "$dir/db-password"
	printf 'postgres://%s:%s@postgres:5432/nicewiki?sslmode=disable' "$role" "$(cat "$dir/db-password")" > "$dir/database-url"
	printf "CREATE ROLE %s LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD '%s' IN ROLE nw_shared;\nGRANT CONNECT, CREATE ON DATABASE nicewiki TO %s;\n" "$role" "$(cat "$dir/db-password")" "$role" >> deploy/secrets/postgres/roles.sql
done
# 容器里的服务用户（uid 与宿主机不同）要能读到挂载进去的密钥文件：文件放开读，靠顶层目录 0700 挡住宿主机上的其他用户；主密钥与数据库口令只给 root。
find deploy/secrets -type d -exec chmod 755 {} +
find deploy/secrets -type f -exec chmod 644 {} +
chmod 600 deploy/secrets/master-key deploy/secrets/postgres/superuser-password
chmod 700 deploy/secrets
echo "密钥已生成（务必另行备份 deploy/secrets，丢了无法恢复数据）；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d（在源码仓库里加 --build 本机构建）"
