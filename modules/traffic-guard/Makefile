.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/traffic-guard

run:
	go run ./cmd/traffic-guard --config config/defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
