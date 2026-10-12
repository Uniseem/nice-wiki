SHELL := /bin/sh
GO := go
PNPM := pnpm
SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
OAPI := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1

.PHONY: dev dev-server dev-web gen sqlc gen-api build check format-check lint test privacy hooks gen-check e2e eval test-update catalog

dev:
	@$(MAKE) dev-server & server_pid=$$!; trap 'kill $$server_pid 2>/dev/null || true' EXIT INT TERM; $(MAKE) dev-web

dev-server:
	cd server && $(GO) run ./cmd/nice-wiki --data ../data --dev

dev-web:
	cd web && $(PNPM) dev

gen: gen-api sqlc

gen-api:
	cd server && $(OAPI) --config oapi-codegen.yaml ../api/openapi.yaml
	cd web && $(PNPM) gen:api

sqlc:
	@if [ -d server/internal/store/migrations ] && [ -d server/internal/store/queries ]; then cd server && $(SQLC) generate; else echo 'SQL 生成尚未启用：等待迁移与查询'; fi

build:
	cd web && $(PNPM) build
	mkdir -p bin
	cd server && CGO_ENABLED=0 $(GO) build -trimpath -o ../bin/nice-wiki ./cmd/nice-wiki

format-check:
	@files="$$(cd server && gofmt -l .)"; if [ -n "$$files" ]; then echo "$$files"; exit 1; fi
	git diff --check
	git diff --cached --check

lint:
	cd server && $(GO) vet ./...
	cd server && $(LINT) run
	cd web && $(PNPM) lint
	cd web && $(PNPM) typecheck

# 没有测试文件时 go test 仍编译所有包；前端尚无测试，不安装无用途工具。
test:
	cd server && $(GO) test -race ./...

privacy:
	cd server && $(GO) run ./tools/privacycheck -mode working

gen-check:
	@temporary=$$(mktemp -d); trap 'rm -rf "$$temporary"' EXIT INT TERM; \
	cp -R server/internal/api/gen "$$temporary/api"; \
	cp -R server/internal/store/sqlc "$$temporary/sqlc"; \
	cp web/src/shared/api/schema.gen.ts "$$temporary/schema.ts"; \
	$(MAKE) gen && diff -ru "$$temporary/api" server/internal/api/gen && \
	diff -ru "$$temporary/sqlc" server/internal/store/sqlc && \
	diff -u "$$temporary/schema.ts" web/src/shared/api/schema.gen.ts

check: format-check lint test gen-check privacy

hooks:
	git config core.hooksPath .githooks

# 这些操作需要用户确认，尚未建立对应的测试或评测入口。
e2e eval test-update:
	@echo '该操作需要用户确认且对应工具尚未建立'; exit 1

catalog:
	@echo '模型目录工具尚未实现'; exit 1

# 连接信息只从私有文件读取。服务器地址不得写进这个文件。
VPS_USER ?= root
VPS_PORT ?= 22
VPS_SSH := ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/tmp/nice-wiki-vps-known-hosts -p $(VPS_PORT)

.PHONY: vps-check vps-build vps-test
vps-check vps-build vps-test:
	@set -eu; address=$$(tr -d '\r\n' < vps.txt); \
	case "$$address" in ''|*[!0-9.]*) echo 'vps.txt 必须仅包含服务器 IPv4 地址'; exit 1;; esac; \
	case "$(VPS_USER)" in ''|*[!A-Za-z0-9_-]*) echo 'VPS_USER 无效'; exit 1;; esac; \
	destination="$(VPS_USER)@$$address"; \
	archive=$$(mktemp); trap 'rm -f "$$archive"' EXIT INT TERM; \
	COPYFILE_DISABLE=1 tar --no-xattrs --no-acls --exclude='._*' --exclude=.git --exclude=vps.txt --exclude=node_modules --exclude=web/node_modules \
	    --exclude=data --exclude=bin --exclude=server/internal/webui/dist --exclude=.DS_Store \
	    --exclude='*.tsbuildinfo' --exclude='.env*' -czf "$$archive" .; \
	remote=$$($(VPS_SSH) "$$destination" 'mkdir -p /root/.cache/nice-wiki-builds; mktemp -d /root/.cache/nice-wiki-builds/run.XXXXXX'); \
	case "$$remote" in /root/.cache/nice-wiki-builds/run.*) ;; *) echo '远程构建目录无效'; exit 1;; esac; \
	echo "VPS 构建目录: $$remote"; \
	$(VPS_SSH) "$$destination" "tar --no-same-owner -xzf - -C '$$remote'" < "$$archive"; \
	$(VPS_SSH) "$$destination" "cd '$$remote' && \
	 export PATH=/root/.cache/nice-wiki-tools/go/bin:/root/.cache/nice-wiki-tools/node-v24.13.0-linux-x64/bin:/root/.cache/nice-wiki-tools/pnpm/node_modules/.bin:\$$PATH && \
	 export GOMAXPROCS=8 GOFLAGS=-p=8 GOMEMLIMIT=1536MiB NODE_OPTIONS=--max-old-space-size=1024 && \
	 git init -q && mkdir -p server/internal/webui/dist && touch server/internal/webui/dist/.gitkeep && \
	 git add . && GIT_AUTHOR_NAME=Uniseem GIT_AUTHOR_EMAIL=325086315+Uniseem@users.noreply.github.com GIT_COMMITTER_NAME=Uniseem GIT_COMMITTER_EMAIL=325086315+Uniseem@users.noreply.github.com git commit -q -m 'chore: 临时构建快照' -m 'Co-Authored-By: GPT-6 <noreply@openai.com>' && pnpm --dir web install --frozen-lockfile && \
	 make gen && (cd server && go mod tidy) && pnpm --dir web lint --fix && \
	 pnpm --dir web build && make $(@:vps-%=%)"; \
	echo 'VPS 构建目录已保留供排查；不会同步私有连接文件或运行数据'
