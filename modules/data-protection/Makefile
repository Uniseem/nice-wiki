.PHONY: generate build run verify ui

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/data-protection

run:
	go run ./cmd/data-protection --config config/defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"

ui:
	@echo '界面构建等待 web/package.json 与 pnpm-lock.yaml' >&2
	@exit 1
