#!/bin/sh
# 首次部署：生成各服务的签名密钥与公钥表，并写出容器内使用的引用文件。
# 需要 Go；私钥只写入 deploy/secrets（已在 .gitignore 里），不要提交。
set -eu
cd "$(dirname "$0")/.."
(cd libs/platform && go run ./cmd/nw-keys --services ../../deploy/config/services.json --out ../../deploy/config/trust-anchors.json)
for dir in deploy/secrets/*/; do
	slug=$(basename "$dir")
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
chmod -R go-rwx deploy/secrets
echo "密钥已生成；下一步：cp deploy/.env.example deploy/.env 并修改域名，然后 docker compose -f deploy/docker-compose.yml up -d --build"
