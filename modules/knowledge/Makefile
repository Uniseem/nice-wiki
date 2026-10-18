.PHONY: generate build run verify

# 仅作为GitLab任务入口；本机不执行编译、生成或前端安装。
generate:
	cd ../.. && $(MAKE) gen

build:
	go build -mod=readonly ./cmd/knowledge
	pnpm --dir ui install --frozen-lockfile
	pnpm --dir ui run check
	pnpm --dir ui run build

run:
	go run -mod=readonly ./cmd/knowledge --config deploy/service.config.json

verify:
	go build -mod=readonly ./...
	go vet -mod=readonly ./...
	test -z "$$(gofmt -l .)"
	pnpm --dir ui run check
