.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/model-gateway

run:
	go run ./cmd/model-gateway --config deploy/service.config.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
