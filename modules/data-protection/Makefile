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
	cd web && pnpm install --frozen-lockfile && pnpm typecheck && pnpm build
