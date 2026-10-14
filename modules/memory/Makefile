.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/memory

run:
	go run ./cmd/memory --config deploy/service.config.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
