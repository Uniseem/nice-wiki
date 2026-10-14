.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/content

run:
	go run ./cmd/content --config config/service.defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
