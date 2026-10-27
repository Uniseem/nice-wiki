#!/bin/sh
# 首次部署：生成各组件的签名密钥与公钥表、主密钥、数据库与检索口令，并写出容器内使用的引用文件。
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
	case "$slug" in budget|postgres|search) continue ;; esac
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
mkdir -p deploy/secrets/postgres deploy/secrets/search
rand() { head -c 24 /dev/urandom | base64 | tr -d '=+/\n'; }
[ -f deploy/secrets/master-key ] || head -c 32 /dev/urandom > deploy/secrets/master-key
[ -f deploy/secrets/postgres/superuser-password ] || rand > deploy/secrets/postgres/superuser-password
[ -f deploy/secrets/postgres/app-password ] || rand > deploy/secrets/postgres/app-password
[ -f deploy/secrets/search/secret ] || head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > deploy/secrets/search/secret
printf 'postgres://nw_app:%s@postgres:5432/nicewiki?sslmode=disable' "$(cat deploy/secrets/postgres/app-password)" > deploy/secrets/database-url
# 容器里的服务用户（uid 与宿主机不同）要能读到挂载进去的密钥文件：文件放开读，靠顶层目录 0700 挡住宿主机上的其他用户；主密钥与数据库口令只给 root。
find deploy/secrets -type d -exec chmod 755 {} +
find deploy/secrets -type f -exec chmod 644 {} +
chmod 600 deploy/secrets/master-key deploy/secrets/postgres/superuser-password
chmod 700 deploy/secrets
echo "密钥已生成（务必另行备份 deploy/secrets，丢了无法恢复数据）；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d（在源码仓库里加 --build 本机构建）"
