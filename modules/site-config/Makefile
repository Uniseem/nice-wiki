.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/site-config

run:
	go run ./cmd/site-config --config config/service.defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
