.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/identity

run:
	go run ./cmd/identity --config config/service.defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
