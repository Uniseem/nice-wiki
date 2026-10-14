.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/analytics

run:
	go run ./cmd/analytics --config config/defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
