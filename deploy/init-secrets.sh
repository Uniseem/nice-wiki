#!/bin/sh
# 首次部署：生成各服务的签名密钥与公钥表，并写出容器内使用的引用文件。
# 需要 Go；私钥只写入 deploy/secrets（已在 .gitignore 里），不要提交。
set -eu
cd "$(dirname "$0")/.."
# secretsDir 是相对仓库根的路径，所以要在仓库根运行工具（先编译出来）。
tool=$(mktemp)
(cd libs/platform && go build -o "$tool" ./cmd/nw-keys)
"$tool" --services deploy/config/services.json --out deploy/config/trust-anchors.json --budget-dir deploy/secrets/budget
rm -f "$tool"
budget=$(ls deploy/secrets/budget/budget-*.pem | head -n1)
cp "$budget" deploy/secrets/budget/budget.pem
printf '{"keyId":"%s","privateKeyRef":"/run/secrets/budget.pem"}\n' "$(basename "$budget" .pem | sed 's/^budget-//')" > deploy/secrets/budget/budget-key.json
for dir in deploy/secrets/*/; do
	slug=$(basename "$dir")
	[ "$slug" = budget ] && continue
	pem=$(ls "$dir"signing-*.pem | head -n1)
	keyid=$(basename "$pem" .pem | sed 's/^signing-//')
	cp "$pem" "$dir/signing.pem"
	printf '{"keyId":"%s","privateKeyRef":"/run/secrets/signing.pem"}\n' "$keyid" > "$dir/signing-key.json"
	if [ "$slug" = portal ]; then
		dpem=$(ls "$dir"delegation-*.pem | head -n1)
		did=$(basename "$dpem" .pem | sed 's/^delegation-//')
		cp "$dpem" "$dir/delegation.pem"
		printf '{"keyId":"%s","privateKeyRef":"/run/secrets/delegation.pem"}\n' "$did" > "$dir/delegation-key.json"
	fi
done
for s in model-gateway data-protection; do
	[ -f deploy/secrets/$s/master-key ] || head -c 32 /dev/urandom > deploy/secrets/$s/master-key
done
# 容器里的服务用户（uid 与宿主机不同）要能读到挂载进去的密钥文件：文件放开读，靠顶层目录 0700 挡住宿主机上的其他用户。
find deploy/secrets -type d -exec chmod 755 {} +
find deploy/secrets -type f -exec chmod 644 {} +
chmod 700 deploy/secrets
echo "密钥已生成；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d --build"
