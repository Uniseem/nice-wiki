.PHONY: generate build ui-build run verify

generate:
	cd ../.. && $(MAKE) gen

ui-build:
	cd ui && pnpm install --frozen-lockfile && pnpm run build

build: ui-build
	go build ./cmd/localization

run:
	go run ./cmd/localization --config deploy/service.config.json

verify: ui-build
	go build ./...
	go vet ./...
	@if (gofmt -l . | Select-String .) { exit 1 }

